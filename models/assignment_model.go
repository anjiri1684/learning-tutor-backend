package models

import (
	"time"

	"github.com/google/uuid"
)

type Assignment struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	BookingID    uuid.UUID  `gorm:"not null;index" json:"booking_id"`
	TeacherID    uuid.UUID  `gorm:"not null" json:"teacher_id"`
	StudentID    uuid.UUID  `gorm:"not null" json:"student_id"`
	Title        string     `gorm:"size:255;not null" json:"title"`
	Instructions string     `gorm:"type:text" json:"instructions"`
	DueDate      *time.Time `json:"due_date"`
	CreatedAt    time.Time  `json:"created_at"`

	Submission *AssignmentSubmission `gorm:"foreignkey:AssignmentID" json:"submission,omitempty"`
}

type AssignmentSubmission struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AssignmentID   uuid.UUID  `gorm:"not null;uniqueIndex" json:"assignment_id"`
	SubmissionText *string    `gorm:"type:text" json:"submission_text"`
	SubmissionLink *string    `gorm:"size:512" json:"submission_link"`
	FileObjectKey  *string    `gorm:"size:512" json:"-"`
	FileName       *string    `gorm:"size:255" json:"file_name"`
	SubmittedAt    time.Time  `json:"submitted_at"`
	Grade          *float64   `json:"grade"`
	Feedback       *string    `gorm:"type:text" json:"feedback"`
	GradedAt       *time.Time `json:"graded_at"`
}
