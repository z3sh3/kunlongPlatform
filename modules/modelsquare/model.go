package modelsquare

import (
	"time"

	"gorm.io/gorm"
)

// Finetune job lifecycle statuses. Only draft jobs are editable; the
// submitted→queued→running→succeeded/failed transitions are driven by an
// administrator (training scheduler integration is a follow-up).
const (
	JobStatusDraft     = 0
	JobStatusSubmitted = 1
	JobStatusQueued    = 2
	JobStatusRunning   = 3
	JobStatusSucceeded = 4
	JobStatusFailed    = 5
	JobStatusCanceled  = 6
)

// FinetuneJob is a user-created post-training job over a base model.
// DatasetRefs and HyperParams are JSON strings kept opaque at this layer.
type FinetuneJob struct {
	Id          int            `json:"id"`
	UserId      int            `json:"user_id" gorm:"index"`
	Name        string         `json:"name" gorm:"index;size:255"`
	BaseModel   string         `json:"base_model" gorm:"size:255"`
	Method      string         `json:"method" gorm:"size:32"`
	DatasetRefs string         `json:"dataset_refs" gorm:"type:text"`
	HyperParams string         `json:"hyper_params" gorm:"type:text"`
	Status      int            `json:"status" gorm:"index"`
	ResultModel string         `json:"result_model" gorm:"size:255"`
	ErrorMsg    string         `json:"error_msg" gorm:"type:text"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
