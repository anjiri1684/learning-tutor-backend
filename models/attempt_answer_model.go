package models

import "github.com/google/uuid"

type AttemptAnswer struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TestAttemptID uuid.UUID `gorm:"not null"`
	QuestionID    uuid.UUID `gorm:"not null"`
	SelectedAnswer string    `gorm:"type:text;not null"`
	IsCorrect     bool      `gorm:"not null"`

	TestAttempt TestAttempt `gorm:"foreignkey:TestAttemptID"`
	Question    Question    `gorm:"foreignkey:QuestionID"`
}

