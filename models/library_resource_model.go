package models

import (
	"time"

	"github.com/google/uuid"
)

type LibraryResource struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UploadedBy  uuid.UUID `gorm:"not null" json:"uploaded_by"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	ObjectKey   string    `gorm:"size:512;not null" json:"-"`
	FileName    string    `gorm:"size:255;not null" json:"file_name"`
	CreatedAt   time.Time `json:"created_at"`

	Uploader User                    `gorm:"foreignkey:UploadedBy" json:"uploader"`
	Access   []LibraryResourceAccess `gorm:"foreignkey:LibraryResourceID" json:"access,omitempty"`
}

type LibraryResourceAccess struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	LibraryResourceID uuid.UUID  `gorm:"not null" json:"library_resource_id"`
	StudentEmail      string     `gorm:"size:255;not null" json:"student_email"`
	StudentID         *uuid.UUID `json:"student_id"`
	GrantedAt         time.Time  `json:"granted_at"`
	RevokedAt         *time.Time `json:"revoked_at"`
}
