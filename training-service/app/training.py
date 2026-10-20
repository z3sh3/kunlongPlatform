"""LLaMA Factory fine-tuning job management.

Like the DataFlow runner, every job is a detached process
(`python -m app.training --job-id X`). That process prepares the dataset
directory, renders the training YAML, runs `llamafactory-cli train`, and writes
the terminal status plus the artifact scan when the CLI exits.

Method mapping to LLaMA Factory's `finetuning_type`:

    lora  -> lora    (LoRA adapters, the default for limited GPUs)
    sft   -> freeze  (head-only tuning between LoRA and full fine-tuning)
    full  -> full    (full-parameter fine-tuning)

`stage` is always `sft` in this service; DPO/PPO datasets and reward models are
out of scope for the platform's model training module.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

from app import datasets, store
from app.capabilities import validate_hyper_params
from app.settings import settings

JOB_KIND = "job"

METHOD_TO_FINETUNING_TYPE = {
    "lora": "lora",
    "sft": "freeze",
    "full": "full",
}

TERMINAL_STATUSES = frozenset({"succeeded", "failed", "canceled"})

_PERCENT_PATTERN = re.compile(r"(\d{1,3})%\|")
_EPOCH_PATTERN = re.compile(r"'epoch':\s*([0-9.]+)")
_TRAIN_RUNTIME_PATTERN = re.compile(r"'train_runtime':\s*([0-9.]+)")

# Artifact detection: LoRA saves an adapter config, full/freeze saves weights.
_ADAPTER_MARKERS = ("adapter_config.json", "adapter_model.safetensors")
_WEIGHT_PATTERNS = ("*.safetensors", "*.bin")


class TrainingError(RuntimeError):
    """Raised when a job cannot be created or a request is invalid."""


def _job_root(job_id: str) -> Path:
    return settings.jobs_dir / store.require_valid_id(job_id)


def _log_path(job_id: str) -> Path:
    return _job_root(job_id) / "train.log"


def _service_root() -> Path:
    return Path(__file__).resolve().parent.parent


def active_job_count() -> int:
    return sum(
        1
        for record in store.list_records(JOB_KIND)
        if record.get("status") in ("pending", "running")
    )


def start_job(payload: dict[str, Any]) -> dict[str, Any]:
    """Validate a job request and spawn its training process."""
    name = (payload.get("name") or "").strip()
    if not name:
        raise TrainingError("job name is required")
    base_model = (payload.get("base_model") or "").strip()
    if not base_model:
        raise TrainingError("base model is required")
    method = (payload.get("method") or "").strip().lower()
    if method not in METHOD_TO_FINETUNING_TYPE:
        raise TrainingError(f"unsupported method {method!r}")

    dataset_ref = (payload.get("dataset_id") or "").strip()
    if not dataset_ref:
        raise TrainingError("a prepared dataset is required")
    dataset = store.load(store.DATASET_KIND, dataset_ref)
    if dataset is None:
        raise TrainingError("dataset not found")
    if (dataset.get("prepare") or {}).get("status") in ("pending", "running"):
        raise TrainingError("the dataset is still being prepared")

    hyper_params = validate_hyper_params(payload.get("hyper_params"))
    template = (payload.get("template") or "").strip()

    if active_job_count() >= settings.max_concurrent_jobs:
        raise TrainingError("the service is already running the maximum number of jobs")

    job_id = store.new_id()
    root = _job_root(job_id)
    try:
        root.mkdir(parents=True, exist_ok=False)
    except OSError as exc:
        raise TrainingError(f"failed to create the job directory: {exc}") from exc

    record = store.save(
        JOB_KIND,
        {
            "id": job_id,
            "name": name[:200],
            "base_model": base_model,
            "method": method,
            "dataset_id": dataset_ref,
            "dataset_name": dataset.get("name", ""),
            "template": template,
            "hyper_params": hyper_params,
            "status": "pending",
            "progress": 0,
            "error": "",
            "log_path": str(_log_path(job_id)),
            "output_dir": str(root / "output"),
            "pid": 0,
            "artifact": None,
            "usage": None,
            "created_at": store.now_ms(),
        },
    )

    try:
        log_file = open(  # noqa: SIM115 - handed to the child process
            _log_path(job_id), "ab"
        )
    except OSError as exc:
        store.update(JOB_KIND, job_id, status="failed", error=str(exc))
        raise TrainingError(f"failed to open the training log: {exc}") from exc

    try:
        process = subprocess.Popen(
            [sys.executable, "-m", "app.training", "--job-id", job_id],
            cwd=str(_service_root()),
            env={**os.environ, "PYTHONUNBUFFERED": "1"},
            stdout=log_file,
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )
    except OSError as exc:
        log_file.close()
        store.update(JOB_KIND, job_id, status="failed", error=str(exc))
        raise TrainingError(f"failed to start the training process: {exc}") from exc
    finally:
        log_file.close()

    return (
        store.update(
            JOB_KIND,
            job_id,
            pid=process.pid,
            status="running",
            started_at=store.now_ms(),
        )
        or record
    )


def get_job(job_id: str) -> dict[str, Any] | None:
    """Return a job, refreshing progress and detecting an unexpected exit."""
    record = store.load(JOB_KIND, job_id)
    if record is None:
        return None
    if record.get("status") in TERMINAL_STATUSES:
        return record

    pid = record.get("pid") or 0
    if pid and not _pid_alive(pid):
        return (
            store.update(
                JOB_KIND,
                job_id,
                status="failed",
                error="the training process exited unexpectedly; see the log",
                finished_at=store.now_ms(),
            )
            or record
        )

    progress = _progress_from_log(
        Path(record["log_path"]),
        float(record.get("hyper_params", {}).get("num_train_epochs") or 1),
    )
    if progress > int(record.get("progress") or 0):
        return store.update(JOB_KIND, job_id, progress=progress) or record
    return record


def cancel_job(job_id: str) -> bool:
    record = store.load(JOB_KIND, job_id)
    if record is None or record.get("status") in TERMINAL_STATUSES:
        return False
    _terminate(record.get("pid") or 0)
    store.update(
        JOB_KIND,
        job_id,
        status="canceled",
        error="canceled by the operator",
        finished_at=store.now_ms(),
    )
    return True


def delete_job(job_id: str) -> bool:
    record = store.load(JOB_KIND, job_id)
    if record is None:
        return False
    if record.get("status") not in TERMINAL_STATUSES:
        raise TrainingError("stop the job before deleting it")
    root = _job_root(job_id)
    if root.is_dir():
        try:
            shutil.rmtree(root)
        except OSError as exc:
            raise TrainingError(f"failed to remove the job directory: {exc}") from exc
    store.delete(JOB_KIND, job_id)
    return True


def _pid_alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def _terminate(pid: int) -> None:
    if not pid:
        return
    for sig in (signal.SIGTERM, signal.SIGKILL):
        try:
            os.killpg(os.getpgid(pid), sig)
        except (ProcessLookupError, PermissionError):
            return
        deadline = time.monotonic() + settings.cancel_grace_seconds
        while time.monotonic() < deadline:
            if not _pid_alive(pid):
                return
            time.sleep(0.5)


def _progress_from_log(log_path: Path, total_epochs: float) -> int:
    """Estimate job progress (0-99) from the tail of the training log."""
    try:
        size = log_path.stat().st_size
        with open(log_path, "rb") as handle:
            if size > settings.log_tail_bytes:
                handle.seek(size - settings.log_tail_bytes)
            tail = handle.read().decode("utf-8", errors="replace")
    except OSError:
        return 0

    percentages = _PERCENT_PATTERN.findall(tail)
    if percentages:
        return max(0, min(99, int(percentages[-1])))

    epochs = _EPOCH_PATTERN.findall(tail)
    if epochs and total_epochs > 0:
        try:
            latest = float(epochs[-1])
        except ValueError:
            return 0
        return max(0, min(99, int(latest / total_epochs * 100)))
    return 0


def _yaml_scalar(value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (int, float)):
        return repr(value)
    text = str(value)
    if text == "" or re.search(r"[^A-Za-z0-9_./+-]", text) or text[0] in "0123456789":
        return json.dumps(text, ensure_ascii=False)
    return text


def render_train_yaml(
    record: dict[str, Any], dataset_dir: Path, dataset_name: str
) -> str:
    """Render the LLaMA Factory training YAML for one job."""
    hyper = record["hyper_params"]
    finetuning_type = METHOD_TO_FINETUNING_TYPE[record["method"]]
    lines = [
        "### model",
        f"model_name_or_path: {_yaml_scalar(record['base_model'])}",
        "trust_remote_code: true",
        "",
        "### method",
        "stage: sft",
        "do_train: true",
        f"finetuning_type: {finetuning_type}",
    ]
    if finetuning_type == "lora":
        lines += [
            f"lora_rank: {hyper['lora_rank']}",
            "lora_target: all",
        ]
    lines += [
        "",
        "### dataset",
        f"dataset: {_yaml_scalar(dataset_name)}",
        f"dataset_dir: {_yaml_scalar(str(dataset_dir))}",
        f"cutoff_len: {hyper['cutoff_len']}",
        "preprocessing_num_workers: 8",
        "dataloader_num_workers: 4",
        "",
        "### output",
        f"output_dir: {_yaml_scalar(record['output_dir'])}",
        "logging_steps: 5",
        "save_steps: 200",
        "plot_loss: true",
        "overwrite_output_dir: true",
        "save_only_model: false",
        "report_to: none",
        "",
        "### train",
        f"per_device_train_batch_size: {hyper['per_device_train_batch_size']}",
        f"gradient_accumulation_steps: {hyper['gradient_accumulation_steps']}",
        f"learning_rate: {_yaml_scalar(hyper['learning_rate'])}",
        f"num_train_epochs: {_yaml_scalar(hyper['num_train_epochs'])}",
        f"lr_scheduler_type: {hyper['lr_scheduler_type']}",
        "warmup_ratio: 0.1",
        f"bf16: {'true' if hyper['bf16'] else 'false'}",
        "ddp_timeout: 180000000",
        "",
    ]
    return "\n".join(lines)


def scan_artifact(output_dir: Path) -> dict[str, Any] | None:
    """Describe the trained artifact, or None when nothing was written."""
    if not output_dir.is_dir():
        return None
    size_bytes = 0
    file_count = 0
    kind = ""
    found_markers: set[str] = set()
    weight_suffixes = {pattern.lstrip("*") for pattern in _WEIGHT_PATTERNS}
    for entry in output_dir.rglob("*"):
        # `rglob` can descend into resharded checkpoint trees; the platform only
        # needs totals plus a kind hint, so a full recursive walk is enough.
        if not entry.is_file():
            continue
        file_count += 1
        try:
            size_bytes += entry.stat().st_size
        except OSError:
            continue
        if entry.name in _ADAPTER_MARKERS:
            found_markers.add(entry.name)
        if entry.suffix in weight_suffixes:
            kind = kind or "full"
    if found_markers:
        kind = "lora"
    if size_bytes == 0:
        return None
    return {
        "output_dir": str(output_dir),
        "kind": kind or "unknown",
        "file_count": file_count,
        "size_bytes": size_bytes,
    }


# ---------------------------------------------------------------------------
# Child process: prepares the dataset, runs llamafactory-cli, records the result
# ---------------------------------------------------------------------------


def run_child(job_id: str) -> int:
    record = store.load(JOB_KIND, job_id)
    if record is None:
        print(f"[train] unknown job {job_id}", flush=True)
        return 2

    root = _job_root(job_id)
    dataset_key = f"newapi_{job_id}"
    try:
        print(f"[train] preparing dataset {record['dataset_id']}", flush=True)
        prepared = datasets.build_training_dataset(
            record["dataset_id"], root / "data", dataset_key
        )
        print(
            f"[train] dataset {prepared['dataset_name']} ready: "
            f"{prepared['rows']} rows ({prepared['formatting']})",
            flush=True,
        )

        config_path = root / "train.yaml"
        try:
            config_path.write_text(
                render_train_yaml(
                    record, Path(prepared["dataset_dir"]), prepared["dataset_name"]
                ),
                encoding="utf-8",
            )
        except OSError as exc:
            raise TrainingError(f"failed to write the training config: {exc}") from exc

        store.update(
            JOB_KIND,
            job_id,
            started_at=store.now_ms(),
            progress=1,
            dataset_rows=prepared["rows"],
        )
        return_code = _execute_training(config_path, record)

        artifact = scan_artifact(Path(record["output_dir"]))
        current = store.load(JOB_KIND, job_id) or record
        started_at = current.get("started_at") or store.now_ms()
        usage = {
            "gpu_seconds": max(int((store.now_ms() - started_at) / 1000), 0),
            "rows": prepared["rows"],
            "epochs": record["hyper_params"]["num_train_epochs"],
        }
        runtime = _train_runtime(Path(record["log_path"]))
        if runtime is not None:
            usage["train_runtime_seconds"] = runtime

        if return_code != 0:
            store.update(
                JOB_KIND,
                job_id,
                status="failed",
                error=f"llamafactory-cli exited with code {return_code}",
                finished_at=store.now_ms(),
                usage=usage,
                artifact=artifact,
            )
            return return_code or 1

        if artifact is None:
            store.update(
                JOB_KIND,
                job_id,
                status="failed",
                error="training finished but no artifact was written to output_dir",
                finished_at=store.now_ms(),
                usage=usage,
            )
            return 1

        store.update(
            JOB_KIND,
            job_id,
            status="succeeded",
            progress=100,
            finished_at=store.now_ms(),
            artifact=artifact,
            usage=usage,
        )
        print(
            f"[train] succeeded: {artifact['kind']} artifact at {artifact['output_dir']}",
            flush=True,
        )
        return 0
    except BaseException as exc:  # noqa: BLE001 - the child must always record a status
        message = str(exc).strip() or exc.__class__.__name__
        print(f"[train] failed: {message}", flush=True)
        store.update(
            JOB_KIND,
            job_id,
            status="failed",
            error=message[:2000],
            finished_at=store.now_ms(),
        )
        return 1


def _train_runtime(log_path: Path) -> float | None:
    try:
        with open(log_path, "rb") as handle:
            handle.seek(0, os.SEEK_END)
            size = handle.tell()
            handle.seek(max(0, size - settings.log_tail_bytes))
            tail = handle.read().decode("utf-8", errors="replace")
    except OSError:
        return None
    matches = _TRAIN_RUNTIME_PATTERN.findall(tail)
    if not matches:
        return None
    try:
        return float(matches[-1])
    except ValueError:
        return None


# ---------------------------------------------------------------------------
# Trainer invocation
# ---------------------------------------------------------------------------


def _execute_training(config_path: Path, record: dict[str, Any]) -> int:
    """Run the real trainer, or simulate it when the service is in demo mode."""
    if settings.demo_mode:
        return _simulate_training(record)

    argv = [settings.llamafactory_cli, "train", str(config_path)]
    print(f"[train] {' '.join(argv)}", flush=True)
    completed = subprocess.run(  # noqa: S603 - fixed argv, no shell
        argv,
        cwd=str(settings.llamafactory_home),
        check=False,
    )
    return completed.returncode


def _simulate_training(record: dict[str, Any]) -> int:
    """Emit realistic log output and a placeholder artifact without a GPU.

    The log lines deliberately follow LLaMA Factory's shape so progress parsing,
    the SSE log stream and artifact detection all run for real.
    """
    output_dir = Path(record["output_dir"])
    total_steps = 20
    print("[train] demo mode: simulating LLaMA Factory training (no GPU)", flush=True)
    print("***** Running training *****", flush=True)
    print(f"  Num examples = {record.get('dataset_rows') or 0}", flush=True)
    for step in range(1, total_steps + 1):
        percent = int(step / total_steps * 100)
        loss = max(0.12, 2.4 - step * 0.11)
        epochs = record["hyper_params"]["num_train_epochs"]
        epoch = round(step / total_steps * float(epochs), 2)
        print({"loss": round(loss, 4), "epoch": epoch, "step": step}, flush=True)
        bar = "#" * (percent // 5)
        print(f"{percent}%|{bar:<20}| {step}/{total_steps} [00:0{step % 10}<00:00]", flush=True)
        time.sleep(0.15)

    try:
        output_dir.mkdir(parents=True, exist_ok=True)
        (output_dir / "adapter_config.json").write_text(
            json.dumps(
                {
                    "peft_type": "LORA",
                    "r": record["hyper_params"]["lora_rank"],
                    "demo": True,
                },
                indent=2,
            ),
            encoding="utf-8",
        )
        (output_dir / "adapter_model.safetensors").write_bytes(
            b"new-api training demo artifact\n"
        )
        (output_dir / "trainer_state.json").write_text(
            json.dumps({"global_step": total_steps, "demo": True}, indent=2),
            encoding="utf-8",
        )
    except OSError as exc:
        print(f"[train] failed to write the demo artifact: {exc}", flush=True)
        return 1

    print(f"[train] demo mode finished; placeholder artifact in {output_dir}", flush=True)
    return 0


def main() -> int:
    import argparse

    parser = argparse.ArgumentParser(description="Run one LLaMA Factory training job")
    parser.add_argument("--job-id", required=True)
    args = parser.parse_args()
    return run_child(args.job_id)


if __name__ == "__main__":
    sys.exit(main())
