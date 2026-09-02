package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/middleware"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ListAuditLogs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset := (page - 1) * limit

	var logs []models.AuditLog
	var total int64
	database.DB.Model(&models.AuditLog{}).Count(&total)
	database.DB.Preload("Actor").Order("created_at desc").Offset(offset).Limit(limit).Find(&logs)

	return c.JSON(fiber.Map{
		"data": logs,
		"meta": fiber.Map{
			"total":     total,
			"page":      page,
			"last_page": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

func getActorID(c *fiber.Ctx) uuid.UUID {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	actorID, _ := uuid.Parse(claims["user_id"].(string))
	return actorID
}

func ListPendingApplications(c *fiber.Ctx) error {
	var pendingTeachers []models.Teacher
	if err := database.DB.Preload("User").Where("status = ?", "pending").Find(&pendingTeachers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	return c.JSON(pendingTeachers)
}

func ManageApplication(c *fiber.Ctx) error {
	type MgtRequest struct {
		Status string `json:"status" validate:"required,oneof=active rejected"`
	}

	var req MgtRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	teacherUserID := c.Params("teacherId")

	var teacherApp models.Teacher
	if err := database.DB.Where("user_id = ?", teacherUserID).First(&teacherApp).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Application not found"})
	}

	var user models.User
	if err := database.DB.Where("id = ?", teacherUserID).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Associated user not found"})
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	teacherApp.Status = req.Status
	if err := tx.Save(&teacherApp).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update application status"})
	}

	if req.Status == "active" {
		user.Role = "teacher"
		if err := tx.Save(&user).Error; err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user role"})
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Transaction commit failed"})
	}

	userIDCopy := user.ID
	go services.RecordAuditLog(getActorID(c), "teacher_application."+req.Status, "user", &userIDCopy, "Teacher application for "+user.Email+" set to "+req.Status)

	switch req.Status {
	case "active":
		go func() {
			subject, html := notifications.TeacherApplicationApprovedTemplate(user.FullName)
			notifications.SendEmail(user.FullName, user.Email, subject, html)
		}()
		go services.CreateNotification(user.ID, "application_approved", "Application approved",
			"Your teacher application has been approved. You can now set your availability.", "/teacher")
	case "rejected":
		go func() {
			subject, html := notifications.TeacherApplicationRejectedTemplate(user.FullName)
			notifications.SendEmail(user.FullName, user.Email, subject, html)
		}()
		go services.CreateNotification(user.ID, "application_rejected", "Application update",
			"Your teacher application was not approved.", "/dashboard/apply-to-teach")
	}

	return c.JSON(fiber.Map{"message": "Application status updated successfully"})
}

type LanguageRequest struct {
	Name            string  `json:"name" validate:"required,min=2"`
	PricePerSession float64 `json:"price_per_session" validate:"required,gt=0"`
	Currency        string  `json:"currency" validate:"required,iso4217"`
}

func CreateLanguage(c *fiber.Ctx) error {
	var req LanguageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	language := models.Language{
		Name:            req.Name,
		PricePerSession: req.PricePerSession,
	}
	if err := database.DB.Create(&language).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create language"})
	}

	return c.Status(fiber.StatusCreated).JSON(language)
}

func ListLanguages(c *fiber.Ctx) error {
	var languages []models.Language
	database.DB.Find(&languages)
	return c.JSON(languages)
}

func UpdateLanguage(c *fiber.Ctx) error {
	langID := c.Params("languageId")
	var language models.Language
	if err := database.DB.Where("id = ?", langID).First(&language).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Language not found"})
	}

	var req LanguageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	language.Name = req.Name
	language.PricePerSession = req.PricePerSession
	database.DB.Save(&language)

	return c.JSON(language)
}

func DeleteLanguage(c *fiber.Ctx) error {
	langID := c.Params("languageId")
	result := database.DB.Delete(&models.Language{}, "id = ?", langID)

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete language"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Language not found"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

type AddLinkRequest struct {
	MeetingLink string `json:"meeting_link" validate:"required,url"`
}

func AddMeetingLink(c *fiber.Ctx) error {
	bookingID := c.Params("bookingId")

	var req AddLinkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var booking models.Booking
	if err := database.DB.Preload("Student").Preload("Teacher").Where("id = ?", bookingID).First(&booking).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}

	if booking.Status != "confirmed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Meeting links can only be added to confirmed bookings"})
	}

	booking.MeetingLink = &req.MeetingLink
	database.DB.Save(&booking)

	go func() {
		subject, html := notifications.MeetingLinkTemplate(booking.Student.FullName, req.MeetingLink)
		notifications.SendEmail(booking.Student.FullName, booking.Student.Email, subject, html)
		subject, html = notifications.MeetingLinkTemplate(booking.Teacher.FullName, req.MeetingLink)
		notifications.SendEmail(booking.Teacher.FullName, booking.Teacher.Email, subject, html)
	}()

	return c.JSON(fiber.Map{"message": "Meeting link added and notifications sent successfully"})
}

