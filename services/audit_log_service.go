package services

import (
	"log"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/google/uuid"
)

func RecordAuditLog(actorID uuid.UUID, action, targetType string, targetID *uuid.UUID, details string) {
	entry := models.AuditLog{
		ActorID:    actorID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    details,
	}
	if err := database.DB.Create(&entry).Error; err != nil {
		log.Printf("Failed to record audit log: %v", err)
	}
}
