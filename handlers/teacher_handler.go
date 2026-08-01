package handlers

import (
	"errors"
	"strconv"
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TeacherApplicationRequest struct {
	Headline           string `json:"headline" validate:"required"`
	Bio                string `json:"bio" validate:"required"`
	VerificationDocURL string `json:"verification_doc_url" validate:"omitempty,url"`
}

func ApplyToBeATeacher(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userIDStr := claims["user_id"].(string)
	userID, _ := uuid.Parse(userIDStr)

	var req TeacherApplicationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var existingTeacher models.Teacher
	err := database.DB.Where("user_id = ?", userID).First(&existingTeacher).Error
	if err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "You have already submitted an application."})
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	newApplication := models.Teacher{
		UserID:   userID,
		Headline: &req.Headline,
		Bio:      &req.Bio,
	}
	if req.VerificationDocURL != "" {
		newApplication.VerificationDocURL = &req.VerificationDocURL
	}

	if err := database.DB.Create(&newApplication).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create application"})
	}

	return c.Status(fiber.StatusCreated).JSON(newApplication)
}


type CreateAvailabilityRequest struct {
	StartTime   string `json:"start_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	EndTime     string `json:"end_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	LanguageID  string `json:"language_id" validate:"required,uuid"`
	MaxStudents int    `json:"max_students,omitempty"` 
}

func CreateAvailabilitySlot(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherIDStr := claims["user_id"].(string)
	teacherID, _ := uuid.Parse(teacherIDStr)

	var req CreateAvailabilityRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)

	if startTime.After(endTime) || startTime.Equal(endTime) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Start time must be before end time"})
	}

	maxStudents := 1
	if req.MaxStudents > 1 {
		maxStudents = req.MaxStudents
	}

	newSlot := models.AvailabilitySlot{
		TeacherID:   teacherID,
		LanguageID: func() *uuid.UUID { 
			
			id := uuid.MustParse(req.LanguageID); return &id }(),
		StartTime:   startTime,
		EndTime:     endTime,
		MaxStudents: maxStudents, 
	}

	if err := database.DB.Create(&newSlot).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create availability slot"})
	}

	return c.Status(fiber.StatusCreated).JSON(newSlot)
}

type CreateRecurringAvailabilityRequest struct {
	StartTime    string `json:"start_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	EndTime      string `json:"end_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	LanguageID   string `json:"language_id" validate:"required,uuid"`
	MaxStudents  int    `json:"max_students,omitempty"`
	RepeatWeeks  int    `json:"repeat_weeks" validate:"required,gt=0,lte=52"`
}

func CreateRecurringAvailability(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var req CreateRecurringAvailabilityRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)

	if !startTime.Before(endTime) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Start time must be before end time"})
	}

	languageID := uuid.MustParse(req.LanguageID)
	maxStudents := 1
	if req.MaxStudents > 1 {
		maxStudents = req.MaxStudents
	}

	var createdSlots []models.AvailabilitySlot
	for i := 0; i < req.RepeatWeeks; i++ {
		offset := time.Duration(i) * 7 * 24 * time.Hour
		slot := models.AvailabilitySlot{
			TeacherID:   teacherID,
			LanguageID:  &languageID,
			StartTime:   startTime.Add(offset),
			EndTime:     endTime.Add(offset),
			MaxStudents: maxStudents,
		}
		if err := database.DB.Create(&slot).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create recurring availability slots"})
		}
		createdSlots = append(createdSlots, slot)
	}

	return c.Status(fiber.StatusCreated).JSON(createdSlots)
}

func GetMyAvailability(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherIDStr := claims["user_id"].(string)

	var slots []models.AvailabilitySlot
	database.DB.Where("teacher_id = ?", teacherIDStr).Find(&slots)

	return c.JSON(slots)
}



func GetTeacherAvailability(c *fiber.Ctx) error {
	teacherID := c.Params("teacherId")

	var slots []models.AvailabilitySlot
	database.DB.Where("teacher_id = ? AND status IN ('available', 'booked', 'full') AND start_time > ?", teacherID, time.Now()).
		Order("start_time asc").
		Find(&slots)

	return c.JSON(slots)
}



type AddLanguageRequest struct {
	LanguageID string `json:"language_id" validate:"required,uuid"`
}

func AddLanguageToProfile(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherIDStr := claims["user_id"].(string)

	var req AddLanguageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var teacher models.Teacher
	if err := database.DB.Where("user_id = ?", teacherIDStr).First(&teacher).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher profile not found"})
	}

	var language models.Language
	if err := database.DB.Where("id = ?", req.LanguageID).First(&language).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Language not found"})
	}
	
	database.DB.Model(&teacher).Association("Languages").Append(&language)

	return c.JSON(fiber.Map{"message": "Language added successfully"})
}