type DashboardAnalyticsResponse struct {
	TotalStudents              int64            `json:"total_students"`
	TotalActiveTeachers        int64            `json:"total_active_teachers"`
	TotalRevenue               float64          `json:"total_revenue"`
	BookingsLast30Days         int64            `json:"bookings_last_30_days"`
	PendingContactRequests     int64            `json:"pending_contact_requests"`
	PendingCorporateEnquiries  int64            `json:"pending_corporate_enquiries"`
	PendingTeacherApplications int64            `json:"pending_teacher_applications"`
	RecentBookings             []models.Booking `json:"recent_bookings"`
}

func GetDashboardAnalytics(c *fiber.Ctx) error {
	var response DashboardAnalyticsResponse
	var totalRevenue float64

	database.DB.Model(&models.User{}).Where("role = ?", "student").Count(&response.TotalStudents)

	database.DB.Model(&models.Teacher{}).Where("status = ?", "active").Count(&response.TotalActiveTeachers)

	database.DB.Model(&models.Payment{}).Where("status = ?", "succeeded").Select("COALESCE(SUM(amount), 0)").Row().Scan(&totalRevenue)
	response.TotalRevenue = totalRevenue

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	database.DB.Model(&models.Booking{}).Where("created_at > ?", thirtyDaysAgo).Count(&response.BookingsLast30Days)

	database.DB.Model(&models.ContactRequest{}).Where("status = ?", "new").Count(&response.PendingContactRequests)
	database.DB.Model(&models.CorporateEnquiry{}).Where("status = ?", "new").Count(&response.PendingCorporateEnquiries)
	database.DB.Model(&models.Teacher{}).Where("status = ?", "pending").Count(&response.PendingTeacherApplications)

	database.DB.Order("created_at desc").Limit(5).Preload("Student").Preload("Teacher").Find(&response.RecentBookings)

	return c.JSON(response)
}

func ListRefundRequests(c *fiber.Ctx) error {
	var payments []models.Payment
	database.DB.Preload("Booking.Student").Where("refund_status = ?", "requested").Find(&payments)
	return c.JSON(payments)
}

func ProcessRefund(c *fiber.Ctx) error {
	paymentID := c.Params("paymentId")

	type ProcessRequest struct {
		Decision string `json:"decision" validate:"required,oneof=approve reject"`
	}
	var req ProcessRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var payment models.Payment
	if err := database.DB.Preload("Booking.Student").First(&payment, "id = ?", paymentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payment record not found"})
	}

	if req.Decision == "approve" {
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			approvedStatus := "approved"
			refundedStatus := "refunded"
			payment.RefundStatus = &approvedStatus
			payment.Status = refundedStatus
			if err := tx.Save(&payment).Error; err != nil {
				return err
			}

			var booking models.Booking
			if err := tx.First(&booking, "id = ?", payment.BookingID).Error; err != nil {
				return err
			}
			booking.Status = "cancelled"
			if err := tx.Save(&booking).Error; err != nil {
				return err
			}

			var slot models.AvailabilitySlot
			if err := tx.First(&slot, "id = ?", booking.AvailabilitySlotID).Error; err != nil {
				return err
			}
			if slot.CurrentStudents > 0 {
				slot.CurrentStudents--
			}
			if slot.CurrentStudents < slot.MaxStudents {
				slot.Status = "available"
			}
			if err := tx.Save(&slot).Error; err != nil {
				return err
			}

			if payment.Provider == "credit" {
				var student models.User
				if err := tx.First(&student, "id = ?", booking.StudentID).Error; err != nil {
					return err
				}
				student.CreditBalance += payment.Amount
				if err := tx.Save(&student).Error; err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update internal records for refund"})
		}

		go func() {
			subject, html := notifications.RefundProcessedTemplate(payment.Booking.Student.FullName)
			notifications.SendEmail(payment.Booking.Student.FullName, payment.Booking.Student.Email, subject, html)
		}()
		go services.CreateNotification(payment.Booking.StudentID, "refund_approved", "Refund approved",
			"Your refund request has been approved.", "/dashboard/my-classes")
		go services.PromoteNextWaitlistEntry(payment.Booking.AvailabilitySlotID)
		go services.RecordAuditLog(getActorID(c), "refund.approve", "payment", &payment.ID, "Approved refund for "+payment.Booking.Student.Email)

	} else {
		rejectedStatus := "rejected"
		payment.RefundStatus = &rejectedStatus
		database.DB.Save(&payment)

		go func() {
			subject, html := notifications.RefundRejectedTemplate(payment.Booking.Student.FullName)
			notifications.SendEmail(payment.Booking.Student.FullName, payment.Booking.Student.Email, subject, html)
		}()
		go services.CreateNotification(payment.Booking.StudentID, "refund_rejected", "Refund rejected",
			"Your refund request was not approved.", "/dashboard/my-classes")
		go services.RecordAuditLog(getActorID(c), "refund.reject", "payment", &payment.ID, "Rejected refund for "+payment.Booking.Student.Email)
	}

	return c.JSON(fiber.Map{"message": "Refund request processed successfully"})
}

