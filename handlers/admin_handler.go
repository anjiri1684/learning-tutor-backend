package handlers

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

	switch req.Status {
	case "active":
		go notifications.SendEmail(
			user.FullName,
			user.Email,
			"Your Teacher Application has been Approved!",
			"<h1>Congratulations!</h1><p>Your application to become a teacher has been approved. You can now set your availability and start teaching.</p>",
		)
	case "rejected":
		go notifications.SendEmail(
			user.FullName,
			user.Email,
			"Update on Your Teacher Application",
			"<h1>Application Update</h1><p>We regret to inform you that after careful review, your teacher application was not approved at this time.</p>",
		)
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
		Name: req.Name,
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
		emailSubject := "Your Meeting Link is Here!"
		emailBody := fmt.Sprintf("<h1>Class Link</h1><p>Hi there,</p><p>Here is the link for your upcoming class: <a href='%s'>Join Class</a>.</p>", req.MeetingLink)
		
		notifications.SendEmail(booking.Student.FullName, booking.Student.Email, emailSubject, emailBody)
		notifications.SendEmail(booking.Teacher.FullName, booking.Teacher.Email, emailSubject, emailBody)
	}()

	return c.JSON(fiber.Map{"message": "Meeting link added and notifications sent successfully"})
}


type DashboardAnalyticsResponse struct {
	TotalStudents       int64           `json:"total_students"`
	TotalActiveTeachers int64           `json:"total_active_teachers"`
	TotalRevenue        float64         `json:"total_revenue"`
	BookingsLast30Days  int64           `json:"bookings_last_30_days"`
	RecentBookings      []models.Booking `json:"recent_bookings"`
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
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

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
			if err := tx.Save(&payment).Error; err != nil { return err }

			var booking models.Booking
			if err := tx.First(&booking, "id = ?", payment.BookingID).Error; err != nil { return err }
			booking.Status = "cancelled"
			if err := tx.Save(&booking).Error; err != nil { return err }
			
			var slot models.AvailabilitySlot
			if err := tx.First(&slot, "id = ?", booking.AvailabilitySlotID).Error; err != nil { return err }
			slot.Status = "available"
			if err := tx.Save(&slot).Error; err != nil { return err }
			
			if payment.Provider == "credit" {
				var student models.User
				if err := tx.First(&student, "id = ?", booking.StudentID).Error; err != nil { return err }
				student.CreditBalance += payment.Amount
				if err := tx.Save(&student).Error; err != nil { return err }
			}
			
			return nil
		})
		if err != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update internal records for refund"}) }

		go notifications.SendEmail(payment.Booking.Student.FullName, payment.Booking.Student.Email, "Your Refund has been Processed", "<h1>Refund Processed</h1><p>Your refund request has been approved and processed by our team.</p>")

	} else { 
		rejectedStatus := "rejected"
		payment.RefundStatus = &rejectedStatus
		database.DB.Save(&payment)

		go notifications.SendEmail(payment.Booking.Student.FullName, payment.Booking.Student.Email, "Update on Your Refund Request", "<h1>Refund Request Update</h1><p>Your refund request has been reviewed and was not approved.</p>")
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
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var teacher models.Teacher
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&teacher, "user_id = ?", teacherID).Error; err != nil {
			return errors.New("teacher profile not found")
		}
		if teacher.CurrentBalance < req.Amount {
			return errors.New("insufficient balance for this payout request")
		}

		teacher.CurrentBalance -= req.Amount
		if err := tx.Save(&teacher).Error; err != nil { return err }

		payoutRequest := models.PayoutRequest{
			TeacherID:   teacherID,
			Amount:      req.Amount,
			Status:      "pending",
			RequestedAt: time.Now(),
		}
		if err := tx.Create(&payoutRequest).Error; err != nil { return err }

		return nil
	})
	if err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }
	

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
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	var payoutRequest models.PayoutRequest
	if err := database.DB.Preload("Teacher").First(&payoutRequest, "id = ?", requestID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payout request not found"})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		payoutRequest.Status = req.Decision
		payoutRequest.AdminNotes = &req.AdminNotes
		payoutRequest.ProcessedAt = &now

		if err := tx.Save(&payoutRequest).Error; err != nil { return err }

		if req.Decision == "reject" {
			if err := tx.Model(&models.Teacher{}).Where("user_id = ?", payoutRequest.TeacherID).Update("current_balance", gorm.Expr("current_balance + ?", payoutRequest.Amount)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process payout request"}) }
	
	teacher := payoutRequest.Teacher
	if req.Decision == "complete" {
		go notifications.SendEmail(
			teacher.FullName,
			teacher.Email,
			"Your Payout Has Been Processed",
			fmt.Sprintf("<h1>Payout Processed</h1><p>Hello %s,</p><p>Your payout request for the amount of $%.2f has been processed and sent by our team.</p>", teacher.FullName, payoutRequest.Amount),
		)
	} else {
		go notifications.SendEmail(
			teacher.FullName,
			teacher.Email,
			"Update on Your Payout Request",
			fmt.Sprintf("<h1>Payout Request Update</h1><p>Hello %s,</p><p>Your payout request for the amount of $%.2f was rejected. The funds have been returned to your account balance.</p><p><b>Admin Notes:</b> %s</p>", teacher.FullName, payoutRequest.Amount, req.AdminNotes),
		)
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
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }

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
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	bundle.Name = req.Name
	bundle.LanguageID = uuid.MustParse(req.LanguageID)
	bundle.NumberOfClasses = req.NumberOfClasses
	bundle.Price = req.Price
	database.DB.Save(&bundle)

	return c.JSON(bundle)
}

func AdminDeactivateBundle(c *fiber.Ctx) error {
	bundleID := c.Params("bundleId")
	result := database.DB.Model(&models.Bundle{}).Where("id = ?", bundleID).Update("is_active", false)

	if result.Error != nil { return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to deactivate bundle"}) }
	if result.RowsAffected == 0 { return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Bundle not found"}) }

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
			"total": totalBookings,
			"page":  page,
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
		"meta": fiber.Map{ "total": total, "page": page, "last_page": int(math.Ceil(float64(total) / float64(limit))) },
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
	
	return c.SendStatus(fiber.StatusNoContent)
}


func AdminDeleteUser(c *fiber.Ctx) error {
	userID := c.Params("userId")

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, "id = ?", userID).Error; err != nil {
			return errors.New("user not found")
		}

		if user.Role == "teacher" {
			if err := tx.Where("teacher_id = ?", userID).Delete(&models.Review{}).Error; err != nil { return err }
			if err := tx.Where("teacher_id = ?", userID).Delete(&models.Booking{}).Error; err != nil { return err }
			if err := tx.Where("teacher_id = ?", userID).Delete(&models.AvailabilitySlot{}).Error; err != nil { return err }
			if err := tx.Where("teacher_id = ?", userID).Delete(&models.PayoutRequest{}).Error; err != nil { return err }
			if err := tx.Model(&models.Teacher{UserID: user.ID}).Association("Languages").Clear(); err != nil { return err }
			if err := tx.Where("user_id = ?", userID).Delete(&models.Teacher{}).Error; err != nil { return err }
		}
		
		if err := tx.Delete(&user).Error; err != nil {
			return err
		}
		
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}