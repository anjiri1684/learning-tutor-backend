package services

import (
	"log"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	xpForClassCompletion = 10
	badgeNameFirstClass  = "First Class"
)

func AwardRewardsForClassCompletion(studentID uuid.UUID) {
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var student models.User
		if err := tx.Preload("Badges").First(&student, "id = ?", studentID).Error; err != nil {
			return err
		}

		student.XP += xpForClassCompletion
		if err := tx.Save(&student).Error; err != nil {
			return err
		}

		var completedClassesCount int64
		tx.Model(&models.Booking{}).Where("student_id = ? AND status = ?", studentID, "completed").Count(&completedClassesCount)

		if completedClassesCount == 1 {
			for _, badge := range student.Badges {
				if badge.Name == badgeNameFirstClass {
					return nil 
				}
			}

			var firstClassBadge models.Badge
			if err := tx.Where("name = ?", badgeNameFirstClass).First(&firstClassBadge).Error; err == nil {
				if err := tx.Model(&student).Association("Badges").Append(&firstClassBadge); err != nil {
					return err
				}
			} else {
				log.Printf("Warning: Badge '%s' not found in database. Cannot award.", badgeNameFirstClass)
			}
		}
		
		return nil
	})

	if err != nil {
		log.Printf("🔥 Failed to award rewards to student %s: %v", studentID, err)
	} else {
		log.Printf("✅ Awarded %d XP to student %s.", xpForClassCompletion, studentID)
	}
}