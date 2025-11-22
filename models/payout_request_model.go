package models

import (
	"time"
	"github.com/google/uuid"
)

type PayoutRequest struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TeacherID    uuid.UUID `gorm:"not null"`
	Amount       float64   `gorm:"type:numeric(10,2);not null"`
	Status       string    `gorm:"size:20;not null;default:'pending'"` 
	AdminNotes   *string   `gorm:"type:text"`
	RequestedAt  time.Time `gorm:"not null"`
	ProcessedAt  *time.Time

	Teacher User `gorm:"foreignkey:TeacherID"`
}