func GenerateTransactionReport(c *fiber.Ctx) error {
	startDateStr := c.Query("start_date", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	endDateStr := c.Query("end_date", time.Now().Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid start_date format. Use YYYY-MM-DD."})
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid end_date format. Use YYYY-MM-DD."})
	}
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	var payments []models.Payment
	database.DB.
		Preload("Booking.Student").
		Preload("StudentBundle.Student").
		Where("status = ? AND created_at BETWEEN ? AND ?", "succeeded", startDate, endDate).
		Order("created_at desc").
		Find(&payments)

	b := new(bytes.Buffer)
	w := csv.NewWriter(b)

	headers := []string{"Transaction ID", "Date", "Student Name", "Amount", "Provider", "Type", "Reference ID"}
	if err := w.Write(headers); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to write CSV header"})
	}

	for _, p := range payments {
		var studentName, purchaseType, referenceID string
		if p.BookingID != nil {
			studentName = p.Booking.Student.FullName
			purchaseType = "Single Class"
			referenceID = p.BookingID.String()
		} else if p.StudentBundleID != nil {
			studentName = p.StudentBundle.Student.FullName
			purchaseType = "Bundle"
			referenceID = p.StudentBundleID.String()
		}

		row := []string{
			*p.ProviderTxnID,
			p.CreatedAt.Format("2006-01-02 15:04"),
			studentName,
			fmt.Sprintf("%.2f", p.Amount),
			p.Provider,
			purchaseType,
			referenceID,
		}
		if err := w.Write(row); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to write CSV row"})
		}
	}
	w.Flush()

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"transactions_%s_to_%s.csv\"", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))

	return c.Send(b.Bytes())
}

type PayoutRequest struct {
	Amount float64 `json:"amount" validate:"required,gt=0"`
}

func RequestPayout(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var req PayoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var teacher models.Teacher
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&teacher, "user_id = ?", teacherID).Error; err != nil {
			return errors.New("teacher profile not found")
		}
		if teacher.CurrentBalance < req.Amount {
			return errors.New("insufficient balance for this payout request")
		}

		teacher.CurrentBalance -= req.Amount
		if err := tx.Save(&teacher).Error; err != nil {
			return err
		}

		payoutRequest := models.PayoutRequest{
			TeacherID:   teacherID,
			Amount:      req.Amount,
			Status:      "pending",
			RequestedAt: time.Now(),
		}
		if err := tx.Create(&payoutRequest).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "Payout request submitted successfully."})
}

func ListPayoutRequests(c *fiber.Ctx) error {
	var requests []models.PayoutRequest
	database.DB.Preload("Teacher").Where("status = ?", "pending").Find(&requests)
	return c.JSON(requests)
}

