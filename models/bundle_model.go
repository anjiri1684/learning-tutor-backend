package models

import (
	"github.com/google/uuid"
	"time"
)

type Bundle struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name            string    `gorm:"size:255;not null"`
	LanguageID      uuid.UUID `gorm:"not null"`
	NumberOfClasses int       `gorm:"not null"`
	Price           float64   `gorm:"type:numeric(10,2);not null"`
	Currency        string    `gorm:"size:3;default:'USD'"`
	IsActive        bool      `gorm:"default:true"`
	Type            string    `gorm:"size:20;not null;default:'standard'" json:"type"`
	Description     string    `gorm:"type:text" json:"description"`

	Language  Language `gorm:"foreignkey:LanguageID" json:"language"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
