package models

import (
	"time"
	"github.com/google/uuid"
)

type User struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	FullName          string    `gorm:"size:255;not null" json:"full_name"`
	Email             string    `gorm:"size:255;not null;unique" json:"email"`
	Password          string    `gorm:"not null" json:"-"` 
	Role              string    `gorm:"size:20;not null;default:'student'" json:"role"`
	
	ReferralCode      *string   `gorm:"size:10;unique" json:"referral_code"`
	ReferredByCode    *string   `gorm:"size:10" json:"referred_by_code"`
	CreditBalance     float64   `gorm:"type:numeric(10,2);default:0.00" json:"credit_balance"`
	
	XP                int             `gorm:"default:0" json:"xp"`
	Badges            []*Badge        `gorm:"many2many:user_badges;" json:"badges,omitempty"`
	Conversations     []*Conversation `gorm:"many2many:conversation_participants;" json:"-"` 
	
	ProfilePictureURL *string `gorm:"size:255" json:"profile_picture_url"`
	TimeZone          *string `gorm:"size:100" json:"time_zone"`
	LearningGoals     *string `gorm:"type:text" json:"learning_goals"`
	ProficiencyLevel  *string `gorm:"size:50" json:"proficiency_level"`
	
	ResetPasswordToken        *string    `gorm:"size:255;unique" json:"-"`
	ResetPasswordTokenExpiresAt *time.Time `json:"-"`
	IsActive          bool   `gorm:"default:true"` // <-- Add this line
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}