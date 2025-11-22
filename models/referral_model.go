package models

import (
	"time"
	"github.com/google/uuid"
)

type Referral struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ReferrerID     uuid.UUID `gorm:"not null"` 
	ReferredUserID uuid.UUID `gorm:"not null;unique"` 
	Status         string    `gorm:"size:20;not null;default:'pending'"` 
	RewardAmount   float64   `gorm:"type:numeric(10,2);default:0.00"` 

	Referrer     User `gorm:"foreignkey:ReferrerID"`
	ReferredUser User `gorm:"foreignkey:ReferredUserID"`

	CreatedAt time.Time
}