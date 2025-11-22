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
	CreatedAt      time.Time   `json:"-"`
	UpdatedAt      time.Time   `json:"-"`
}