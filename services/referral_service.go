package services

import (
	"log"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const ReferralRewardAmount = 5.00 

func CompleteReferralIfApplicable(studentID uuid.UUID) {
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var referral models.Referral
		if err := tx.Preload("Referrer").Where("referred_user_id = ? AND status = ?", studentID, "pending").First(&referral).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}

		referrer := referral.Referrer
		referrer.CreditBalance += ReferralRewardAmount
		if err := tx.Save(&referrer).Error; err != nil {
			return err
		}

		referral.Status = "completed"
		referral.RewardAmount = ReferralRewardAmount
		if err := tx.Save(&referral).Error; err != nil {
			return err
		}

		go notifications.SendEmail(
			referrer.FullName,
			referrer.Email,
			"You've Earned a Referral Credit!",
			"<h1>Congratulations!</h1><p>Someone you referred has made their first purchase. A credit of $5.00 has been added to your account.</p>",
		)

		return nil 
	})

	if err != nil {
		log.Printf("🔥 Error processing referral for student %s: %v", studentID, err)
	}
}