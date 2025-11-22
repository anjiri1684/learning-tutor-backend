package models

import "github.com/google/uuid"


type Language struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"ID"`
	Name            string    `gorm:"size:100;not null;unique" json:"name"` 
	PricePerSession float64   `gorm:"type:numeric(10,2);not null;default:0.00" json:"PricePerSession"`
	Currency        string    `gorm:"size:3;not null;default:'USD'"`
}