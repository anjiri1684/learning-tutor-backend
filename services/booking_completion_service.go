package services

import (
	"log"
	"strconv"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CompleteBooking marks a confirmed booking as completed, credits the teacher's
// earnings, and triggers the same rewards/certificate/notification side-effects
// as an explicit teacher "mark complete" action. Used both by the manual
// endpoint and by the auto-complete cron job once a class's end time has passed.
func CompleteBooking(bookingID uuid.UUID) error {
	var booking models.Booking
	if err := database.DB.
		Preload("AvailabilitySlot.Language").
		Preload("Student").
		Preload("Teacher").
		First(&booking, "id = ?", bookingID).Error; err != nil {
		return err
	}

	if booking.Status != "confirmed" {
		return nil
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		booking.Status = "completed"
		if err := tx.Save(&booking).Error; err != nil {
			return err
		}

		commissionRate, _ := strconv.ParseFloat(config.Config("PLATFORM_COMMISSION_RATE"), 64)
		earnings := booking.Price * (1 - commissionRate)

		if err := tx.Model(&models.Teacher{}).Where("user_id = ?", booking.TeacherID).
			Update("current_balance", gorm.Expr("current_balance + ?", earnings)).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	go AwardRewardsForClassCompletion(booking.StudentID)
	go CheckAndGenerateCertificate(booking)
	go CreateNotification(booking.StudentID, "class_completed", "Class completed",
		"Your class with "+booking.Teacher.FullName+" is complete. Leave a review!", "/dashboard/my-classes")

	return nil
}

// AutoCompleteEndedBookings finds bookings still "confirmed" whose class ended
// a while ago and completes them automatically, so students aren't blocked from
// reviewing (and teachers aren't blocked from earning) just because nobody
// clicked "Mark Complete" in time.
func AutoCompleteEndedBookings(bookingIDs []uuid.UUID) {
	for _, id := range bookingIDs {
		if err := CompleteBooking(id); err != nil {
			log.Printf("Failed to auto-complete booking %s: %v", id, err)
		}
	}
}