func RemoveLanguageFromProfile(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherIDStr := claims["user_id"].(string)
	langID := c.Params("languageId")

	var teacher models.Teacher
	if err := database.DB.Where("user_id = ?", teacherIDStr).First(&teacher).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher profile not found"})
	}

	var language models.Language
	if err := database.DB.Where("id = ?", langID).First(&language).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Language not found"})
	}
	
	database.DB.Model(&teacher).Association("Languages").Delete(&language)
	
	return c.SendStatus(fiber.StatusNoContent)
}



func ListRescheduleRequests(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var requests []models.Booking
	database.DB.Preload("Student").Where("teacher_id = ? AND status = ?", teacherID, "reschedule_requested").Find(&requests)
	
	return c.JSON(requests)
}

func ProcessReschedule(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	type ProcessRequest struct {
		Decision string `json:"decision" validate:"required,oneof=approve reject"`
	}
	var req ProcessRequest
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	var booking models.Booking
	if err := database.DB.Preload("Student").First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}
	if booking.TeacherID != teacherID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "This is not your booking to manage"})
	}

	if req.Decision == "approve" {
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			var slot models.AvailabilitySlot
			if err := tx.First(&slot, "id = ?", booking.AvailabilitySlotID).Error; err != nil { return err }

			slot.StartTime = *booking.ProposedStartTime
			slot.EndTime = *booking.ProposedEndTime
			if err := tx.Save(&slot).Error; err != nil { return err }

			booking.Status = "confirmed"
			booking.ProposedStartTime = nil
			booking.ProposedEndTime = nil
			if err := tx.Save(&booking).Error; err != nil { return err }
			
			return nil
		})
		if err != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process reschedule"}) }
		
		go func() {
			subject, html := notifications.RescheduleApprovedTemplate(booking.Student.FullName)
			notifications.SendEmail(booking.Student.FullName, booking.Student.Email, subject, html)
		}()
		go services.CreateNotification(booking.StudentID, "reschedule_approved", "Reschedule approved",
			"Your reschedule request was approved.", "/dashboard/my-classes")

	} else {
		booking.Status = "confirmed"
		booking.ProposedStartTime = nil
		booking.ProposedEndTime = nil
		database.DB.Save(&booking)
		
		go func() {
			subject, html := notifications.RescheduleRejectedTemplate(booking.Student.FullName)
			notifications.SendEmail(booking.Student.FullName, booking.Student.Email, subject, html)
		}()
	}

	return c.JSON(fiber.Map{"message": "Reschedule request processed successfully"})
}

func GetTeacherProfile(c *fiber.Ctx) error {
	teacherID := c.Params("teacherId")
	
	var teacher models.Teacher
	if err := database.DB.Preload("User").Preload("Languages").First(&teacher, "user_id = ? AND status = ?", teacherID, "active").Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Active teacher not found"})
	}

	return c.JSON(teacher)
}


func ListActiveTeachers(c *fiber.Ctx) error {
	var activeTeachers []models.Teacher
	query := database.DB.Preload("User").Preload("Languages").Where("status = ?", "active")

	
	if langID := c.Query("language_id"); langID != "" {
		query = query.Joins("JOIN teacher_languages ON teacher_languages.teacher_user_id = teachers.user_id").Where("teacher_languages.language_id = ?", langID)
	}
	if minRating := c.Query("min_rating"); minRating != "" {
		query = query.Where("avg_rating >= ?", minRating)
	}

	if err := query.Find(&activeTeachers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve teachers"})
	}

	return c.JSON(activeTeachers)
}


func DeleteAvailabilitySlot(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	slotID := c.Params("slotId")

	var slot models.AvailabilitySlot
	if err := database.DB.First(&slot, "id = ? AND teacher_id = ?", slotID, teacherID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Availability slot not found or you do not have permission to delete it."})
	}

	if slot.Status != "available" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot delete a slot that has already been booked."})
	}

	database.DB.Delete(&slot)

	return c.SendStatus(fiber.StatusNoContent)
}


func GetTeacherEarnings(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var teacher models.Teacher
	if err := database.DB.First(&teacher, "user_id = ?", teacherID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher profile not found"})
	}

	return c.JSON(fiber.Map{"current_balance": teacher.CurrentBalance})
}

func GetMyPayoutRequests(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var requests []models.PayoutRequest
	database.DB.Where("teacher_id = ?", teacherID).Order("requested_at desc").Find(&requests)

	return c.JSON(requests)
}