func ProcessPayoutRequest(c *fiber.Ctx) error {
	requestID := c.Params("requestId")

	type ProcessRequest struct {
		Decision   string `json:"decision" validate:"required,oneof=complete reject"`
		AdminNotes string `json:"admin_notes"`
	}
	var req ProcessRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var payoutRequest models.PayoutRequest
	if err := database.DB.Preload("Teacher").First(&payoutRequest, "id = ?", requestID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payout request not found"})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		payoutRequest.Status = req.Decision
		payoutRequest.AdminNotes = &req.AdminNotes
		payoutRequest.ProcessedAt = &now

		if err := tx.Save(&payoutRequest).Error; err != nil {
			return err
		}

		if req.Decision == "reject" {
			if err := tx.Model(&models.Teacher{}).Where("user_id = ?", payoutRequest.TeacherID).Update("current_balance", gorm.Expr("current_balance + ?", payoutRequest.Amount)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process payout request"})
	}

	teacher := payoutRequest.Teacher
	if req.Decision == "complete" {
		go func() {
			subject, html := notifications.PayoutProcessedTemplate(teacher.FullName, payoutRequest.Amount)
			notifications.SendEmail(teacher.FullName, teacher.Email, subject, html)
		}()
		go services.CreateNotification(payoutRequest.TeacherID, "payout_completed", "Payout processed",
			"Your payout request has been completed.", "/teacher/earnings")
		go services.RecordAuditLog(getActorID(c), "payout.complete", "payout_request", &payoutRequest.ID, "Completed payout for "+teacher.Email)
	} else {
		go func() {
			subject, html := notifications.PayoutRejectedTemplate(teacher.FullName, payoutRequest.Amount, req.AdminNotes)
			notifications.SendEmail(teacher.FullName, teacher.Email, subject, html)
		}()
		go services.CreateNotification(payoutRequest.TeacherID, "payout_rejected", "Payout rejected",
			"Your payout request was not approved.", "/teacher/earnings")
		go services.RecordAuditLog(getActorID(c), "payout.reject", "payout_request", &payoutRequest.ID, "Rejected payout for "+teacher.Email)
	}

	return c.JSON(fiber.Map{"message": "Payout request processed."})
}

func GetAllUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := strings.TrimSpace(c.Query("search"))
	offset := (page - 1) * limit

	var users []models.User
	var totalUsers int64

	query := database.DB.Model(&models.User{})
	countQuery := database.DB.Model(&models.User{})

	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("full_name ILIKE ? OR email ILIKE ?", searchTerm, searchTerm)
		countQuery = countQuery.Where("full_name ILIKE ? OR email ILIKE ?", searchTerm, searchTerm)
	}

	countQuery.Count(&totalUsers)
	query.Offset(offset).Limit(limit).Find(&users)

	return c.JSON(fiber.Map{
		"data": users,
		"meta": fiber.Map{
			"total_users":  totalUsers,
			"total_pages":  int(math.Ceil(float64(totalUsers) / float64(limit))),
			"current_page": page,
		},
	})
}

