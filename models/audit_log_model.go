package models

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ActorID    uuid.UUID  `gorm:"not null" json:"actor_id"`
	Action     string     `gorm:"size:100;not null" json:"action"`
	TargetType string     `gorm:"size:50" json:"target_type"`
	TargetID   *uuid.UUID `json:"target_id"`
	Details    string     `gorm:"type:text" json:"details"`
	CreatedAt  time.Time  `json:"created_at"`

	Actor User `gorm:"foreignkey:ActorID" json:"actor"`
}
