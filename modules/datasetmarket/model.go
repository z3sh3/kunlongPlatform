package datasetmarket

import (
	"time"

	"gorm.io/gorm"
)

// Dataset is a publishable dataset listing in the data marketplace.
type Dataset struct {
	Id          int            `json:"id"`
	UserId      int            `json:"user_id" gorm:"index"`
	Name        string         `json:"name" gorm:"index;size:255"`
	Description string         `json:"description" gorm:"type:text"`
	Category    string         `json:"category" gorm:"size:64;index"`
	Format      string         `json:"format" gorm:"size:64"`
	Tags        string         `json:"tags" gorm:"type:text"`
	FileUrl     string         `json:"file_url" gorm:"type:text"`
	CoverUrl    string         `json:"cover_url" gorm:"type:text"`
	SizeBytes   int64          `json:"size_bytes" gorm:"bigint"`
	PriceMode   int            `json:"price_mode"`
	UnitPrice   int64          `json:"unit_price" gorm:"bigint"`
	Status      int            `json:"status" gorm:"index"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// DatasetGrant records a user's authorized (or pending) access to a
// dataset. Free datasets are approved immediately on apply; paid ones
// stay pending until an administrator approves them. Actual billing
// settlement is a follow-up and is not implemented yet.
type DatasetGrant struct {
	Id        int            `json:"id"`
	DatasetId int            `json:"dataset_id" gorm:"index"`
	UserId    int            `json:"user_id" gorm:"index"`
	Status    int            `json:"status" gorm:"index"`
	ExpiresAt int64          `json:"expires_at" gorm:"bigint"`
	Remark    string         `json:"remark" gorm:"type:text"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Dataset Dataset `json:"dataset" gorm:"foreignKey:DatasetId"`
}
