package models

import "github.com/google/uuid"

type Question struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	QuestionText   string    `gorm:"type:text;not null"`
	QuestionType   string    `gorm:"size:50;not null;default:'multiple_choice'"` 
	Options        string    `gorm:"type:text"` 
	CorrectAnswer  string    `gorm:"type:text;not null"`
}