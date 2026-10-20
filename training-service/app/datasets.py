"""Dataset intake: streaming upload, URL download, format detection and
LLaMA Factory dataset registration.

Every dataset lives in its own directory under `<DATA_ROOT>/datasets/<id>/`, so
row counting, preview and training-time normalisation can never escape the data
root even when a client supplies a hostile file name.
"""

from __future__ import annotations

import contextlib
import importlib
import json
import os
import re
import shutil
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from app import store
from app.settings import settings

ALLOWED_EXTENSIONS = frozenset(
    {".jsonl", ".json", ".csv", ".parquet", ".txt", ".zip", ".pdf", ".docx"}
)

# LLaMA Factory only reads two dataset shapes: alpaca-style column triples and
# sharegpt-style message lists. Anything else has to be normalised first.
FORMATTING_ALPACA = "alpaca"
FORMATTING_SHAREGPT = "sharegpt"

_INSTRUCTION_KEYS = ("instruction", "question", "prompt", "query", "input_text")
_INPUT_KEYS = ("input", "context", "passage")
_OUTPUT_KEYS = ("output", "answer", "response", "completion", "label")
_MESSAGE_KEYS = ("messages", "conversations")

_UNSAFE_NAME_CHARS = re.compile(r"[^A-Za-z0-9._-]+")

PREPARE_PENDING = "pending"
PREPARE_RUNNING = "running"
PREPARE_SUCCEEDED = "succeeded"
PREPARE_FAILED = "failed"
PREPARE_CANCELED = "canceled"


class DatasetError(ValueError):
    """Raised for unusable uploads or downloads."""


def safe_display_name(filename: str) -> str:
    """Return the file name as stored on disk: basename, no path segments."""
    base = os.path.basename((filename or "").replace("\\", "/")).strip()
    cleaned = _UNSAFE_NAME_CHARS.sub("_", base).strip("._")
    return cleaned[:120] or "dataset"


def _extension(filename: str) -> str:
    suffix = Path(safe_display_name(filename)).suffix.lower()
    if suffix not in ALLOWED_EXTENSIONS:
        raise DatasetError(
            f"unsupported file type {suffix or '(none)'}; allowed: "
            f"{', '.join(sorted(ALLOWED_EXTENSIONS))}"
        )
    return suffix


def _dataset_dir(dataset_id: str) -> Path:
    return settings.datasets_dir / store.require_valid_id(dataset_id)


def dataset_file_path(dataset_id: str, file_name: str) -> Path:
    """Resolve one stored dataset file.

    Both components come from the database and from a request path, so the id is
    validated and the name is re-sanitised: a crafted id or file name must never
    resolve outside `<DATA_ROOT>/datasets/`.
    """
    candidate = _dataset_dir(dataset_id) / safe_display_name(file_name)
    root = settings.datasets_dir.resolve()
    if not candidate.resolve().is_relative_to(root):
        raise DatasetError("dataset path escapes the data root")
    return candidate


def _write_stream(target: Path, chunks: Iterable[bytes]) -> int:
    """Write chunks to disk, aborting as soon as the size cap is exceeded."""
    total = 0
    tmp_path = target.with_name(f"{target.name}.part")
    try:
        with open(tmp_path, "wb") as handle:
            for chunk in chunks:
                if not chunk:
                    continue
                total += len(chunk)
                if total > settings.dataset_max_bytes:
                    raise DatasetError(
                        f"dataset exceeds the {settings.dataset_max_mb} MB limit"
                    )
                handle.write(chunk)
            handle.flush()
            os.fsync(handle.fileno())
        if total == 0:
            raise DatasetError("dataset is empty")
        os.replace(tmp_path, target)
    except OSError as exc:
        raise DatasetError(f"failed to store dataset: {exc}") from exc
    finally:
        with contextlib.suppress(OSError):
            tmp_path.unlink(missing_ok=True)
    return total


def _new_dataset_record(
    name: str, filename: str, source: str, source_ref: str
) -> dict[str, Any]:
    return {
        "id": store.new_id(),
        "name": name.strip()[:200] or filename,
        "file_name": filename,
        "source": source,
        "source_ref": source_ref[:1000],
        "size_bytes": 0,
        "rows": None,
        "format": Path(filename).suffix.lower().lstrip("."),
        "status": "ready",
        "error": "",
        "prepare": None,
        "created_at": store.now_ms(),
    }


def _finalize(record: dict[str, Any], path: Path) -> dict[str, Any]:
    record["size_bytes"] = path.stat().st_size
    record["rows"] = count_rows(path)
    return store.save(store.DATASET_KIND, record)


