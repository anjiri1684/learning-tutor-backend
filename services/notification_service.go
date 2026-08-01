package services

import (
	"log"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/google/uuid"
)

func CreateNotification(userID uuid.UUID, notifType, title, body, link string) {
	notification := models.Notification{
		UserID: userID,
		Type:   notifType,
		Title:  title,
		Body:   body,
		Link:   link,
	}
	if err := database.DB.Create(&notification).Error; err != nil {
		log.Printf("Failed to create notification for user %s: %v", userID, err)
	}
}

func PromoteNextWaitlistEntry(slotID uuid.UUID) {
	var entry models.Waitlist
	if err := database.DB.Order("created_at asc").First(&entry, "availability_slot_id = ?", slotID).Error; err != nil {
		return
	}

	now := time.Now()
	entry.NotifiedAt = &now
	database.DB.Save(&entry)

	CreateNotification(entry.StudentID, "waitlist_spot_open", "A spot opened up!",
		"A spot just opened up in a class you were waitlisted for. Book now before it fills up again.", "/dashboard/find-teachers")

	database.DB.Delete(&entry)
}

func NotifyAllAdmins(notifType, title, body, link string) {
	var admins []models.User
	if err := database.DB.Where("role = ?", "admin").Find(&admins).Error; err != nil {
		log.Printf("Failed to fetch admins for notification: %v", err)
		return
	}
	for _, admin := range admins {
		CreateNotification(admin.ID, notifType, title, body, link)
	}
}
