package models

import (
	"time"

	"github.com/google/uuid"
)

type Waitlist struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AvailabilitySlotID uuid.UUID  `gorm:"not null;index" json:"availability_slot_id"`
	StudentID          uuid.UUID  `gorm:"not null" json:"student_id"`
	NotifiedAt         *time.Time `json:"notified_at"`
	CreatedAt          time.Time  `json:"created_at"`

	AvailabilitySlot AvailabilitySlot `gorm:"foreignkey:AvailabilitySlotID" json:"availability_slot,omitempty"`
	Student          User             `gorm:"foreignkey:StudentID" json:"student,omitempty"`
}
