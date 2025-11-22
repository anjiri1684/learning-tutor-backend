package models

import "github.com/google/uuid"

type TeacherLanguage struct {
	TeacherUserID uuid.UUID `gorm:"type:uuid;primaryKey"`
	LanguageID    uuid.UUID `gorm:"type:uuid;primaryKey"`

	Teacher Teacher  `gorm:"foreignKey:TeacherUserID"`
	Language Language `gorm:"foreignKey:LanguageID"`
}
