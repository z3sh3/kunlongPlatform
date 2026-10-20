"""HTTP surface of the training service.

The platform (new-api) is the only client: every endpoint requires the shared
bearer token, and no endpoint is meant to be reachable from a browser. Responses
are plain JSON records owned by `store`; errors use a stable `{"code",
"message"}` envelope so the console can distinguish "not configured" from
"broken".
"""

from __future__ import annotations

import asyncio
import contextlib
import importlib.metadata
import json
import secrets
import subprocess
from collections.abc import AsyncIterator, Callable
from typing import Any

from fastapi import (
    Depends,
    FastAPI,
    File,
    Form,
    Header,
    Query,
    UploadFile,
)
from fastapi.responses import JSONResponse, StreamingResponse
from starlette.concurrency import run_in_threadpool

from app import datasets, logs, pipelines, store, training
from app.capabilities import InvalidPipelineParams, capabilities
from app.settings import settings

_UPLOAD_CHUNK_BYTES = 1 << 16
_BEARER_PREFIX = "Bearer "
_SSE_HEADERS = {
    "Cache-Control": "no-cache",
    "Connection": "keep-alive",
    "X-Accel-Buffering": "no",
}


class ApiError(Exception):
    """An error rendered as the `{"code","message"}` envelope."""

    def __init__(self, code: str, message: str, status_code: int = 400) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.status_code = status_code


def require_token(authorization: str = Header(default="")) -> None:
    """Reject any request that does not carry the shared bearer token."""
    if not authorization.startswith(_BEARER_PREFIX):
        raise ApiError("unauthorized", "missing bearer token", status_code=401)
    presented = authorization[len(_BEARER_PREFIX) :].strip()
    if not secrets.compare_digest(presented, settings.token):
        raise ApiError("unauthorized", "invalid bearer token", status_code=401)


app = FastAPI(
    title="new-api training service",
    version="v1",
    dependencies=[Depends(require_token)],
)


@app.exception_handler(ApiError)
async def _api_error_handler(_: Any, exc: ApiError) -> JSONResponse:
    return JSONResponse(
        status_code=exc.status_code, content={"code": exc.code, "message": exc.message}
    )


@app.exception_handler(datasets.DatasetError)
async def _dataset_error_handler(_: Any, exc: datasets.DatasetError) -> JSONResponse:
    return JSONResponse(
        status_code=400, content={"code": "invalid_dataset", "message": str(exc)}
    )


@app.exception_handler(pipelines.PrepareError)
async def _prepare_error_handler(_: Any, exc: pipelines.PrepareError) -> JSONResponse:
    return JSONResponse(
        status_code=400, content={"code": "prepare_failed", "message": str(exc)}
    )


@app.exception_handler(training.TrainingError)
async def _training_error_handler(_: Any, exc: training.TrainingError) -> JSONResponse:
    return JSONResponse(
        status_code=400, content={"code": "job_failed", "message": str(exc)}
    )


@app.exception_handler(InvalidPipelineParams)
async def _params_error_handler(_: Any, exc: InvalidPipelineParams) -> JSONResponse:
    return JSONResponse(
        status_code=400, content={"code": "invalid_params", "message": str(exc)}
    )


@app.exception_handler(store.InvalidRecordId)
async def _record_id_handler(_: Any, exc: store.InvalidRecordId) -> JSONResponse:
    return JSONResponse(
        status_code=400, content={"code": "invalid_id", "message": str(exc)}
    )


@app.exception_handler(Exception)
async def _unhandled_handler(_: Any, exc: Exception) -> JSONResponse:
    # Never leak a traceback to the platform: the detail is in the log file.
    print(f"[error] unhandled: {exc!r}", flush=True)
    return JSONResponse(
        status_code=500, content={"code": "internal_error", "message": "internal error"}
    )


def _dependency_version(distribution: str) -> str:
    try:
        return importlib.metadata.version(distribution)
    except importlib.metadata.PackageNotFoundError:
        return ""


