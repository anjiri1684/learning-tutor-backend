package models

import (
	"time"
	"github.com/google/uuid"
)

type Review struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	BookingID uuid.UUID `gorm:"not null;unique"` 
	StudentID uuid.UUID `gorm:"not null"`
	TeacherID uuid.UUID `gorm:"not null"`
	Rating    int       `gorm:"not null"` 
	Comment   string    `gorm:"type:text"`

	Booking   Booking   `gorm:"foreignkey:BookingID"`
	Student   User      `gorm:"foreignkey:StudentID"`
	Teacher   User      `gorm:"foreignkey:TeacherID"`
	
	CreatedAt time.Time
	UpdatedAt time.Time
}