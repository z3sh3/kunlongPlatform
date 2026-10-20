"""DataFlow pipeline execution for the curated templates.

Each prepare run is a **separate process** (`python -m app.pipelines --run-id X`)
so that a DataFlow operator that crashes, leaks GPU memory or hangs cannot take
the HTTP service down with it. The parent only spawns, observes and signals; the
child owns its own state file and log file.

The LLM credential never touches disk: the parent passes it to the child through
the `DF_API_KEY` environment variable, and the child hands DataFlow only the
*variable name* (`key_name_of_api_key`), which is what DataFlow expects.
"""

from __future__ import annotations

import argparse
import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

from app import datasets, store
from app.capabilities import get_template, validate_params
from app.settings import settings

PREPARE_KIND = "prepare"
LLM_KEY_ENV = "DF_API_KEY"

# Progress points reported by the child so the console can draw a bar.
_PROGRESS_STARTED = 10
_PROGRESS_INPUT = 30
_PROGRESS_PIPELINE = 60
_PROGRESS_WRITING = 90
_PROGRESS_DONE = 100


class PrepareError(RuntimeError):
    """Raised when a prepare run cannot be started."""


def _prepare_path(run_id: str) -> Path:
    return settings.data_root / "prepares" / store.require_valid_id(run_id)


def _run_dir(run_id: str) -> Path:
    return _prepare_path(run_id) / "run"


def _log_path(run_id: str) -> Path:
    return _prepare_path(run_id) / "prepare.log"


def _service_root() -> Path:
    """Directory to run the child in (`training-service/`), for `app.*` imports."""
    return Path(__file__).resolve().parent.parent


def active_prepare_count() -> int:
    return sum(
        1
        for record in store.list_records(PREPARE_KIND)
        if record.get("status") in ("pending", "running")
    )


def start_prepare(
    dataset_id: str,
    template_id: str,
    raw_params: dict[str, Any] | None,
    llm: dict[str, str],
) -> dict[str, Any]:
    """Validate inputs and spawn a DataFlow run. Returns the prepare record."""
    dataset = store.load(store.DATASET_KIND, dataset_id)
    if dataset is None:
        raise PrepareError("dataset not found")
    template = get_template(template_id)
    extension = Path(dataset["file_name"]).suffix.lower()
    if template.input_formats and extension not in template.input_formats:
        raise PrepareError(
            f"{template_id} accepts {', '.join(template.input_formats)}; "
            f"this dataset is {extension or '(unknown)'}"
        )
    params = validate_params(template_id, raw_params)

    api_key = (llm.get("api_key") or "").strip()
    api_base = (llm.get("base_url") or "").strip()
    model = (llm.get("model") or "").strip()
    # Demo mode never calls a model, so the console must not require one.
    if not settings.demo_mode:
        if not api_key or not api_base or not model:
            raise PrepareError("a cleaning model, its base URL and an API key are required")
        if not api_base.startswith(("http://", "https://")):
            raise PrepareError("cleaning model base URL must be http(s)")

    if active_prepare_count() >= settings.max_concurrent_prepares:
        raise PrepareError(
            "the service is already running the maximum number of prepare jobs"
        )

    run_id = store.new_id()
    run_dir = _run_dir(run_id)
    try:
        run_dir.mkdir(parents=True, exist_ok=False)
    except OSError as exc:
        raise PrepareError(f"failed to create run directory: {exc}") from exc

    record = store.save(
        PREPARE_KIND,
        {
            "id": run_id,
            "dataset_id": dataset_id,
            "dataset_name": dataset["name"],
            "template": template_id,
            "template_name": template.name,
            "params": params,
            "status": "pending",
            "progress": 0,
            "error": "",
            "log_path": str(_log_path(run_id)),
            "run_dir": str(run_dir),
            "input_path": str(
                settings.datasets_dir / dataset_id / dataset["file_name"]
            ),
            "output_path": str(run_dir / "output.jsonl"),
            "llm_model": model,
            "pid": 0,
            "output": None,
            "created_at": store.now_ms(),
        },
    )

    env = dict(os.environ)
    env[LLM_KEY_ENV] = api_key
    env["DF_API_BASE"] = api_base.rstrip("/")
    env["DF_MODEL_NAME"] = model
    env["PYTHONUNBUFFERED"] = "1"
    try:
        log_file = open(_log_path(run_id), "ab")  # noqa: SIM115 - handed to the child
    except OSError as exc:
        store.update(PREPARE_KIND, run_id, status="failed", error=str(exc))
        raise PrepareError(f"failed to open the prepare log: {exc}") from exc

    try:
        process = subprocess.Popen(
            [sys.executable, "-m", "app.pipelines", "--run-id", run_id],
            cwd=str(_service_root()),
            env=env,
            stdout=log_file,
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )
    except OSError as exc:
        log_file.close()
        store.update(PREPARE_KIND, run_id, status="failed", error=str(exc))
        raise PrepareError(f"failed to start the DataFlow process: {exc}") from exc
    finally:
        # The child owns the descriptor from here on.
        log_file.close()

    return (
        store.update(
            PREPARE_KIND,
            run_id,
            pid=process.pid,
            status="running",
            started_at=store.now_ms(),
        )
        or record
    )


