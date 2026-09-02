package models

import (
	"time"

	"github.com/google/uuid"
)

// ContactRequest is a message submitted from a public "contact us" / "apply to
// teach" form. Admins review these from the dashboard instead of email.
type ContactRequest struct {
	ID      uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"ID"`
	Name    string    `gorm:"size:255;not null" json:"name"`
	Email   string    `gorm:"size:255;not null" json:"email"`
	Subject string    `gorm:"size:255" json:"subject"`
	Message string    `gorm:"type:text;not null" json:"message"`
	Type    string    `gorm:"size:30;not null;default:'general'" json:"type"` // general, teacher_application, other
	Status  string    `gorm:"size:20;not null;default:'new'" json:"status"`   // new, read, resolved

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
