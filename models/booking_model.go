package models

import (
	"time"
	"github.com/google/uuid"
)

type Booking struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	StudentID        uuid.UUID `gorm:"not null"`
	TeacherID        uuid.UUID `gorm:"not null"`
	AvailabilitySlotID uuid.UUID `gorm:"not null"`
	Status           string    `gorm:"size:20;not null;default:'pending_payment'"`
	Price            float64   `gorm:"type:numeric(10,2);not null"`
	Currency 			string    `gorm:"size:3"`
	MeetingLink      *string   `gorm:"size:255"`

	TeacherFeedback  *string   `gorm:"type:text"`
	StudentFeedback  *string   `gorm:"type:text"`

	StudentRating    *int      `gorm:"check:student_rating >= 1 AND student_rating <= 5" json:"student_rating"`
	StudentComment   *string   `gorm:"type:text" json:"student_comment"`
	
	ProposedStartTime *time.Time
	ProposedEndTime   *time.Time

	Student          User             `gorm:"foreignkey:StudentID"`
	Teacher          User             `gorm:"foreignkey:TeacherID"`
	AvailabilitySlot AvailabilitySlot `gorm:"foreignkey:AvailabilitySlotID"`
	
	CreatedAt        time.Time
	UpdatedAt        time.Time
}