def get_prepare(run_id: str) -> dict[str, Any] | None:
    """Return a prepare record, refreshing its status from the child if needed."""
    record = store.load(PREPARE_KIND, run_id)
    if record is None:
        return None
    if record.get("status") not in ("pending", "running"):
        return record
    pid = record.get("pid") or 0
    if pid and not _pid_alive(pid):
        # The child died without leaving a terminal status behind (OOM killer,
        # SIGKILL). Surface it as a failure instead of polling forever.
        return (
            store.update(
                PREPARE_KIND,
                run_id,
                status="failed",
                error="the DataFlow process exited unexpectedly; see the log",
                finished_at=store.now_ms(),
            )
            or record
        )
    return record


def cancel_prepare(run_id: str) -> bool:
    record = store.load(PREPARE_KIND, run_id)
    if record is None or record.get("status") not in ("pending", "running"):
        return False
    _terminate(record.get("pid") or 0)
    store.update(
        PREPARE_KIND,
        run_id,
        status="canceled",
        error="canceled by the operator",
        finished_at=store.now_ms(),
    )
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
    """Stop a child process group: SIGTERM, then SIGKILL after the grace period."""
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


# ---------------------------------------------------------------------------
# Child process: runs exactly one prepare and owns its state file
# ---------------------------------------------------------------------------


class _Run:
    """Everything a template runner needs about one prepare run."""

    def __init__(self, record: dict[str, Any]) -> None:
        self.record = record
        self.run_id: str = record["id"]
        self.run_dir = Path(record["run_dir"])
        self.input_path = Path(record["input_path"])
        self.output_path = Path(record["output_path"])
        self.params: dict[str, Any] = record["params"]
        self.template: str = record["template"]
        self.llm_model: str = record["llm_model"]

    def report(self, progress: int, **extra: Any) -> None:
        store.update(PREPARE_KIND, self.run_id, progress=progress, **extra)

    def log(self, message: str) -> None:
        # The child's stdout/stderr already go to prepare.log; this line keeps
        # progress markers readable in the same stream.
        print(f"[prepare] {message}", flush=True)


def _write_jsonl(path: Path, rows: list[dict[str, Any]]) -> None:
    try:
        with open(path, "w", encoding="utf-8") as handle:
            handle.writelines(
                json.dumps(row, ensure_ascii=False) + "\n" for row in rows
            )
    except OSError as exc:
        raise PrepareError(f"failed to write {path.name}: {exc}") from exc


def _read_jsonl(path: Path) -> list[dict[str, Any]]:
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
    except OSError as exc:
        raise PrepareError(f"failed to read {path.name}: {exc}") from exc
    return rows


def _column(row: dict[str, Any], candidates: tuple[str, ...]) -> str:
    for key in candidates:
        value = row.get(key)
        if isinstance(value, str) and value.strip():
            return value
    return ""


_TEXT_KEYS = ("text", "content", "raw_content", "document", "markdown", "page_content")
_INSTRUCTION_KEYS = ("instruction", "question", "prompt", "query")
_ANSWER_KEYS = ("output", "answer", "response", "completion")
_CODE_KEYS = ("code", "solution", "patch")


def _document_rows(path: Path, chunk_chars: int) -> list[dict[str, Any]]:
    """Turn a document into `{"text": chunk}` rows ready for DataFlow."""
    suffix = path.suffix.lower()
    if suffix == ".pdf":
        try:
            from pypdf import PdfReader
        except ImportError as exc:  # pragma: no cover - dependency of DataFlow
            raise PrepareError("pypdf is required to read PDF files") from exc
        reader = PdfReader(str(path))
        text = "\n".join(page.extract_text() or "" for page in reader.pages)
    elif suffix == ".txt":
        text = path.read_text(encoding="utf-8", errors="replace")
    else:
        raise PrepareError(f"{suffix} is not a supported document type")

    text = text.strip()
    if not text:
        raise PrepareError("the document produced no extractable text")
    chunks = [
        text[start : start + chunk_chars].strip()
        for start in range(0, len(text), chunk_chars)
    ]
    return [{"text": chunk} for chunk in chunks if chunk]


