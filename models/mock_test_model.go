package models

import (
	"time"
	"github.com/google/uuid"
)

type MockTest struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Title            string    `gorm:"size:255;not null"`
	Description      string    `gorm:"type:text"`
	DurationMinutes  int       `gorm:"not null"`
	
	Questions        []*Question `gorm:"many2many:mock_test_questions;"`

	CreatedAt        time.Time
	UpdatedAt        time.Time
}