package models

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID  `gorm:"not null;index" json:"user_id"`
	Type      string     `gorm:"size:50;not null" json:"type"`
	Title     string     `gorm:"size:255;not null" json:"title"`
	Body      string     `gorm:"type:text" json:"body"`
	Link      string     `gorm:"size:255" json:"link"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}