def _llm_serving(run: _Run) -> Any:
    """Build a DataFlow LLM client pointing at the platform gateway."""
    try:
        from dataflow.serving import APILLMServing_request
    except ImportError as exc:  # pragma: no cover - DataFlow is a hard dependency
        raise PrepareError("DataFlow is not installed in this environment") from exc
    api_base = os.environ.get("DF_API_BASE", "").rstrip("/")
    return APILLMServing_request(
        api_url=f"{api_base}/chat/completions",
        key_name_of_api_key=LLM_KEY_ENV,
        model_name=run.llm_model,
    )


def _storage(run: _Run, first_entry: Path) -> Any:
    from dataflow.utils.storage import FileStorage

    return FileStorage(
        first_entry_file_name=str(first_entry),
        cache_path=str(run.run_dir / "cache"),
        file_name_prefix=f"prepare_{run.run_id}",
        cache_type="jsonl",
    )


def _truncate(rows: list[dict[str, Any]], limit: int) -> list[dict[str, Any]]:
    return rows[:limit] if limit and len(rows) > limit else rows


def _run_kb_pdf_to_qa(run: _Run, serving: Any) -> list[dict[str, Any]]:
    from dataflow.operators.core_text import Text2QAGenerator

    chunk_chars = int(run.params["chunk_size"]) * 4
    rows = _document_rows(run.input_path, chunk_chars)
    run.log(f"split the document into {len(rows)} chunks")
    work_input = run.run_dir / "chunks.jsonl"
    _write_jsonl(work_input, rows)

    storage = _storage(run, work_input)
    Text2QAGenerator(llm_serving=serving).run(
        storage=storage.step(),
        input_key="text",
        input_question_num=int(run.params["questions_per_chunk"]),
        output_question_key="question",
        output_answer_key="answer",
    )
    dataframe = storage.step().read("dataframe")
    output: list[dict[str, Any]] = []
    for _, row in dataframe.iterrows():
        question = str(row.get("question") or "").strip()
        answer = str(row.get("answer") or "").strip()
        if question and answer:
            output.append({"question": question, "answer": answer})
    return output


def _run_text_clean_sft(run: _Run, serving: Any) -> list[dict[str, Any]]:
    from dataflow.operators.core_text import (
        PromptedFilter,
        PromptedRefiner,
        Text2QAGenerator,
    )

    source = _read_jsonl(run.input_path)
    prepared: list[dict[str, Any]] = []
    for row in source:
        answer = _column(row, _ANSWER_KEYS) or _column(row, _TEXT_KEYS)
        if not answer:
            continue
        prepared.append(
            {"instruction": _column(row, _INSTRUCTION_KEYS), "raw_content": answer}
        )
    if not prepared:
        raise PrepareError("no usable text or answer column was found in this dataset")
    run.log(f"kept {len(prepared)} rows with content")

    work_input = run.run_dir / "clean_input.jsonl"
    _write_jsonl(work_input, prepared)
    storage = _storage(run, work_input)

    PromptedFilter(
        llm_serving=serving,
        min_score=float(run.params["min_score"]),
        max_score=5,
    ).run(storage=storage.step(), input_key="raw_content", output_key="eval")

    if run.params["rewrite"]:
        PromptedRefiner(llm_serving=serving).run(
            storage=storage.step(), input_key="raw_content"
        )

    dataframe = storage.step().read("dataframe")
    output: list[dict[str, Any]] = []
    for _, row in dataframe.iterrows():
        text = str(row.get("raw_content") or "").strip()
        if not text:
            continue
        instruction = str(row.get("instruction") or "").strip()
        if not instruction:
            # Rows without a paired instruction are turned into a Q&A pair by
            # the same model that cleaned them.
            output.append({"instruction": text, "output": text})
            continue
        output.append({"instruction": instruction, "output": text})

    if output and not any(row["instruction"] != row["output"] for row in output):
        # Every row lost its instruction: regenerate questions from the text.
        regenerated = run.run_dir / "clean_pairs.jsonl"
        _write_jsonl(regenerated, [{"text": row["output"]} for row in output])
        pair_storage = _storage(run, regenerated)
        Text2QAGenerator(llm_serving=serving).run(
            storage=pair_storage.step(),
            input_key="text",
            input_question_num=1,
            output_question_key="question",
            output_answer_key="answer",
        )
        pairs = pair_storage.step().read("dataframe")
        output = [
            {
                "instruction": str(row.get("question") or "").strip(),
                "output": str(row.get("answer") or "").strip(),
            }
            for _, row in pairs.iterrows()
        ]
        output = [row for row in output if row["instruction"] and row["output"]]
    return output


