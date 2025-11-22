package models

import (
	"time"
	"github.com/google/uuid"
)

type Payment struct {
	ID                 uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	BookingID          *uuid.UUID `gorm:"unique"` 
	StudentBundleID    *uuid.UUID `gorm:"unique"` 
	ProviderOrderID    *string   `gorm:"size:255;unique"` 
	MerchantRequestID  *string   `gorm:"size:255;unique"` 
	Amount             float64   `gorm:"type:numeric(10,2);not null"`
	Currency string    `gorm:"size:3"`
	Provider           string    `gorm:"size:50;not null"`
	ProviderTxnID      *string   `gorm:"size:255;unique"`
	Status        string    `gorm:"size:20;not null"`
	RefundStatus *string `gorm:"size:20"` 
	RefundReason *string `gorm:"type:text"`

	Booking   Booking   `gorm:"foreignkey:BookingID"`
	StudentBundle StudentBundle `gorm:"foreignkey:StudentBundleID"` 

	CreatedAt time.Time
	UpdatedAt time.Time
}