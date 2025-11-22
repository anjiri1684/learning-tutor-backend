package jobs

import (
	"log"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
)

func CheckForUnattendedClasses() {
	log.Println("Running job: CheckForUnattendedClasses...")

	now := time.Now()
	upperBound := now.Add(-5 * time.Minute)
	lowerBound := now.Add(-15 * time.Minute)

	var unattendedBookings []models.Booking

	err := database.DB.
		Joins("JOIN availability_slots on bookings.availability_slot_id = availability_slots.id").
		Where("bookings.status = ? AND availability_slots.end_time BETWEEN ? AND ?", "confirmed", lowerBound, upperBound).
		Find(&unattendedBookings).Error

	if err != nil {
		log.Printf("Error checking for unattended classes: %v", err)
		return
	}

	if len(unattendedBookings) == 0 {
		log.Println("No unattended classes found.")
		return
	}

	for _, booking := range unattendedBookings {
		booking.Status = "unattended"
		database.DB.Save(&booking)
	}

	log.Printf("Marked %d booking(s) as unattended.", len(unattendedBookings))
}