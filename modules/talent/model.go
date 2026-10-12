package talent

import (
	"time"

	"gorm.io/gorm"
)

// Training course categories.
const (
	CategoryCertification = "certification"
	CategoryConsultation  = "consultation"
)

// Enrollment lifecycle on top of the shared review statuses: an approved
// enrollment can additionally be marked completed.
const (
	EnrollmentStatusCompleted = 4
)

// TrainingCourse is a certification-training or consultation course
// published by an administrator.
type TrainingCourse struct {
	Id          int            `json:"id"`
	Title       string         `json:"title" gorm:"index;size:255"`
	Category    string         `json:"category" gorm:"size:64;index"`
	Description string         `json:"description" gorm:"type:text"`
	Content     string         `json:"content" gorm:"type:text"`
	Schedule    string         `json:"schedule" gorm:"type:text"`
	Capacity    int            `json:"capacity"`
	PriceMode   int            `json:"price_mode"`
	Price       int64          `json:"price" gorm:"bigint"`
	Status      int            `json:"status" gorm:"index"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// TrainingEnrollment records a user's enrollment request for a course.
type TrainingEnrollment struct {
	Id        int            `json:"id"`
	CourseId  int            `json:"course_id" gorm:"index"`
	UserId    int            `json:"user_id" gorm:"index"`
	Status    int            `json:"status" gorm:"index"`
	Remark    string         `json:"remark" gorm:"type:text"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Course TrainingCourse `json:"course" gorm:"foreignKey:CourseId"`
}
