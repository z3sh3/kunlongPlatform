"""Curated DataFlow pipeline templates and their parameter schemas.

Only a handful of end-to-end pipelines are exposed. Each template is a complete,
runnable DataFlow recipe rather than a single operator, because the platform
renders one small form per template instead of a full canvas editor.

The platform treats `id`, parameter `key`, `type` and `options` as stable API:
it renders forms from this schema and maps known ids to its own translations,
falling back to the English `name`/`label` returned here.
"""

from __future__ import annotations

from dataclasses import asdict, dataclass, field
from typing import Any

from app.settings import settings

PARAM_STRING = "string"
PARAM_TEXT = "text"
PARAM_INT = "int"
PARAM_FLOAT = "float"
PARAM_BOOL = "bool"
PARAM_ENUM = "enum"


class InvalidPipelineParams(ValueError):
    """Raised when a prepare request carries unusable pipeline parameters."""


@dataclass(frozen=True)
class ParamSpec:
    key: str
    label: str
    type: str
    required: bool = False
    default: Any = None
    minimum: float | None = None
    maximum: float | None = None
    options: list[str] = field(default_factory=list)
    unit: str = ""
    help: str = ""

    def to_dict(self) -> dict[str, Any]:
        data = asdict(self)
        # Drop empty optional fields so the platform can treat a missing key as
        # "no constraint" without special-casing every property.
        return {
            key: value for key, value in data.items() if value not in ("", [], None)
        }


@dataclass(frozen=True)
class PipelineTemplate:
    id: str
    name: str
    description: str
    input_formats: list[str]
    params: list[ParamSpec]

    def to_dict(self) -> dict[str, Any]:
        return {
            "id": self.id,
            "name": self.name,
            "description": self.description,
            "input_formats": self.input_formats,
            "params": [spec.to_dict() for spec in self.params],
        }


_LANGUAGE = ParamSpec(
    key="language",
    label="Language",
    type=PARAM_ENUM,
    default="zh",
    options=["zh", "en"],
    help="Language of the generated question-answer pairs.",
)

_MAX_SAMPLES = ParamSpec(
    key="max_samples",
    label="Maximum output samples",
    type=PARAM_INT,
    default=1000,
    minimum=1,
    maximum=200000,
    unit="rows",
)

PIPELINE_TEMPLATES: tuple[PipelineTemplate, ...] = (
    PipelineTemplate(
        id="kb_pdf_to_qa",
        name="Documents to Q&A",
        description=(
            "Split PDFs and text documents into chunks, then generate question-answer "
            "pairs per chunk. Output is SFT-ready JSONL."
        ),
        input_formats=[".pdf", ".txt", ".docx", ".jsonl"],
        params=[
            _LANGUAGE,
            ParamSpec(
                key="chunk_size",
                label="Chunk size",
                type=PARAM_INT,
                default=512,
                minimum=128,
                maximum=4096,
                unit="tokens",
            ),
            ParamSpec(
                key="questions_per_chunk",
                label="Questions per chunk",
                type=PARAM_INT,
                default=3,
                minimum=1,
                maximum=10,
            ),
            _MAX_SAMPLES,
        ],
    ),
    PipelineTemplate(
        id="text_clean_sft",
        name="Noisy text to clean SFT",
        description=(
            "Score noisy text or low-quality question-answer pairs, filter out the low "
            "scoring rows, then optionally rewrite the survivors into cleaner "
            "instruction-response pairs."
        ),
        input_formats=[".jsonl", ".json", ".csv"],
        params=[
            _LANGUAGE,
            ParamSpec(
                key="min_score",
                label="Minimum quality score",
                type=PARAM_FLOAT,
                default=4.0,
                minimum=0.0,
                maximum=10.0,
            ),
            ParamSpec(
                key="rewrite",
                label="Rewrite filtered rows",
                type=PARAM_BOOL,
                default=True,
                help="Rewrite the surviving rows with the cleaning model instead of keeping them verbatim.",
            ),
            _MAX_SAMPLES,
        ],
    ),
    PipelineTemplate(
        id="code_sft",
        name="Code corpus to SFT",
        description=(
            "Deduplicate and lint a code corpus or code question set, drop trivial and "
            "broken samples, and emit SFT-ready instruction-response pairs."
        ),
        input_formats=[".jsonl", ".json"],
        params=[
            ParamSpec(
                key="language",
                label="Programming language",
                type=PARAM_ENUM,
                default="python",
                options=[
                    "python",
                    "javascript",
                    "typescript",
                    "java",
                    "go",
                    "rust",
                    "cpp",
                    "any",
                ],
            ),
            ParamSpec(
                key="min_lines",
                label="Minimum solution length",
                type=PARAM_INT,
                default=3,
                minimum=1,
                maximum=500,
                unit="lines",
            ),
            ParamSpec(
                key="drop_tests",
                label="Drop test-only samples",
                type=PARAM_BOOL,
                default=True,
            ),
            _MAX_SAMPLES,
        ],
    ),
    PipelineTemplate(
        id="reasoning_sft",
        name="Reasoning augmentation",
        description=(
            "Expand math and reasoning samples with generated step-by-step solutions, "
            "then verify the final answer and keep only consistent rows."
        ),
        input_formats=[".jsonl", ".json"],
        params=[
            _LANGUAGE,
            ParamSpec(
                key="difficulty",
                label="Difficulty",
                type=PARAM_ENUM,
                default="medium",
                options=["easy", "medium", "hard"],
            ),
            ParamSpec(
                key="verify_answers",
                label="Verify answers",
                type=PARAM_BOOL,
                default=True,
            ),
            _MAX_SAMPLES,
        ],
    ),
)

_TEMPLATES_BY_ID = {template.id: template for template in PIPELINE_TEMPLATES}