func ToggleUserStatus(c *fiber.Ctx) error {
	userID := c.Params("userId")
	type Request struct {
		IsActive bool `json:"is_active"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("is_active", req.IsActive).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(fiber.Map{"message": "User status updated successfully."})
}

type AdminBundleRequest struct {
	Name            string  `json:"name" validate:"required"`
	LanguageID      string  `json:"language_id" validate:"required,uuid"`
	NumberOfClasses int     `json:"number_of_classes" validate:"required,gt=0"`
	Price           float64 `json:"price" validate:"required,gt=0"`
	Type            string  `json:"type" validate:"omitempty,oneof=standard corporate"`
	Description     string  `json:"description"`
}

func adminNormalizeBundleType(t string) string {
	if t == "corporate" {
		return "corporate"
	}
	return "standard"
}

func AdminListBundles(c *fiber.Ctx) error {
	var bundles []models.Bundle
	database.DB.Preload("Language").Find(&bundles)
	return c.JSON(bundles)
}

func AdminCreateBundle(c *fiber.Ctx) error {
	var req AdminBundleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	bundle := models.Bundle{
		Name:            req.Name,
		LanguageID:      uuid.MustParse(req.LanguageID),
		NumberOfClasses: req.NumberOfClasses,
		Price:           req.Price,
		IsActive:        true,
		Type:            adminNormalizeBundleType(req.Type),
		Description:     req.Description,
	}

	if err := database.DB.Create(&bundle).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create bundle"})
	}
	return c.Status(fiber.StatusCreated).JSON(bundle)
}

func AdminUpdateBundle(c *fiber.Ctx) error {
	bundleID := c.Params("bundleId")
	var bundle models.Bundle
	if err := database.DB.First(&bundle, "id = ?", bundleID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Bundle not found"})
	}

	var req AdminBundleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	bundle.Name = req.Name
	bundle.LanguageID = uuid.MustParse(req.LanguageID)
	bundle.NumberOfClasses = req.NumberOfClasses
	bundle.Price = req.Price
	bundle.Type = adminNormalizeBundleType(req.Type)
	bundle.Description = req.Description
	database.DB.Save(&bundle)

	return c.JSON(bundle)
}

func AdminDeactivateBundle(c *fiber.Ctx) error {
	bundleID := c.Params("bundleId")
	result := database.DB.Model(&models.Bundle{}).Where("id = ?", bundleID).Update("is_active", false)

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to deactivate bundle"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Bundle not found"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// AdminDeleteBundle permanently removes a bundle that has never been purchased.
func AdminDeleteBundle(c *fiber.Ctx) error {
	bundleID := c.Params("bundleId")

	var purchases int64
	database.DB.Model(&models.StudentBundle{}).Where("bundle_id = ?", bundleID).Count(&purchases)
	if purchases > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "This package has purchase history and cannot be deleted. Deactivate it instead.",
		})
	}

	result := database.DB.Delete(&models.Bundle{}, "id = ?", bundleID)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete bundle"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Bundle not found"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func AdminGetAllBookings(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	status := c.Query("status")
	offset := (page - 1) * limit

	var bookings []models.Booking
	var totalBookings int64

	query := database.DB.Model(&models.Booking{})
	countQuery := database.DB.Model(&models.Booking{})

	if status != "" {
		query = query.Where("status = ?", status)
		countQuery = countQuery.Where("status = ?", status)
	}

	countQuery.Count(&totalBookings)
	query.Order("created_at desc").Offset(offset).Limit(limit).Preload("Student").Preload("Teacher").Find(&bookings)

	return c.JSON(fiber.Map{
		"data": bookings,
		"meta": fiber.Map{
			"total":     totalBookings,
			"page":      page,
			"last_page": int(math.Ceil(float64(totalBookings) / float64(limit))),
		},
	})
}

func AdminGetPayments(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Payment{})
	countQuery := database.DB.Model(&models.Payment{})

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
		countQuery = countQuery.Where("status = ?", status)
	}
	if provider := c.Query("provider"); provider != "" {
		query = query.Where("provider = ?", provider)
		countQuery = countQuery.Where("provider = ?", provider)
	}

	var total int64
	var payments []models.Payment
	countQuery.Count(&total)
	query.Order("created_at desc").Offset(offset).Limit(limit).Preload("Booking.Student").Preload("StudentBundle.Student").Find(&payments)

	return c.JSON(fiber.Map{
		"data": payments,
		"meta": fiber.Map{"total": total, "page": page, "last_page": int(math.Ceil(float64(total) / float64(limit)))},
	})
}

func AdminGetReviews(c *fiber.Ctx) error {
	var reviews []models.Review
	database.DB.Order("created_at desc").Preload("Student").Preload("Teacher.User").Find(&reviews)
	return c.JSON(reviews)
}

func AdminDeleteReview(c *fiber.Ctx) error {
	reviewID := c.Params("reviewId")

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var review models.Review
		if err := tx.First(&review, "id = ?", reviewID).Error; err != nil {
			return errors.New("review not found")
		}

		teacherID := review.TeacherID

		if err := tx.Delete(&review).Error; err != nil {
			return err
		}

		var result struct{ Avg float64 }
		tx.Model(&models.Review{}).Where("teacher_id = ?", teacherID).Select("COALESCE(AVG(rating), 0) as avg").Scan(&result)

		if err := tx.Model(&models.Teacher{}).Where("user_id = ?", teacherID).Update("avg_rating", result.Avg).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	go services.RecordAuditLog(getActorID(c), "review.delete", "review", nil, "Deleted review "+reviewID)

	return c.SendStatus(fiber.StatusNoContent)
}

type AdminCreateUserRequest struct {
	FullName         string   `json:"full_name" validate:"required,min=2"`
	Email            string   `json:"email" validate:"required,email"`
	Password         string   `json:"password" validate:"required,min=6"`
	Role             string   `json:"role" validate:"required,oneof=student teacher admin coach"`
	Headline         string   `json:"headline"`
	Bio              string   `json:"bio"`
	AdminPermissions []string `json:"admin_permissions"`
}

// buildAdminPermissions returns a JSON-array string of the admin-area sections a
// coach is allowed to see. It returns nil for every non-coach role (full admins
// are unrestricted; students/teachers have no admin area). A coach always gets a
// concrete list ("[]" when nothing was granted) so access is deny-by-default.
func buildAdminPermissions(role string, perms []string) *string {
	if role != "coach" {
		return nil
	}
	valid := make([]string, 0, len(perms))
	for _, p := range perms {
		for _, s := range middleware.AdminSections {
			if p == s {
				valid = append(valid, p)
				break
			}
		}
	}
	b, err := json.Marshal(valid)
	if err != nil {
		empty := "[]"
		return &empty
	}
	str := string(b)
	return &str
}

func AdminCreateUser(c *fiber.Ctx) error {
	var req AdminCreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to hash password",
		})
	}

	var newUser models.User
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		newUser = models.User{
			FullName:         req.FullName,
			Email:            req.Email,
			Password:         string(hashedPassword),
			Role:             req.Role,
			AdminPermissions: buildAdminPermissions(req.Role, req.AdminPermissions),
		}
		if err := tx.Create(&newUser).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errors.New("email already exists")
			}
			return err
		}

		if req.Role == "teacher" {
			teacher := models.Teacher{
				UserID:   newUser.ID,
				Headline: &req.Headline,
				Bio:      &req.Bio,
				Status:   "active",
			}
			if err := tx.Create(&teacher).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		if err.Error() == "email already exists" {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Email already exists",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create user",
		})
	}

	go func() {
		subject, html := notifications.WelcomeTemplate(newUser.FullName)
		notifications.SendEmail(newUser.FullName, newUser.Email, subject, html)
	}()

	newUserIDCopy := newUser.ID
	go services.RecordAuditLog(getActorID(c), "user.create", "user", &newUserIDCopy, "Created "+req.Role+" user "+req.Email)

	return c.Status(fiber.StatusCreated).JSON(newUser)
}

type AdminUpdateUserRequest struct {
	FullName         string   `json:"full_name" validate:"required,min=2"`
	Email            string   `json:"email" validate:"required,email"`
	Role             string   `json:"role" validate:"required,oneof=student teacher admin coach"`
	AdminPermissions []string `json:"admin_permissions"`
}

func AdminUpdateUser(c *fiber.Ctx) error {
	userID := c.Params("userId")

	var req AdminUpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	previousRole := user.Role

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		user.FullName = req.FullName
		user.Email = req.Email
		user.Role = req.Role
		user.AdminPermissions = buildAdminPermissions(req.Role, req.AdminPermissions)
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		if previousRole != "teacher" && req.Role == "teacher" {
			var existingTeacher models.Teacher
			if err := tx.Where("user_id = ?", user.ID).First(&existingTeacher).Error; err != nil {
				teacher := models.Teacher{UserID: user.ID, Status: "active"}
				if err := tx.Create(&teacher).Error; err != nil {
					return err
				}
			}
		} else if previousRole == "teacher" && req.Role != "teacher" {
			if err := tx.Model(&models.Teacher{UserID: user.ID}).Association("Languages").Clear(); err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", user.ID).Delete(&models.Teacher{}).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already exists"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user"})
	}

	return c.JSON(user)
}

type AdminUpdateTeacherProfileRequest struct {
	Headline string `json:"headline" validate:"required"`
	Bio      string `json:"bio" validate:"required"`
	Status   string `json:"status" validate:"required,oneof=pending active rejected"`
}

func AdminUpdateTeacherProfile(c *fiber.Ctx) error {
	teacherID := c.Params("teacherId")

	var req AdminUpdateTeacherProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var teacher models.Teacher
	if err := database.DB.Where("user_id = ?", teacherID).First(&teacher).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher not found"})
	}

	teacher.Headline = &req.Headline
	teacher.Bio = &req.Bio
	teacher.Status = req.Status
	if err := database.DB.Save(&teacher).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update teacher"})
	}

	return c.JSON(teacher)
}

type AdminAssignClassRequest struct {
	TeacherID   string `json:"teacher_id" validate:"required,uuid"`
	StudentID   string `json:"student_id" validate:"required,uuid"`
	LanguageID  string `json:"language_id" validate:"required,uuid"`
	StartTime   string `json:"start_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	EndTime     string `json:"end_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	MeetingLink string `json:"meeting_link" validate:"omitempty,url"`
}

func AdminAssignClass(c *fiber.Ctx) error {
	var req AdminAssignClassRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid start_time"})
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid end_time"})
	}
	if !startTime.Before(endTime) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Start time must be before end time"})
	}

	var teacher models.Teacher
	if err := database.DB.Preload("User").First(&teacher, "user_id = ?", req.TeacherID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher not found"})
	}

	var student models.User
	if err := database.DB.First(&student, "id = ?", req.StudentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Student not found"})
	}

	var language models.Language
	if err := database.DB.First(&language, "id = ?", req.LanguageID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Language not found"})
	}

	teacherID, _ := uuid.Parse(req.TeacherID)
	studentID, _ := uuid.Parse(req.StudentID)
	languageID, _ := uuid.Parse(req.LanguageID)

	var booking models.Booking
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		slot := models.AvailabilitySlot{
			TeacherID:       teacherID,
			LanguageID:      &languageID,
			StartTime:       startTime,
			EndTime:         endTime,
			Status:          "booked",
			MaxStudents:     1,
			CurrentStudents: 1,
		}
		if err := tx.Create(&slot).Error; err != nil {
			return err
		}

		booking = models.Booking{
			StudentID:          studentID,
			TeacherID:          teacherID,
			AvailabilitySlotID: slot.ID,
			Status:             "confirmed",
			Price:              0,
			Currency:           language.Currency,
		}
		if req.MeetingLink != "" {
			booking.MeetingLink = &req.MeetingLink
		}
		services.ApplyDefaultMeetingLink(tx, &booking)

		if err := tx.Create(&booking).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to assign class"})
	}

	go services.CreateNotification(studentID, "class_assigned", "A class has been scheduled for you",
		language.Name+" class with "+teacher.User.FullName+" has been added to your classes by an admin.", "/dashboard/my-classes")
	go services.CreateNotification(teacherID, "class_assigned", "A class has been scheduled for you",
		language.Name+" class with "+student.FullName+" has been added to your classes by an admin.", "/teacher/classes")

	bookingIDCopy := booking.ID
	go services.RecordAuditLog(getActorID(c), "booking.assign", "booking", &bookingIDCopy,
		"Assigned "+language.Name+" class between teacher "+teacher.User.Email+" and student "+student.Email)

	return c.Status(fiber.StatusCreated).JSON(booking)
}

