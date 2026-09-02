package models

import (
	"time"

	"github.com/google/uuid"
)

// CorporateEnquiry is a lead captured from the "Request a custom quote"
// form on the corporate training page / landing page.
type CorporateEnquiry struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"ID"`
	CompanyName string     `gorm:"size:255;not null" json:"company_name"`
	ContactName string     `gorm:"size:255;not null" json:"contact_name"`
	Email       string     `gorm:"size:255;not null" json:"email"`
	Phone       string     `gorm:"size:50" json:"phone"`
	TeamSize    int        `gorm:"not null;default:0" json:"team_size"`
	LanguageID  *uuid.UUID `json:"language_id"`
	Message     string     `gorm:"type:text" json:"message"`
	Status      string     `gorm:"size:20;not null;default:'new'" json:"status"` // new, contacted, closed

	Language *Language `gorm:"foreignkey:LanguageID" json:"language"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
