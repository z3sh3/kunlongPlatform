package modelsquare

import (
	"time"

	"gorm.io/gorm"
)

// Finetune job lifecycle statuses. Draft jobs are editable; submitted jobs are
// dispatched to the training service, which drives queued→running→succeeded/
// failed. Admins can still force a status through the console.
const (
	JobStatusDraft     = 0
	JobStatusSubmitted = 1
	JobStatusQueued    = 2
	JobStatusRunning   = 3
	JobStatusSucceeded = 4
	JobStatusFailed    = 5
	JobStatusCanceled  = 6
)

// Dataset intake and preparation statuses.
const (
	DatasetStatusPending = 0
	DatasetStatusReady   = 1
	DatasetStatusFailed  = 2
)

// Dataset record methods.
const (
	DatasetSourceUpload   = "upload"
	DatasetSourceURL      = "url"
	DatasetSourceMarket   = "market"
	DatasetSourcePipeline = "pipeline"
)

// Trained model artifact statuses.
const (
	ArtifactStatusAvailable = 0
	ArtifactStatusMissing   = 1
)

// FinetuneJob is a post-training job over a base model.
//
// The job row is the platform's source of truth for ownership and lifecycle;
// ServiceJobId links it to the execution state held by the training service.
// ServiceTokenId tracks the short-lived API token minted for the DataFlow data
// cleaning calls so it can be revoked when the job stops.
type FinetuneJob struct {
	Id               int            `json:"id"`
	UserId           int            `json:"user_id" gorm:"index"`
	Name             string         `json:"name" gorm:"index;size:255"`
	BaseModel        string         `json:"base_model" gorm:"size:255"`
	Method           string         `json:"method" gorm:"size:32"`
	DatasetRefs      string         `json:"dataset_refs" gorm:"type:text"`
	HyperParams      string         `json:"hyper_params" gorm:"type:text"`
	Status           int            `json:"status" gorm:"index"`
	ResultModel      string         `json:"result_model" gorm:"size:255"`
	ErrorMsg         string         `json:"error_msg" gorm:"type:text"`
	DataflowTemplate string         `json:"dataflow_template" gorm:"size:64"`
	DataflowParams   string         `json:"dataflow_params" gorm:"type:text"`
	DatasetId        int            `json:"dataset_id" gorm:"index"`
	ServiceJobId     string         `json:"service_job_id" gorm:"size:64;index"`
	ServiceTokenId   int            `json:"-" gorm:"index"`
	Progress         int            `json:"progress"`
	OutputDir        string         `json:"output_dir" gorm:"size:512"`
	Usage            string         `json:"usage" gorm:"type:text"`
	StartedAt        int64          `json:"started_at" gorm:"bigint"`
	FinishedAt       int64          `json:"finished_at" gorm:"bigint"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

// TrainingDataset is a dataset the training service holds for one user.
//
// The file itself lives on the training host; this row records who may use it,
// how it was obtained and whether a DataFlow pipeline has turned it into
// SFT-ready rows (PreparedRef points at the produced dataset).
type TrainingDataset struct {
	Id             int            `json:"id"`
	UserId         int            `json:"user_id" gorm:"index"`
	Name           string         `json:"name" gorm:"index;size:255"`
	Source         string         `json:"source" gorm:"size:32"`
	SourceRef      string         `json:"source_ref" gorm:"type:text"`
	Format         string         `json:"format" gorm:"size:32"`
	SizeBytes      int64          `json:"size_bytes" gorm:"bigint"`
	Rows           int64          `json:"rows" gorm:"bigint"`
	Status         int            `json:"status" gorm:"index"`
	ErrorMsg       string         `json:"error_msg" gorm:"type:text"`
	RemoteId       string         `json:"remote_id" gorm:"size:64;index"`
	Prepared       bool           `json:"prepared"`
	PreparedRef    string         `json:"prepared_ref" gorm:"size:64"`
	Pipeline       string         `json:"pipeline" gorm:"size:64"`
	PipelineParams string         `json:"pipeline_params" gorm:"type:text"`
	PrepareRunId   string         `json:"prepare_run_id" gorm:"size:64;index"`
	PrepareStatus  int            `json:"prepare_status"`
	PrepareError   string         `json:"prepare_error" gorm:"type:text"`
	ServiceTokenId int            `json:"-" gorm:"index"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

// TrainedModel is a training artifact the owner can reuse or an administrator
// can promote into the platform model catalog. ChannelId records the inference
// channel created for it, if any.
type TrainedModel struct {
	Id             int            `json:"id"`
	UserId         int            `json:"user_id" gorm:"index"`
	JobId          int            `json:"job_id" gorm:"index"`
	Name           string         `json:"name" gorm:"index;size:255"`
	BaseModel      string         `json:"base_model" gorm:"size:255"`
	Method         string         `json:"method" gorm:"size:32"`
	ArtifactKind   string         `json:"artifact_kind" gorm:"size:32"`
	ArtifactRef    string         `json:"artifact_ref" gorm:"size:512"`
	SizeBytes      int64          `json:"size_bytes" gorm:"bigint"`
	Status         int            `json:"status" gorm:"index"`
	ChannelId      int            `json:"channel_id"`
	ModelCatalogId int            `json:"model_catalog_id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}
