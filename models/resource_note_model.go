package models

import (
	"time"

	"github.com/google/uuid"
)

type ResourceNote struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID            uuid.UUID `gorm:"not null;uniqueIndex:idx_user_resource_note" json:"user_id"`
	LibraryResourceID uuid.UUID `gorm:"not null;uniqueIndex:idx_user_resource_note" json:"library_resource_id"`
	Note              string    `gorm:"type:text;not null" json:"note"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
