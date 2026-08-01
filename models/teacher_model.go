package models

import (
	"time"
	"github.com/google/uuid"
)

type Teacher struct {
	UserID         uuid.UUID   `gorm:"primary_key" json:"user_id"`
	Headline       *string     `gorm:"size:255" json:"headline"`
	Bio            *string     `gorm:"type:text" json:"bio"`
	Status         string      `gorm:"size:20;not null;default:'pending'" json:"status"`
	AvgRating      float32     `gorm:"default:0" json:"avg_rating"`
	CurrentBalance float64     `gorm:"type:numeric(10,2);default:0.00" json:"-"` 
	Languages      []*Language `gorm:"many2many:teacher_languages;" json:"languages"`
	User           User        `gorm:"foreignkey:UserID" json:"user"`
	PayoutMethod       *string `gorm:"size:20" json:"payout_method"`
	PayoutAccountName  *string `gorm:"size:255" json:"payout_account_name"`
	PayoutAccountNumber *string `gorm:"size:100" json:"payout_account_number"`
	PayoutBankName     *string `gorm:"size:255" json:"payout_bank_name"`
	VerificationDocURL *string `gorm:"size:512" json:"verification_doc_url"`
	DefaultMeetingLink *string `gorm:"size:512" json:"default_meeting_link"`
	CreatedAt      time.Time   `json:"-"`
	UpdatedAt      time.Time   `json:"-"`
}