package models

import (
	"time"
	"github.com/google/uuid"
)

type Certificate struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	StudentID      uuid.UUID `gorm:"not null"`
	TeacherID      uuid.UUID `gorm:"not null"`
	LanguageID     uuid.UUID `gorm:"not null"`
	CourseTitle    string    `gorm:"size:255;not null"`
	CompletionDate time.Time `gorm:"not null"`
	CertificateURL string    `gorm:"type:text;not null"` 

	Student  User     `gorm:"foreignkey:StudentID"`
	Teacher  User     `gorm:"foreignkey:TeacherID"`
	Language Language `gorm:"foreignkey:LanguageID"`
}