func GetMyTeacherProfile(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var teacher models.Teacher
	if err := database.DB.Preload("User").First(&teacher, "user_id = ?", teacherID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher profile not found"})
	}
	return c.JSON(teacher)
}

func UpdateMyTeacherProfile(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	
	type UpdateRequest struct {
		Headline           string `json:"headline" validate:"required"`
		Bio                string `json:"bio" validate:"required"`
		DefaultMeetingLink string `json:"default_meeting_link" validate:"omitempty,url"`
	}
	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	var teacher models.Teacher
	if err := database.DB.First(&teacher, "user_id = ?", teacherID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher profile not found"})
	}

	teacher.Headline = &req.Headline
	teacher.Bio = &req.Bio
	if req.DefaultMeetingLink != "" {
		teacher.DefaultMeetingLink = &req.DefaultMeetingLink
	} else {
		teacher.DefaultMeetingLink = nil
	}
	database.DB.Save(&teacher)

	return c.JSON(teacher)
}



func GetMyReviews(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var reviews []models.Review
	database.DB.Preload("Student").Where("teacher_id = ?", teacherID).Order("created_at desc").Find(&reviews)

	return c.JSON(reviews)
}


func GetStudentProgressForTeacher(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	studentID := c.Params("studentId")

	var student models.User
	if err := database.DB.First(&student, "id = ?", studentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Student not found"})
	}

	var bookings []models.Booking
	database.DB.
		Preload("AvailabilitySlot.Language").
		Where("teacher_id = ? AND student_id = ?", teacherID, studentID).
		Order("availability_slots.start_time desc").
		Joins("JOIN availability_slots on bookings.availability_slot_id = availability_slots.id").
		Find(&bookings)

	var totalClasses int64
	database.DB.Model(&models.Booking{}).Where("teacher_id = ? AND student_id = ? AND status = 'completed'", teacherID, studentID).Count(&totalClasses)
	
	var avgRating struct{ Avg float64 }
	database.DB.Model(&models.Review{}).Where("teacher_id = ? AND student_id = ?", teacherID, studentID).Select("COALESCE(AVG(rating), 0) as avg").Scan(&avgRating)

	return c.JSON(fiber.Map{
		"student_name":   student.FullName,
		"total_classes":  totalClasses,
		"average_rating": avgRating.Avg,
		"bookings":       bookings,
	})
}



type MonthlyEarning struct {
	Month    string  `json:"month"`
	Earnings float64 `json:"earnings"`
}

type UpdatePayoutMethodRequest struct {
	PayoutMethod        string `json:"payout_method" validate:"required,oneof=bank mpesa"`
	PayoutAccountName   string `json:"payout_account_name" validate:"required"`
	PayoutAccountNumber string `json:"payout_account_number" validate:"required"`
	PayoutBankName      string `json:"payout_bank_name"`
}

func UpdateMyPayoutMethod(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var req UpdatePayoutMethodRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var teacher models.Teacher
	if err := database.DB.First(&teacher, "user_id = ?", teacherID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher profile not found"})
	}

	teacher.PayoutMethod = &req.PayoutMethod
	teacher.PayoutAccountName = &req.PayoutAccountName
	teacher.PayoutAccountNumber = &req.PayoutAccountNumber
	if req.PayoutBankName != "" {
		teacher.PayoutBankName = &req.PayoutBankName
	} else {
		teacher.PayoutBankName = nil
	}
	database.DB.Save(&teacher)

	return c.JSON(teacher)
}

func GetTeacherAnalytics(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var teacher models.Teacher
	if err := database.DB.First(&teacher, "user_id = ?", teacherID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher profile not found"})
	}

	var totalClasses int64
	database.DB.Model(&models.Booking{}).Where("teacher_id = ? AND status = 'completed'", teacherID).Count(&totalClasses)

	commissionRate, _ := strconv.ParseFloat(config.Config("PLATFORM_COMMISSION_RATE"), 64)
	teacherShare := 1 - commissionRate

	var monthlyEarnings []MonthlyEarning
	database.DB.Model(&models.Booking{}).
		Select("TO_CHAR(created_at, 'YYYY-MM') as month, SUM(price * ?) as earnings", teacherShare).
		Where("teacher_id = ? AND status IN ?", teacherID, []string{"completed", "confirmed"}).
		Group("month").
		Order("month asc").
		Scan(&monthlyEarnings)
		
	var totalEarnings float64
	for _, me := range monthlyEarnings {
		totalEarnings += me.Earnings
	}

	return c.JSON(fiber.Map{
		"total_earnings":        totalEarnings,
		"average_rating":        teacher.AvgRating,
		"total_classes_taught":  totalClasses,
		"monthly_earnings_data": monthlyEarnings,
	})
}