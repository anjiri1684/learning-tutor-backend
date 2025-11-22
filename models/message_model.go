package models

import (
	"time"
	"github.com/google/uuid"
)

type Message struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ConversationID uuid.UUID `gorm:"not null"`
	SenderID       uuid.UUID `gorm:"not null"`
	Content        string    `gorm:"type:text;not null"`
	ReadAt         *time.Time

	Sender       User `gorm:"foreignkey:SenderID"`
	Conversation Conversation `gorm:"foreignkey:ConversationID"`
	
	CreatedAt    time.Time
}