package agentworkshop

import (
	"time"

	"gorm.io/gorm"
)

// Agent listing lifecycle. Agents go through a review step before becoming
// publicly available: draft → reviewing → published → offline.
const (
	StatusDraft     = 0
	StatusReviewing = 1
	StatusPublished = 2
	StatusOffline   = 3
)

// Agent is a user-built custom agent. Config holds the low-code
// orchestration definition as an opaque JSON string; the visual editor is
// a follow-up.
type Agent struct {
	Id            int            `json:"id"`
	UserId        int            `json:"user_id" gorm:"index"`
	Name          string         `json:"name" gorm:"index;size:255"`
	Description   string         `json:"description" gorm:"type:text"`
	Icon          string         `json:"icon" gorm:"type:text"`
	Category      string         `json:"category" gorm:"size:64;index"`
	Config        string         `json:"config" gorm:"type:text"`
	PriceMode     int            `json:"price_mode"`
	UnitPrice     int64          `json:"unit_price" gorm:"bigint"`
	Status        int            `json:"status" gorm:"index"`
	ReviewComment string         `json:"review_comment" gorm:"type:text"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}