type AdminUpdateTeacherMeetingLinkRequest struct {
	DefaultMeetingLink string `json:"default_meeting_link" validate:"required,url"`
}

func AdminUpdateTeacherMeetingLink(c *fiber.Ctx) error {
	teacherID := c.Params("teacherId")

	var req AdminUpdateTeacherMeetingLinkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var teacher models.Teacher
	if err := database.DB.Where("user_id = ?", teacherID).First(&teacher).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Teacher not found"})
	}

	teacher.DefaultMeetingLink = &req.DefaultMeetingLink
	database.DB.Save(&teacher)

	return c.JSON(teacher)
}

type BulkImportSkipped struct {
	Row    int    `json:"row"`
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

const maxBulkImportFileSize = 2 * 1024 * 1024 // 2MB; a user-list CSV has no legitimate reason to be larger

func AdminBulkImportUsers(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "CSV file is required"})
	}
	if file.Size > maxBulkImportFileSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "CSV file is too large (max 2MB)"})
	}

	fileHandle, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open uploaded file"})
	}
	defer fileHandle.Close()

	reader := csv.NewReader(fileHandle)
	rows, err := reader.ReadAll()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to parse CSV: " + err.Error()})
	}

	if len(rows) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "CSV file is empty"})
	}

	startRow := 0
	header := rows[0]
	if len(header) > 0 && strings.EqualFold(strings.TrimSpace(header[0]), "full_name") {
		startRow = 1
	}

	var created []models.User
	var skipped []BulkImportSkipped

	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		if len(row) < 4 {
			skipped = append(skipped, BulkImportSkipped{Row: rowNum, Reason: "Expected columns: full_name,email,password,role"})
			continue
		}

		fullName := strings.TrimSpace(row[0])
		email := strings.TrimSpace(row[1])
		password := strings.TrimSpace(row[2])
		role := strings.ToLower(strings.TrimSpace(row[3]))

		if fullName == "" || email == "" || password == "" {
			skipped = append(skipped, BulkImportSkipped{Row: rowNum, Email: email, Reason: "Missing required field"})
			continue
		}
		if role != "student" && role != "teacher" {
			skipped = append(skipped, BulkImportSkipped{Row: rowNum, Email: email, Reason: "Role must be student or teacher"})
			continue
		}
		if len(password) < 6 {
			skipped = append(skipped, BulkImportSkipped{Row: rowNum, Email: email, Reason: "Password must be at least 6 characters"})
			continue
		}

		var existing models.User
		if err := database.DB.Where("email = ?", email).First(&existing).Error; err == nil {
			skipped = append(skipped, BulkImportSkipped{Row: rowNum, Email: email, Reason: "Email already exists"})
			continue
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			skipped = append(skipped, BulkImportSkipped{Row: rowNum, Email: email, Reason: "Failed to hash password"})
			continue
		}

		newUser := models.User{
			FullName: fullName,
			Email:    email,
			Password: string(hashedPassword),
			Role:     role,
		}

		txErr := database.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&newUser).Error; err != nil {
				return err
			}
			if role == "teacher" {
				teacher := models.Teacher{UserID: newUser.ID, Status: "active"}
				if err := tx.Create(&teacher).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if txErr != nil {
			skipped = append(skipped, BulkImportSkipped{Row: rowNum, Email: email, Reason: "Failed to create user"})
			continue
		}

		created = append(created, newUser)
		go func(u models.User) {
			subject, html := notifications.WelcomeTemplate(u.FullName)
			notifications.SendEmail(u.FullName, u.Email, subject, html)
		}(newUser)
	}

	go services.RecordAuditLog(getActorID(c), "user.bulk_import", "user", nil,
		fmt.Sprintf("Bulk imported %d users (%d skipped)", len(created), len(skipped)))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"created": created,
		"skipped": skipped,
	})
}

