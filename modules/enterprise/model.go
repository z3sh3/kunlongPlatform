package enterprise

import (
	"time"

	"gorm.io/gorm"
)

// Incubation program types.
const (
	TypeIncubation = "incubation"
	TypeSMESupport = "sme_support"
	TypeGovPark    = "gov_park"
)

// IncubationProgram is a published offering such as enterprise incubation,
// SME support, or a government/park special program.
type IncubationProgram struct {
	Id           int            `json:"id"`
	Title        string         `json:"title" gorm:"index;size:255"`
	Type         string         `json:"type" gorm:"size:32;index"`
	Description  string         `json:"description" gorm:"type:text"`
	Requirements string         `json:"requirements" gorm:"type:text"`
	Status       int            `json:"status" gorm:"index"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// IncubationApplication is a company's application to a program,
// reviewed by an administrator.
type IncubationApplication struct {
	Id            int            `json:"id"`
	ProgramId     int            `json:"program_id" gorm:"index"`
	UserId        int            `json:"user_id" gorm:"index"`
	CompanyName   string         `json:"company_name" gorm:"size:255"`
	ContactName   string         `json:"contact_name" gorm:"size:128"`
	ContactPhone  string         `json:"contact_phone" gorm:"size:64"`
	Description   string         `json:"description" gorm:"type:text"`
	Status        int            `json:"status" gorm:"index"`
	ReviewComment string         `json:"review_comment" gorm:"type:text"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	Program IncubationProgram `json:"program" gorm:"foreignKey:ProgramId"`
}