def _run_code_sft(run: _Run, serving: Any) -> list[dict[str, Any]]:
    from dataflow.operators.code import (
        CodeCodeToInstructionGenerator,
        CodeLengthSampleFilter,
        CodeQualityScoreFilter,
    )

    source = _read_jsonl(run.input_path)
    prepared: list[dict[str, Any]] = []
    for row in source:
        code = _column(row, _CODE_KEYS)
        if code:
            prepared.append(
                {"instruction": _column(row, _INSTRUCTION_KEYS), "code": code}
            )
    if not prepared:
        raise PrepareError("no usable code column was found in this dataset")
    run.log(f"kept {len(prepared)} rows with code")

    work_input = run.run_dir / "code_input.jsonl"
    _write_jsonl(work_input, prepared)
    storage = _storage(run, work_input)

    if run.params["drop_tests"]:
        # Test-only samples teach nothing about writing code, so they are
        # dropped through the length filter's minimum line count instead of a
        # bespoke heuristic.
        prepared = [
            row
            for row in prepared
            if "def test_" not in row["code"] and "assert " not in row["code"]
        ]
        _write_jsonl(work_input, prepared)
        storage = _storage(run, work_input)

    CodeLengthSampleFilter(min_score=float(run.params["min_lines"])).run(
        storage=storage.step(), input_key="code", output_key="length_filter_label"
    )

    dataframe = storage.step().read("dataframe")
    needs_instruction = any(
        not str(row.get("instruction") or "").strip() for _, row in dataframe.iterrows()
    )
    if needs_instruction:
        CodeCodeToInstructionGenerator(llm_serving=serving).run(
            storage=storage.step(), input_key="code", output_key="generated_instruction"
        )

    CodeQualityScoreFilter(llm_serving=serving, min_score=7, max_score=10).run(
        storage=storage.step(),
        input_instruction_key="instruction",
        input_code_key="code",
        output_score_key="quality_score",
        output_feedback_key="quality_feedback",
        output_key="quality_score_filter_label",
    )

    dataframe = storage.step().read("dataframe")
    output: list[dict[str, Any]] = []
    for _, row in dataframe.iterrows():
        code = str(row.get("code") or "").strip()
        instruction = (
            str(row.get("instruction") or "").strip()
            or str(row.get("generated_instruction") or "").strip()
        )
        if code and instruction:
            output.append({"instruction": instruction, "output": code})
    return output


def _run_reasoning_sft(run: _Run, serving: Any) -> list[dict[str, Any]]:
    from dataflow.operators.reasoning import (
        ReasoningAnswerGenerator,
        ReasoningAnswerPipelineRootFilter,
    )

    source = _read_jsonl(run.input_path)
    prepared: list[dict[str, Any]] = []
    for row in source:
        instruction = _column(row, _INSTRUCTION_KEYS)
        if not instruction:
            continue
        entry = {"instruction": instruction}
        golden = row.get("golden_answer") or row.get("answer")
        if isinstance(golden, str) and golden.strip():
            entry["golden_answer"] = golden.strip()
        entry["output"] = _column(row, _ANSWER_KEYS)
        prepared.append(entry)
    if not prepared:
        raise PrepareError("no usable instruction column was found in this dataset")
    run.log(f"kept {len(prepared)} reasoning samples")

    work_input = run.run_dir / "reasoning_input.jsonl"
    _write_jsonl(work_input, prepared)
    storage = _storage(run, work_input)

    ReasoningAnswerGenerator(llm_serving=serving).run(
        storage=storage.step(), input_key="instruction", output_key="generated_cot"
    )

    dataframe = storage.step().read("dataframe")

    if run.params["verify_answers"] and "golden_answer" in dataframe.columns:
        # Branch filtering keeps only the generated solutions whose final answer
        # agrees with the reference answer.
        verify_storage = _storage(run, run.run_dir / "reasoning_verify.jsonl")
        verify_rows = []
        for _, row in dataframe.iterrows():
            verify_rows.append(
                {
                    "instruction": str(row.get("instruction") or ""),
                    "output": str(row.get("generated_cot") or ""),
                    "golden_answer": str(row.get("golden_answer") or ""),
                }
            )
        _write_jsonl(run.run_dir / "reasoning_verify.jsonl", verify_rows)
        ReasoningAnswerPipelineRootFilter().run(
            storage=verify_storage.step(),
            input_answer_key="output",
            input_gt_key="golden_answer",
            output_branch_key="answer_branch",
        )
        dataframe = verify_storage.step().read("dataframe")

    output: list[dict[str, Any]] = []
    for _, row in dataframe.iterrows():
        instruction = str(row.get("instruction") or "").strip()
        answer = (
            str(row.get("generated_cot") or "").strip()
            or str(row.get("output") or "").strip()
        )
        if instruction and answer:
            output.append({"instruction": instruction, "output": answer})
    return output