func AdminImpersonateUser(c *fiber.Ctx) error {
	userID := c.Params("userId")

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	actorID := getActorID(c)

	claims := jwt.MapClaims{
		"user_id":         user.ID.String(),
		"role":            user.Role,
		"impersonated_by": actorID.String(),
		"exp":             time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(config.Config("JWT_SECRET")))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create impersonation token"})
	}

	userIDCopy := user.ID
	go services.RecordAuditLog(actorID, "user.impersonate", "user", &userIDCopy, "Impersonated "+user.Role+" user "+user.Email)

	return c.JSON(fiber.Map{"token": signedToken, "user": user})
}

func AdminDeleteUser(c *fiber.Ctx) error {
	userID := c.Params("userId")
	var deletedUserEmail, deletedUserRole string

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, "id = ?", userID).Error; err != nil {
			return errors.New("user not found")
		}
		deletedUserEmail = user.Email
		deletedUserRole = user.Role

		if user.Role == "teacher" {
			if err := tx.Where("teacher_id = ?", userID).Delete(&models.Review{}).Error; err != nil {
				return err
			}
			if err := tx.Where("teacher_id = ?", userID).Delete(&models.Booking{}).Error; err != nil {
				return err
			}
			if err := tx.Where("teacher_id = ?", userID).Delete(&models.AvailabilitySlot{}).Error; err != nil {
				return err
			}
			if err := tx.Where("teacher_id = ?", userID).Delete(&models.PayoutRequest{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Teacher{UserID: user.ID}).Association("Languages").Clear(); err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", userID).Delete(&models.Teacher{}).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Where("student_id = ?", userID).Delete(&models.Review{}).Error; err != nil {
				return err
			}
			if err := tx.Where("student_id = ?", userID).Delete(&models.Booking{}).Error; err != nil {
				return err
			}
			if err := tx.Where("student_id = ?", userID).Delete(&models.Waitlist{}).Error; err != nil {
				return err
			}
			if err := tx.Where("student_id = ?", userID).Delete(&models.LibraryResourceAccess{}).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("sender_id = ?", userID).Delete(&models.Message{}).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM conversation_participants WHERE user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.Notification{}).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM library_resource_accesses WHERE library_resource_id IN (SELECT id FROM library_resources WHERE uploaded_by = ?)", userID).Error; err != nil {
			return err
		}
		if err := tx.Where("uploaded_by = ?", userID).Delete(&models.LibraryResource{}).Error; err != nil {
			return err
		}

		if err := tx.Delete(&user).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	go services.RecordAuditLog(getActorID(c), "user.delete", "user", nil, "Deleted "+deletedUserRole+" user "+deletedUserEmail)

	return c.SendStatus(fiber.StatusNoContent)
}