def get_template(template_id: str) -> PipelineTemplate:
    template = _TEMPLATES_BY_ID.get(template_id or "")
    if template is None:
        raise InvalidPipelineParams(f"unknown pipeline template: {template_id!r}")
    return template


def _coerce(spec: ParamSpec, raw: Any) -> Any:
    if spec.type == PARAM_BOOL:
        if isinstance(raw, bool):
            return raw
        if isinstance(raw, str) and raw.lower() in ("true", "false"):
            return raw.lower() == "true"
        raise InvalidPipelineParams(f"{spec.key} must be a boolean")
    if spec.type == PARAM_INT:
        if isinstance(raw, bool):
            raise InvalidPipelineParams(f"{spec.key} must be an integer")
        try:
            value = int(raw)
        except (TypeError, ValueError) as exc:
            raise InvalidPipelineParams(f"{spec.key} must be an integer") from exc
    elif spec.type == PARAM_FLOAT:
        if isinstance(raw, bool):
            raise InvalidPipelineParams(f"{spec.key} must be a number")
        try:
            value = float(raw)
        except (TypeError, ValueError) as exc:
            raise InvalidPipelineParams(f"{spec.key} must be a number") from exc
    else:
        value = str(raw).strip()
        if spec.type == PARAM_ENUM and value not in spec.options:
            raise InvalidPipelineParams(
                f"{spec.key} must be one of {', '.join(spec.options)}"
            )
        if spec.type in (PARAM_STRING, PARAM_TEXT) and len(value) > 2000:
            raise InvalidPipelineParams(f"{spec.key} is too long")
        return value

    if spec.minimum is not None and value < spec.minimum:
        raise InvalidPipelineParams(f"{spec.key} must be >= {spec.minimum}")
    if spec.maximum is not None and value > spec.maximum:
        raise InvalidPipelineParams(f"{spec.key} must be <= {spec.maximum}")
    return value


def validate_params(
    template_id: str, raw_params: dict[str, Any] | None
) -> dict[str, Any]:
    """Return a fully populated, type-checked parameter dict for a template.

    Unknown keys are rejected rather than ignored so a stale platform build
    cannot silently drop a parameter the operator filled in.
    """
    template = get_template(template_id)
    supplied = dict(raw_params or {})
    specs = {spec.key: spec for spec in template.params}

    unknown = sorted(set(supplied) - set(specs))
    if unknown:
        raise InvalidPipelineParams(f"unknown parameters: {', '.join(unknown)}")

    resolved: dict[str, Any] = {}
    for spec in template.params:
        if spec.key in supplied and supplied[spec.key] not in (None, ""):
            resolved[spec.key] = _coerce(spec, supplied[spec.key])
            continue
        if spec.required:
            raise InvalidPipelineParams(f"{spec.key} is required")
        resolved[spec.key] = spec.default
    return resolved


HYPER_PARAMS: tuple[ParamSpec, ...] = (
    ParamSpec(
        key="lora_rank",
        label="LoRA rank",
        type=PARAM_INT,
        default=8,
        minimum=1,
        maximum=256,
    ),
    ParamSpec(
        key="cutoff_len",
        label="Cutoff length",
        type=PARAM_INT,
        default=2048,
        minimum=128,
        maximum=32768,
        unit="tokens",
    ),
    ParamSpec(
        key="num_train_epochs",
        label="Epochs",
        type=PARAM_FLOAT,
        default=3.0,
        minimum=0.1,
        maximum=20.0,
    ),
    ParamSpec(
        key="learning_rate",
        label="Learning rate",
        type=PARAM_FLOAT,
        default=0.0001,
        minimum=1e-7,
        maximum=1.0,
    ),
    ParamSpec(
        key="per_device_train_batch_size",
        label="Batch size per device",
        type=PARAM_INT,
        default=1,
        minimum=1,
        maximum=128,
    ),
    ParamSpec(
        key="gradient_accumulation_steps",
        label="Gradient accumulation steps",
        type=PARAM_INT,
        default=8,
        minimum=1,
        maximum=256,
    ),
    ParamSpec(
        key="lr_scheduler_type",
        label="Learning rate scheduler",
        type=PARAM_ENUM,
        default="cosine",
        options=["cosine", "linear", "constant", "constant_with_warmup"],
    ),
    ParamSpec(key="bf16", label="BF16", type=PARAM_BOOL, default=True),
)

_HYPER_PARAM_SPECS = {spec.key: spec for spec in HYPER_PARAMS}


def validate_hyper_params(raw: dict[str, Any] | None) -> dict[str, Any]:
    """Return type-checked training hyper-parameters with defaults filled in."""
    supplied = dict(raw or {})
    unknown = sorted(set(supplied) - set(_HYPER_PARAM_SPECS))
    if unknown:
        raise InvalidPipelineParams(f"unknown hyper-parameters: {', '.join(unknown)}")

    resolved: dict[str, Any] = {}
    for key, spec in _HYPER_PARAM_SPECS.items():
        if key in supplied and supplied[key] not in (None, ""):
            resolved[key] = _coerce(spec, supplied[key])
        else:
            resolved[key] = spec.default
    return resolved


def capabilities() -> dict[str, Any]:
    """Describe everything the platform needs to build its job and dataset forms."""
    return {
        "pipelines": [template.to_dict() for template in PIPELINE_TEMPLATES],
        "methods": ["lora", "sft", "full"],
        "stages": ["sft"],
        "hyper_params": [spec.to_dict() for spec in HYPER_PARAMS],
        "base_model_required": True,
        "dataset_required": True,
        # Demo mode simulates preparation and training, so the console must not
        # require a cleaning model or an LLM gateway address for a run.
        "demo_mode": settings.demo_mode,
    }