def _demo_prepare(run: _Run) -> list[dict[str, Any]]:
    """Synthesise an SFT dataset without DataFlow or any LLM call.

    Demo mode keeps the console flow intact (progress, live log, produced
    dataset) on a machine without a GPU, deriving rows from the uploaded file.
    """
    run.log("demo mode: generating synthetic rows (no DataFlow, no model calls)")
    suffix = run.input_path.suffix.lower()
    source: list[dict[str, Any]] = []
    if suffix == ".jsonl":
        source = _read_jsonl(run.input_path)
    elif suffix in (".txt", ".pdf"):
        chunk_chars = int(run.params.get("chunk_size") or 512) * 4
        source = _document_rows(run.input_path, chunk_chars)
    if not source:
        source = [{"text": str(run.record.get("dataset_name") or "demo dataset")}]

    limit = int(run.params.get("max_samples") or 0) or 24
    total = max(12, min(limit, 24))
    rows: list[dict[str, Any]] = []
    for index in range(total):
        seed = source[index % len(source)]
        instruction = (
            _column(seed, _INSTRUCTION_KEYS)
            or _column(seed, _TEXT_KEYS)
            or f"Sample question {index + 1}"
        )
        answer = (
            _column(seed, _ANSWER_KEYS)
            or _column(seed, _CODE_KEYS)
            or f"Sample answer {index + 1}"
        )
        rows.append({"instruction": instruction[:600], "output": answer[:2000]})
        if index % 4 == 0 or index == total - 1:
            run.report(_PROGRESS_PIPELINE + int(28 * (index + 1) / total))
            run.log(f"demo: generated {index + 1}/{total} rows")
            time.sleep(0.08)
    return rows


_RUNNERS = {
    "kb_pdf_to_qa": _run_kb_pdf_to_qa,
    "text_clean_sft": _run_text_clean_sft,
    "code_sft": _run_code_sft,
    "reasoning_sft": _run_reasoning_sft,
}


def run_child(run_id: str) -> int:
    """Entry point of the spawned process: execute one prepare, then exit."""
    record = store.load(PREPARE_KIND, run_id)
    if record is None:
        print(f"[prepare] unknown run {run_id}", flush=True)
        return 2
    run = _Run(record)
    try:
        run.report(_PROGRESS_STARTED)
        template = get_template(run.template)
        run.log(f"template={template.id} model={run.llm_model}")
        run.report(_PROGRESS_INPUT)
        run.report(_PROGRESS_PIPELINE)
        if settings.demo_mode:
            rows = _demo_prepare(run)
        else:
            rows = _RUNNERS[run.template](run, _llm_serving(run))
        rows = _truncate(rows, int(run.params.get("max_samples") or 0))
        if not rows:
            raise PrepareError("the pipeline produced no rows")
        run.report(_PROGRESS_WRITING)
        _write_jsonl(run.output_path, rows)
        dataset = datasets.register_prepared(
            source_path=run.output_path,
            name=f"{run.record['dataset_name']} ({template.name})",
            template=template.id,
        )
        run.report(
            _PROGRESS_DONE,
            status="succeeded",
            finished_at=store.now_ms(),
            output={
                "dataset_id": dataset["id"],
                "dataset_name": dataset["name"],
                "rows": dataset["rows"],
                "size_bytes": dataset["size_bytes"],
            },
        )
        run.log(f"done: {len(rows)} rows -> dataset {dataset['id']}")
        return 0
    except BaseException as exc:  # noqa: BLE001 - the child must always record a status
        message = str(exc).strip() or exc.__class__.__name__
        print(f"[prepare] failed: {message}", flush=True)
        store.update(
            PREPARE_KIND,
            run_id,
            status="failed",
            error=message[:2000],
            finished_at=store.now_ms(),
        )
        return 1


def main() -> int:
    parser = argparse.ArgumentParser(description="Run one DataFlow prepare job")
    parser.add_argument("--run-id", required=True)
    args = parser.parse_args()
    return run_child(args.run_id)


if __name__ == "__main__":
    sys.exit(main())
