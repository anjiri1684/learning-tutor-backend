package services

import (
	"github.com/anjiri1684/language_tutor/models"
	"gorm.io/gorm"
)

func ApplyDefaultMeetingLink(tx *gorm.DB, booking *models.Booking) {
	if booking.MeetingLink != nil && *booking.MeetingLink != "" {
		return
	}

	var teacher models.Teacher
	if err := tx.First(&teacher, "user_id = ?", booking.TeacherID).Error; err != nil {
		return
	}

	if teacher.DefaultMeetingLink == nil || *teacher.DefaultMeetingLink == "" {
		return
	}

	booking.MeetingLink = teacher.DefaultMeetingLink
}
