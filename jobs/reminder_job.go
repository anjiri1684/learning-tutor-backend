package jobs

import (
	"fmt"
	"log"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
)

func SendClassReminders() {
	log.Println("Running job: SendClassReminders...")

	now := time.Now()
	lowerBound := now.Add(60 * time.Minute)
	upperBound := now.Add(65 * time.Minute)

	var upcomingBookings []models.Booking

	err := database.DB.
		Preload("Student").
		Preload("Teacher").
		Preload("AvailabilitySlot").
		Where("bookings.status = ? AND availability_slots.start_time BETWEEN ? AND ?", "confirmed", lowerBound, upperBound).
		Joins("JOIN availability_slots on bookings.availability_slot_id = availability_slots.id").
		Find(&upcomingBookings).Error

	if err != nil {
		log.Printf("Error checking for upcoming classes: %v", err)
		return
	}

	if len(upcomingBookings) == 0 {
		return
	}

	for _, booking := range upcomingBookings {
		log.Printf("Sending reminder for booking ID: %s", booking.ID)
		
		emailSubject := "Reminder: Your Class Starts in 1 Hour!"
		emailBody := fmt.Sprintf(
			"<h1>Class Reminder</h1><p>Hi there,</p><p>This is a friendly reminder that your class is scheduled to start in one hour at %s.</p><p><b>Meeting Link:</b> <a href='%s'>Join Class</a></p>",
			booking.AvailabilitySlot.StartTime.Format(time.Kitchen),
			*booking.MeetingLink,
		)
		
		go notifications.SendEmail(booking.Student.FullName, booking.Student.Email, emailSubject, emailBody)
		go notifications.SendEmail(booking.Teacher.FullName, booking.Teacher.Email, emailSubject, emailBody)
	}
}