type AdminSendEmailRequest struct {
	Emails  []string `json:"emails"`
	UserIDs []string `json:"user_ids"`
	Roles   []string `json:"roles"`
	Subject string   `json:"subject" validate:"required"`
	Message string   `json:"message" validate:"required"`
}

type EmailRecipientResult struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

const maxEmailRecipients = 300

func AdminSendEmail(c *fiber.Ctx) error {
	var req AdminSendEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	recipients := make(map[string]string) // email -> name

	for _, raw := range req.Emails {
		email := strings.ToLower(strings.TrimSpace(raw))
		if email == "" || !strings.Contains(email, "@") {
			continue
		}
		if _, exists := recipients[email]; !exists {
			recipients[email] = ""
		}
	}

	if len(req.UserIDs) > 0 {
		var users []models.User
		database.DB.Where("id IN ?", req.UserIDs).Find(&users)
		for _, u := range users {
			recipients[strings.ToLower(u.Email)] = u.FullName
		}
	}

	if len(req.Roles) > 0 {
		var users []models.User
		database.DB.Where("role IN ?", req.Roles).Find(&users)
		for _, u := range users {
			recipients[strings.ToLower(u.Email)] = u.FullName
		}
	}

	if len(recipients) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No valid recipients specified"})
	}
	if len(recipients) > maxEmailRecipients {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Too many recipients (max %d per send)", maxEmailRecipients)})
	}

	results := make([]EmailRecipientResult, 0, len(recipients))
	for email, name := range recipients {
		_, htmlContent := notifications.AdminCustomMessageTemplate(name, req.Subject, req.Message)
		notifications.SendEmail(name, email, req.Subject, htmlContent)
		results = append(results, EmailRecipientResult{Email: email, Name: name})
	}

	services.RecordAuditLog(getActorID(c), "admin.send_email", "email", nil, fmt.Sprintf("Sent %q to %d recipient(s)", req.Subject, len(recipients)))

	return c.JSON(fiber.Map{
		"message":    fmt.Sprintf("Email sent to %d recipient(s)", len(recipients)),
		"recipients": results,
	})
}