def register_prepared(source_path: Path, name: str, template: str) -> dict[str, Any]:
    """Adopt the output of a DataFlow run as a first-class dataset.

    The prepare run directory is temporary, so the produced JSONL is copied into
    a normal dataset directory and referenced exactly like an upload from then on.
    """
    if not source_path.is_file():
        raise DatasetError("the prepare run produced no output file")
    record = _new_dataset_record(name, "output.jsonl", "pipeline", template)
    directory = _dataset_dir(record["id"])
    try:
        directory.mkdir(parents=True, exist_ok=False)
    except OSError as exc:
        raise DatasetError(f"failed to create dataset directory: {exc}") from exc

    target = directory / "output.jsonl"
    try:
        with open(source_path, "rb") as reader, open(target, "wb") as writer:
            shutil.copyfileobj(reader, writer)
    except OSError as exc:
        store.delete(store.DATASET_KIND, record["id"])
        raise DatasetError(f"failed to store the prepare output: {exc}") from exc
    return _finalize(record, target)


def adopt_upload(name: str, filename: str, staged_path: Path) -> dict[str, Any]:
    """Adopt an already-staged upload as a dataset.

    The HTTP layer streams the request body into `<DATA_ROOT>/staging` while
    enforcing the size cap, then hands the temporary file here: the dataset
    record and its directory only exist once the whole body has been received.
    """
    _extension(filename)
    display_name = safe_display_name(filename)
    record = _new_dataset_record(name or display_name, display_name, "upload", "")
    directory = _dataset_dir(record["id"])
    try:
        directory.mkdir(parents=True, exist_ok=False)
    except OSError as exc:
        raise DatasetError(f"failed to create dataset directory: {exc}") from exc

    target = directory / display_name
    try:
        shutil.copyfile(staged_path, target)
    except OSError as exc:
        store.delete(store.DATASET_KIND, record["id"])
        raise DatasetError(f"failed to store dataset: {exc}") from exc
    if target.stat().st_size == 0:
        store.delete(store.DATASET_KIND, record["id"])
        raise DatasetError("dataset is empty")
    return _finalize(record, target)


def save_from_url(name: str, url: str) -> dict[str, Any]:
    """Download a dataset from an HTTP(S) URL with a hard size cap."""
    parsed = urllib.parse.urlparse(url)
    if parsed.scheme not in ("http", "https") or not parsed.netloc:
        raise DatasetError("dataset URL must be an absolute http(s) URL")

    filename = safe_display_name(
        urllib.parse.unquote(Path(parsed.path).name) or "dataset.jsonl"
    )
    _extension(filename)

    record = _new_dataset_record(name or filename, filename, "url", url)
    directory = _dataset_dir(record["id"])
    try:
        directory.mkdir(parents=True, exist_ok=False)
    except OSError as exc:
        raise DatasetError(f"failed to create dataset directory: {exc}") from exc

    # Only HTTP(S) handlers are installed on the opener, so a `file:` or custom
    # scheme cannot be honoured even though the scheme check above already
    # rejects one: the transport itself has no other protocol to speak.
    opener = urllib.request.build_opener(
        urllib.request.HTTPHandler(), urllib.request.HTTPSHandler()
    )
    opener.addheaders = [("User-Agent", "new-api-training-service")]
    try:
        with opener.open(url, timeout=settings.download_timeout_seconds) as response:
            _write_stream(
                directory / filename, iter(lambda: response.read(1 << 16), b"")
            )
    except DatasetError:
        store.delete(store.DATASET_KIND, record["id"])
        raise
    except (urllib.error.URLError, OSError) as exc:
        store.delete(store.DATASET_KIND, record["id"])
        raise DatasetError(f"failed to download dataset: {exc}") from exc
    return _finalize(record, directory / filename)


def _read_jsonl_sample(path: Path, limit: int = 20) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    try:
        with open(path, encoding="utf-8") as handle:
            for line in handle:
                text = line.strip()
                if not text:
                    continue
                try:
                    parsed = json.loads(text)
                except ValueError:
                    continue
                if isinstance(parsed, dict):
                    rows.append(parsed)
                if len(rows) >= limit:
                    break
    except OSError as exc:
        raise DatasetError(f"failed to read dataset: {exc}") from exc
    return rows


