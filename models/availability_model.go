package models

import (
	"time"
	"github.com/google/uuid"
)

type AvailabilitySlot struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TeacherID uuid.UUID `gorm:"not null" json:"-"`
	LanguageID *uuid.UUID `gorm:"" json:"language_id"`
	StartTime time.Time `gorm:"not null" json:"start_time"`
	EndTime   time.Time `gorm:"not null" json:"end_time"`
	Status    string    `gorm:"size:20;not null;default:'available'" json:"status"`

	MaxStudents     int `gorm:"not null;default:1" json:"max_students"`
	CurrentStudents int `gorm:"not null;default:0" json:"current_students"`

	Teacher   User      `gorm:"foreignkey:TeacherID" json:"teacher,omitempty"`
	Language  Language  `gorm:"foreignkey:LanguageID" json:"language,omitempty"`
}
