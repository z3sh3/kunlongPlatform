"""Durable metadata store for datasets and jobs.

The service keeps its own truth in plain JSON documents under
`<DATA_ROOT>/state/` so that a restart can resume polling long-running
training subprocesses instead of losing track of them. Writes are atomic
(temp file + `os.replace`) and serialised by a single process lock: the
service is designed to run as one process per GPU host.
"""

from __future__ import annotations

import contextlib
import json
import os
import re
import threading
import time
import uuid
from pathlib import Path
from typing import Any

from app.settings import settings

_LOCK = threading.RLock()

# Record ids are generated locally and echoed back by the platform, so they are
# validated before being used in a filename: a client must never be able to
# address a path outside `<DATA_ROOT>/state/`.
_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]{1,64}$")

DATASET_KIND = "dataset"
JOB_KIND = "job"

TERMINAL_JOB_STATUSES = frozenset({"succeeded", "failed", "canceled"})


class InvalidRecordId(ValueError):
    """Raised when a caller-supplied id is not a safe filename component."""


def require_valid_id(record_id: str) -> str:
    if not _ID_PATTERN.match(record_id or ""):
        raise InvalidRecordId(f"invalid record id: {record_id!r}")
    return record_id


def new_id() -> str:
    """Return a short, collision-resistant identifier for a record."""
    return uuid.uuid4().hex[:16]


def now_ms() -> int:
    return int(time.time() * 1000)


def _state_path(kind: str, record_id: str) -> Path:
    return settings.state_dir / f"{kind}-{require_valid_id(record_id)}.json"


def _read_json(path: Path) -> dict[str, Any] | None:
    """Read a JSON object, returning None when it is absent or unreadable."""
    try:
        with open(path, encoding="utf-8") as handle:
            parsed = json.load(handle)
    except (OSError, ValueError):
        return None
    if not isinstance(parsed, dict):
        return None
    return parsed


def _write_json(path: Path, record: dict[str, Any]) -> None:
    tmp_path = path.with_name(f"{path.name}.tmp")
    try:
        with open(tmp_path, "w", encoding="utf-8") as handle:
            handle.write(
                json.dumps(record, ensure_ascii=False, indent=2, sort_keys=True)
            )
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(tmp_path, path)
    except OSError:
        with contextlib.suppress(OSError):
            tmp_path.unlink(missing_ok=True)
        raise


def save(kind: str, record: dict[str, Any]) -> dict[str, Any]:
    """Persist one record, stamping `updated_at`."""
    with _LOCK:
        stamped = dict(record)
        stamped["updated_at"] = now_ms()
        _write_json(_state_path(kind, stamped["id"]), stamped)
        return stamped


def update(kind: str, record_id: str, **changes: Any) -> dict[str, Any] | None:
    """Merge `changes` into a stored record and return the new value."""
    with _LOCK:
        record = load(kind, record_id)
        if record is None:
            return None
        record.update(changes)
        return save(kind, record)


def load(kind: str, record_id: str) -> dict[str, Any] | None:
    return _read_json(_state_path(kind, record_id))


def list_records(kind: str) -> list[dict[str, Any]]:
    """Return every stored record of `kind`, newest first."""
    records: list[dict[str, Any]] = []
    with _LOCK:
        try:
            entries = list(settings.state_dir.iterdir())
        except OSError:
            return records
    for entry in entries:
        if not entry.name.startswith(f"{kind}-") or not entry.name.endswith(".json"):
            continue
        # A corrupt record must not break the whole listing; it is skipped and
        # left on disk for inspection.
        record = _read_json(entry)
        if record is not None:
            records.append(record)
    records.sort(key=lambda record: record.get("created_at", 0), reverse=True)
    return records


def delete(kind: str, record_id: str) -> None:
    with _LOCK, contextlib.suppress(OSError):
        _state_path(kind, record_id).unlink(missing_ok=True)
