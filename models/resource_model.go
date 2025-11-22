package models

import (
	"time"
	"github.com/google/uuid"
)

type Resource struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	BookingID  uuid.UUID `gorm:"not null" json:"booking_id"`
	FileName   string    `gorm:"size:255;not null" json:"file_name"`
	FileURL    string    `gorm:"type:text;not null" json:"file_url"`
	UploadedAt time.Time `json:"uploaded_at"`

	Booking Booking `gorm:"foreignkey:BookingID"`
}