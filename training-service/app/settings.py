"""Runtime settings for the training service.

Every value comes from the process environment so the service can be deployed
on any GPU host without editing code. Paths are resolved once at import time and
validated eagerly: a misconfigured data root must fail at startup instead of on
the first training job.

Run from the `training-service/` directory:

    TRAINING_SERVICE_TOKEN=... DATA_ROOT=/var/lib/training \
    LLAMAFACTORY_HOME=/opt/LLaMA-Factory \
    uvicorn app.main:app --host 0.0.0.0 --port 8083
"""

from __future__ import annotations

import importlib.util
import os
import shutil
from dataclasses import dataclass
from pathlib import Path


def _env_str(name: str, default: str) -> str:
    value = os.environ.get(name, "").strip()
    return value or default


def _env_int(name: str, default: int, minimum: int = 1) -> int:
    raw = os.environ.get(name, "").strip()
    if not raw:
        return default
    try:
        value = int(raw)
    except ValueError as exc:
        raise RuntimeError(f"{name} must be an integer, got {raw!r}") from exc
    if value < minimum:
        raise RuntimeError(f"{name} must be >= {minimum}, got {value}")
    return value


@dataclass(frozen=True)
class Settings:
    """Immutable settings snapshot shared by every module."""

    token: str
    bind_host: str
    bind_port: int
    demo_mode: bool
    data_root: Path
    llamafactory_home: Path
    llamafactory_cli: str
    dataset_max_mb: int
    max_concurrent_jobs: int
    max_concurrent_prepares: int
    prepare_timeout_seconds: int
    download_timeout_seconds: int
    cancel_grace_seconds: int
    log_tail_bytes: int

    @property
    def datasets_dir(self) -> Path:
        return self.data_root / "datasets"

    @property
    def jobs_dir(self) -> Path:
        return self.data_root / "jobs"

    @property
    def state_dir(self) -> Path:
        return self.data_root / "state"

    @property
    def staging_dir(self) -> Path:
        return self.data_root / "staging"

    @property
    def dataset_max_bytes(self) -> int:
        return self.dataset_max_mb * 1024 * 1024

    def ensure_dirs(self) -> None:
        """Create the runtime tree and prove it is writable."""
        for path in (
            self.data_root,
            self.datasets_dir,
            self.jobs_dir,
            self.state_dir,
            self.staging_dir,
        ):
            path.mkdir(parents=True, exist_ok=True)
            probe = path / ".write-probe"
            try:
                probe.write_text("ok", encoding="utf-8")
            except OSError as exc:
                raise RuntimeError(
                    f"data directory {path} is not writable: {exc}"
                ) from exc
            finally:
                probe.unlink(missing_ok=True)


def resolve_bind_address() -> tuple[str, int]:
    """Return the address the HTTP server should listen on.

    Loopback by default: the platform reaches the service through the
    operator-provided URL, so a wildcard bind must be an explicit choice. Set
    `TRAINING_SERVICE_HOST=0.0.0.0` when the platform runs on another host, and
    keep that network trusted: the bearer token is the only authentication.
    """
    return (
        _env_str("TRAINING_SERVICE_HOST", "127.0.0.1"),
        _env_int("TRAINING_SERVICE_PORT", 8083, minimum=1),
    )


def load_settings() -> Settings:
    token = _env_str("TRAINING_SERVICE_TOKEN", "")
    if not token:
        raise RuntimeError(
            "TRAINING_SERVICE_TOKEN is required; the platform sends it as a Bearer token"
        )

    data_root = Path(_env_str("DATA_ROOT", "/var/lib/training")).expanduser()
    llamafactory_home = Path(
        _env_str("LLAMAFACTORY_HOME", str(Path.home() / "LlamaFactory"))
    ).expanduser()

    # `llamafactory-cli` must exist on PATH (or be an absolute path) unless the
    # service runs in demo mode; resolving it here turns a silent "job fails
    # after two minutes" into a startup error.
    llamafactory_cli = _env_str("LLAMAFACTORY_CLI", "llamafactory-cli")
    resolved_cli = shutil.which(llamafactory_cli)

    # Demo mode simulates data preparation and training with synthetic rows, so
    # the whole console flow can be exercised without a GPU or the two upstream
    # frameworks. DEMO_MODE decides explicitly; otherwise it is inferred from
    # the environment so a bare machine still starts.
    demo_env = os.environ.get("DEMO_MODE", "").strip().lower()
    if demo_env in ("1", "true", "yes", "on"):
        demo_mode = True
    elif demo_env in ("0", "false", "no", "off"):
        demo_mode = False
    else:
        demo_mode = (
            resolved_cli is None
            or importlib.util.find_spec("dataflow") is None
        )

    if not demo_mode and resolved_cli is None:
        raise RuntimeError(
            f"{llamafactory_cli!r} was not found on PATH; install LLaMA Factory, "
            "or start with DEMO_MODE=1 to run the console flow without it"
        )

    bind_host, bind_port = resolve_bind_address()
    settings = Settings(
        token=token,
        bind_host=bind_host,
        bind_port=bind_port,
        demo_mode=demo_mode,
        data_root=data_root,
        llamafactory_home=llamafactory_home,
        llamafactory_cli=resolved_cli or "",
        dataset_max_mb=_env_int("DATASET_MAX_MB", 200, minimum=1),
        max_concurrent_jobs=_env_int("MAX_CONCURRENT_JOBS", 1, minimum=1),
        max_concurrent_prepares=_env_int("MAX_CONCURRENT_PREPARES", 2, minimum=1),
        prepare_timeout_seconds=_env_int("PREPARE_TIMEOUT_SECONDS", 7200, minimum=60),
        download_timeout_seconds=_env_int("DOWNLOAD_TIMEOUT_SECONDS", 300, minimum=10),
        cancel_grace_seconds=_env_int("CANCEL_GRACE_SECONDS", 30, minimum=1),
        log_tail_bytes=_env_int("LOG_TAIL_BYTES", 262144, minimum=4096),
    )
    settings.ensure_dirs()
    return settings


settings = load_settings()
