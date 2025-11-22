package models

import (
	"time"

	"github.com/google/uuid"
)


type StudentBundle struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"ID"`
	StudentID        uuid.UUID `gorm:"not null" json:"student_id"`
	BundleID         uuid.UUID `gorm:"not null" json:"bundle_id"`
	PurchaseDate     time.Time `gorm:"not null" json:"purchase_date"`
	RemainingClasses int       `gorm:"not null" json:"remaining_classes"`
	Status           string    `gorm:"size:20;not null;default:'pending_payment'" json:"status"`

	Student User   `gorm:"foreignkey:StudentID" json:"student"`
	Bundle  Bundle `gorm:"foreignkey:BundleID" json:"bundle"` 
	
}