def count_rows(path: Path) -> int | None:
    """Count data rows; returns None when the format cannot be counted cheaply."""
    suffix = path.suffix.lower()
    if suffix == ".jsonl":
        total = 0
        try:
            with open(path, "rb") as handle:
                for chunk in iter(lambda: handle.readline(), b""):
                    if chunk.strip():
                        total += 1
        except OSError:
            return None
        return total
    if suffix == ".csv":
        try:
            with open(path, "rb") as handle:
                return max(
                    sum(1 for chunk in iter(lambda: handle.readline(), b"")) - 1, 0
                )
        except OSError:
            return None
    if suffix == ".parquet":
        # pyarrow is an optional dependency of DataFlow, so it is resolved at
        # runtime instead of being imported at module load time.
        try:
            parquet = importlib.import_module("pyarrow.parquet")
        except ImportError:
            return None
        try:
            return int(parquet.read_metadata(path).num_rows)
        except Exception:  # noqa: BLE001 - pyarrow raises its own error types
            return None
    return None


def preview(dataset_id: str, limit: int = 5) -> list[dict[str, Any]]:
    record = store.load(store.DATASET_KIND, dataset_id)
    if record is None:
        raise DatasetError("dataset not found")
    path = _dataset_dir(dataset_id) / record["file_name"]
    if path.suffix.lower() != ".jsonl":
        return []
    return _read_jsonl_sample(path, limit=limit)


def _detect_formatting(row: dict[str, Any]) -> str | None:
    if any(key in row for key in _MESSAGE_KEYS):
        return FORMATTING_SHAREGPT
    if any(key in row for key in _INSTRUCTION_KEYS) and any(
        key in row for key in _OUTPUT_KEYS
    ):
        return FORMATTING_ALPACA
    return None


def _pick(row: dict[str, Any], keys: tuple[str, ...]) -> str:
    for key in keys:
        value = row.get(key)
        if isinstance(value, str) and value.strip():
            return value
    return ""


def _normalize_row(row: dict[str, Any], formatting: str) -> dict[str, Any]:
    if formatting == FORMATTING_SHAREGPT:
        messages = row.get("messages") or row.get("conversations") or []
        return {"messages": messages}
    return {
        "instruction": _pick(row, _INSTRUCTION_KEYS),
        "input": _pick(row, _INPUT_KEYS),
        "output": _pick(row, _OUTPUT_KEYS),
    }


def build_training_dataset(
    dataset_id: str,
    destination: Path,
    dataset_key: str,
) -> dict[str, Any]:
    """Normalise a dataset into a LLaMA Factory dataset directory.

    Writes `<destination>/<dataset_key>.jsonl` plus the `dataset_info.json` entry
    that LLaMA Factory needs, and returns `{"dataset_dir", "dataset_name",
    "rows", "formatting"}`. The platform passes `dataset_key` so the registered
    dataset name matches the job it belongs to.
    """
    record = store.load(store.DATASET_KIND, dataset_id)
    if record is None:
        raise DatasetError("dataset not found")
    source = _dataset_dir(dataset_id) / record["file_name"]
    if source.suffix.lower() != ".jsonl":
        raise DatasetError(
            "only JSONL datasets can be used for training; run a DataFlow "
            "pipeline to convert this file first"
        )

    sample = _read_jsonl_sample(source, limit=20)
    if not sample:
        raise DatasetError("dataset has no readable JSONL rows")
    formatting = _detect_formatting(sample[0])
    if formatting is None:
        raise DatasetError(
            "unsupported dataset columns; expected instruction/input/output or messages"
        )

    destination.mkdir(parents=True, exist_ok=True)
    target = destination / f"{dataset_key}.jsonl"
    written = 0
    try:
        with (
            open(source, encoding="utf-8") as reader,
            open(target, "w", encoding="utf-8") as writer,
        ):
            for line in reader:
                text = line.strip()
                if not text:
                    continue
                try:
                    row = json.loads(text)
                except ValueError:
                    continue
                if not isinstance(row, dict):
                    continue
                writer.write(
                    json.dumps(_normalize_row(row, formatting), ensure_ascii=False)
                    + "\n"
                )
                written += 1
    except OSError as exc:
        raise DatasetError(f"failed to prepare dataset: {exc}") from exc
    if written == 0:
        raise DatasetError("dataset produced no usable rows")

    dataset_info: dict[str, Any] = {dataset_key: {"file_name": target.name}}
    if formatting == FORMATTING_SHAREGPT:
        dataset_info[dataset_key]["formatting"] = FORMATTING_SHAREGPT
        dataset_info[dataset_key]["columns"] = {"messages": "messages"}
    try:
        with open(destination / "dataset_info.json", "w", encoding="utf-8") as handle:
            json.dump(dataset_info, handle, ensure_ascii=False, indent=2)
    except OSError as exc:
        raise DatasetError(f"failed to write dataset_info.json: {exc}") from exc

    return {
        "dataset_dir": str(destination),
        "dataset_name": dataset_key,
        "rows": written,
        "formatting": formatting,
    }
