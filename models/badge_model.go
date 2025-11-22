package models

import (
	"time"
	"github.com/google/uuid"
)

type Badge struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string    `gorm:"size:255;not null;unique"`
	Description string    `gorm:"type:text;not null"`
	IconURL     string    `gorm:"size:255;not null"`
	CreatedAt   time.Time
}