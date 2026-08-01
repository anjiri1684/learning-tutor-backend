package jobs

import (
	"log"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/google/uuid"
)


func CheckForUnattendedClasses() {
	log.Println("Running job: CheckForUnattendedClasses...")

	now := time.Now()
	upperBound := now.Add(-5 * time.Minute)
	lowerBound := now.Add(-15 * time.Minute)

	var endedBookings []models.Booking

	err := database.DB.
		Joins("JOIN availability_slots on bookings.availability_slot_id = availability_slots.id").
		Where("bookings.status = ? AND availability_slots.end_time BETWEEN ? AND ?", "confirmed", lowerBound, upperBound).
		Find(&endedBookings).Error

	if err != nil {
		log.Printf("Error checking for ended classes: %v", err)
		return
	}

	if len(endedBookings) == 0 {
		log.Println("No ended classes to auto-complete.")
		return
	}

	bookingIDs := make([]uuid.UUID, 0, len(endedBookings))
	for _, booking := range endedBookings {
		bookingIDs = append(bookingIDs, booking.ID)
	}

	services.AutoCompleteEndedBookings(bookingIDs)
	log.Printf("Auto-completed %d booking(s).", len(bookingIDs))
}