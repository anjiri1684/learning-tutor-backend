package models

import (
	"time"
	"github.com/google/uuid"
)

type TestAttempt struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	StudentID uuid.UUID `gorm:"not null"`
	MockTestID uuid.UUID `gorm:"not null"`
	StartTime time.Time `gorm:"not null"`
	EndTime   *time.Time 
	Score     *float64   

	Student   User     `gorm:"foreignkey:StudentID"`
	MockTest  MockTest `gorm:"foreignkey:MockTestID"`
}