def _gpus() -> list[dict[str, Any]]:
    """Best-effort GPU inventory for the console's service status panel."""
    try:
        result = subprocess.run(
            [
                "nvidia-smi",
                "--query-gpu=name,memory.total",
                "--format=csv,noheader,nounits",
            ],
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return []
    if result.returncode != 0:
        return []
    gpus: list[dict[str, Any]] = []
    for line in result.stdout.splitlines():
        name, _, memory = line.partition(",")
        if not name.strip():
            continue
        gpus.append({"name": name.strip(), "memory_mb": memory.strip()})
    return gpus


@app.get("/v1/health")
async def health() -> dict[str, Any]:
    return {
        "ok": True,
        "demo_mode": settings.demo_mode,
        "versions": {
            "dataflow": _dependency_version("open-dataflow"),
            "llamafactory": _dependency_version("llamafactory"),
        },
        "gpus": _gpus(),
        "limits": {
            "dataset_max_mb": settings.dataset_max_mb,
            "max_concurrent_jobs": settings.max_concurrent_jobs,
            "max_concurrent_prepares": settings.max_concurrent_prepares,
        },
        "active": {
            "jobs": training.active_job_count(),
            "prepares": pipelines.active_prepare_count(),
        },
    }


@app.get("/v1/capabilities")
async def get_capabilities() -> dict[str, Any]:
    return capabilities()


# ---------------------------------------------------------------------------
# Datasets
# ---------------------------------------------------------------------------


@app.post("/v1/datasets")
async def create_dataset(
    file: UploadFile = File(...),  # noqa: B008
    name: str = Form(default=""),
) -> dict[str, Any]:
    staged = settings.staging_dir / f"{store.new_id()}.part"
    total = 0
    try:
        try:
            handle = await run_in_threadpool(open, staged, "wb")
        except OSError as exc:
            raise datasets.DatasetError(f"failed to stage the upload: {exc}") from exc
        try:
            while chunk := await file.read(_UPLOAD_CHUNK_BYTES):
                total += len(chunk)
                if total > settings.dataset_max_bytes:
                    raise datasets.DatasetError(
                        f"dataset exceeds the {settings.dataset_max_mb} MB limit"
                    )
                await run_in_threadpool(handle.write, chunk)
        finally:
            await run_in_threadpool(handle.close)
        if total == 0:
            raise datasets.DatasetError("dataset is empty")
        return await run_in_threadpool(
            datasets.adopt_upload, name, file.filename or "dataset.jsonl", staged
        )
    finally:
        with contextlib.suppress(OSError):
            staged.unlink(missing_ok=True)


@app.post("/v1/datasets/from-url")
async def create_dataset_from_url(payload: dict[str, Any]) -> dict[str, Any]:
    url = str(payload.get("url") or "").strip()
    name = str(payload.get("name") or "").strip()
    if not url:
        raise datasets.DatasetError("url is required")
    return await run_in_threadpool(datasets.save_from_url, name, url)


@app.delete("/v1/datasets/{dataset_id}")
async def delete_dataset(dataset_id: str) -> dict[str, Any]:
    record = store.load(store.DATASET_KIND, dataset_id)
    if record is None:
        raise datasets.DatasetError("dataset not found")
    store.delete(store.DATASET_KIND, dataset_id)
    return {"deleted": dataset_id}


# ---------------------------------------------------------------------------
# DataFlow preparation
# ---------------------------------------------------------------------------


@app.post("/v1/datasets/{dataset_id}/prepare")
async def start_prepare(dataset_id: str, payload: dict[str, Any]) -> dict[str, Any]:
    llm = payload.get("llm") or {}
    if not isinstance(llm, dict):
        raise ApiError("invalid_request", "llm must be an object")
    return await run_in_threadpool(
        pipelines.start_prepare,
        dataset_id,
        str(payload.get("pipeline") or ""),
        payload.get("params") or {},
        {
            "api_key": str(llm.get("api_key") or ""),
            "base_url": str(llm.get("base_url") or ""),
            "model": str(llm.get("model") or ""),
        },
    )


@app.get("/v1/datasets/{dataset_id}/prepare/{run_id}")
async def get_prepare(dataset_id: str, run_id: str) -> dict[str, Any]:
    record = pipelines.get_prepare(run_id)
    if record is None or record.get("dataset_id") != dataset_id:
        raise ApiError("not_found", "prepare run not found", status_code=404)
    return record


@app.post("/v1/datasets/{dataset_id}/prepare/{run_id}/cancel")
async def cancel_prepare(dataset_id: str, run_id: str) -> dict[str, Any]:
    record = store.load(pipelines.PREPARE_KIND, run_id)
    if record is None or record.get("dataset_id") != dataset_id:
        raise ApiError("not_found", "prepare run not found", status_code=404)
    canceled = await run_in_threadpool(pipelines.cancel_prepare, run_id)
    return {"canceled": canceled}


# ---------------------------------------------------------------------------
# Training jobs
# ---------------------------------------------------------------------------


@app.post("/v1/jobs")
async def create_job(payload: dict[str, Any]) -> dict[str, Any]:
    return await run_in_threadpool(training.start_job, payload)


@app.get("/v1/jobs/{job_id}")
async def get_job(job_id: str) -> dict[str, Any]:
    record = training.get_job(job_id)
    if record is None:
        raise ApiError("not_found", "job not found", status_code=404)
    return record


@app.post("/v1/jobs/{job_id}/cancel")
async def cancel_job(job_id: str) -> dict[str, Any]:
    canceled = await run_in_threadpool(training.cancel_job, job_id)
    return {"canceled": canceled}


@app.delete("/v1/jobs/{job_id}")
async def delete_job(job_id: str) -> dict[str, Any]:
    deleted = await run_in_threadpool(training.delete_job, job_id)
    return {"deleted": deleted}


# ---------------------------------------------------------------------------
# Server-sent events
# ---------------------------------------------------------------------------


def _log_response(
    record: dict[str, Any],
    load: Callable[[], dict[str, Any] | None],
    offset: int,
) -> StreamingResponse:
    """Stream a log file until its owner reaches a terminal status."""
    path = store.Path(record["log_path"])

    async def events() -> AsyncIterator[str]:
        position = offset
        while True:
            frames, position = await run_in_threadpool(
                logs.read_new_lines, path, position
            )
            for frame in frames:
                yield f"data: {json.dumps(frame, ensure_ascii=False)}\n\n"
            current = load()
            if current is None or current.get("status") in (
                "succeeded",
                "failed",
                "canceled",
            ):
                frames, position = await run_in_threadpool(
                    logs.read_new_lines, path, position
                )
                for frame in frames:
                    yield f"data: {json.dumps(frame, ensure_ascii=False)}\n\n"
                yield "data: [DONE]\n\n"
                return
            await asyncio.sleep(0.5)

    return StreamingResponse(
        events(), media_type="text/event-stream", headers=_SSE_HEADERS
    )


@app.get("/v1/jobs/{job_id}/logs")
async def job_logs(
    job_id: str, offset: int = Query(default=0, ge=0)
) -> StreamingResponse:
    record = training.get_job(job_id)
    if record is None:
        raise ApiError("not_found", "job not found", status_code=404)
    return _log_response(record, lambda: store.load(training.JOB_KIND, job_id), offset)


@app.get("/v1/datasets/{dataset_id}/prepare/{run_id}/logs")
async def prepare_logs(
    dataset_id: str, run_id: str, offset: int = Query(default=0, ge=0)
) -> StreamingResponse:
    record = pipelines.get_prepare(run_id)
    if record is None or record.get("dataset_id") != dataset_id:
        raise ApiError("not_found", "prepare run not found", status_code=404)
    return _log_response(
        record, lambda: store.load(pipelines.PREPARE_KIND, run_id), offset
    )


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host=settings.bind_host, port=settings.bind_port)
