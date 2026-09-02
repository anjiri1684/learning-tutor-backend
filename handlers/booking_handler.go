package handlers

import (
	"errors"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/anjiri1684/language_tutor/payments"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreateBookingRequest struct {
	AvailabilitySlotID string `json:"availability_slot_id" validate:"required,uuid"`
	UseCredit          bool   `json:"use_credit,omitempty"`
	PaymentProvider    string `json:"payment_provider,omitempty"`
	MpesaPhoneNumber   string `json:"mpesa_phone_number,omitempty"`
}

func CreateBooking(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))

	var req CreateBookingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	slotID, _ := uuid.Parse(req.AvailabilitySlotID)

	var slot models.AvailabilitySlot
	if err := database.DB.Preload("Language").First(&slot, "id = ?", slotID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Availability slot not found"})
	}
	if slot.Language.ID == uuid.Nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Availability slot has invalid language"})
	}

	if req.UseCredit {
		var student models.User
		if err := database.DB.First(&student, "id = ?", studentID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Student not found"})
		}

		if student.CreditBalance >= slot.Language.PricePerSession {
			var confirmedBooking models.Booking
			err := database.DB.Transaction(func(tx *gorm.DB) error {
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&slot, "id = ?", slotID).Error; err != nil {
					return err
				}

				if slot.Status == "full" || slot.Status == "booked" || slot.CurrentStudents >= slot.MaxStudents {
					return errors.New("this class is full or no longer available")
				}
				slot.CurrentStudents++
				if slot.CurrentStudents >= slot.MaxStudents {
					if slot.MaxStudents > 1 {
						slot.Status = "full"
					} else {
						slot.Status = "booked"
					}
				}

				student.CreditBalance -= slot.Language.PricePerSession
				if err := tx.Save(&student).Error; err != nil {
					return err
				}
				if err := tx.Save(&slot).Error; err != nil {
					return err
				}

				confirmedBooking = models.Booking{
					StudentID: studentID, TeacherID: slot.TeacherID, AvailabilitySlotID: slot.ID,
					Price:    slot.Language.PricePerSession,
					Currency: slot.Language.Currency,
					Status:   "confirmed",
				}
				services.ApplyDefaultMeetingLink(tx, &confirmedBooking)
				if err := tx.Create(&confirmedBooking).Error; err != nil {
					return err
				}

				payment := models.Payment{
					BookingID: &confirmedBooking.ID,
					Amount:    confirmedBooking.Price,
					Currency:  slot.Language.Currency,
					Provider:  "credit",
					Status:    "succeeded",
				}
				if err := tx.Create(&payment).Error; err != nil {
					return err
				}

				go func() {
					if err := tx.Preload("Student").Preload("Teacher").First(&confirmedBooking).Error; err == nil {
						sSub, sHtml := notifications.CreditBookingConfirmedStudentTemplate(confirmedBooking.Student.FullName)
						notifications.SendEmail(confirmedBooking.Student.FullName, confirmedBooking.Student.Email, sSub, sHtml)
						tSub, tHtml := notifications.CreditBookingConfirmedTeacherTemplate(confirmedBooking.Teacher.FullName)
						notifications.SendEmail(confirmedBooking.Teacher.FullName, confirmedBooking.Teacher.Email, tSub, tHtml)
					}
				}()
				return nil
			})
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process credit payment: " + err.Error()})
			}

			database.DB.Preload("Student").Preload("Teacher").First(&confirmedBooking, "id = ?", confirmedBooking.ID)

			return c.Status(fiber.StatusCreated).JSON(fiber.Map{
				"message": "Booking confirmed successfully using your credit balance.",
				"booking": confirmedBooking,
			})
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Insufficient credit balance to make this purchase."})
		}
	}

	var price = slot.Language.PricePerSession
	var currency = slot.Language.Currency

	if req.PaymentProvider == "mpesa" || req.PaymentProvider == "paystack" {
		if currency != "KES" {
			kesPrice, err := services.ConvertUSDToKES(price)
			if err != nil {
				log.Printf("🔥 Currency conversion failed: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not perform currency conversion."})
			}
			price = math.Round(kesPrice)
			currency = "KES"
		}
	}

	var booking models.Booking
	var payment models.Payment
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&slot, "id = ?", slotID).Error; err != nil {
			return err
		}

		if slot.Status == "full" || slot.Status == "booked" || slot.CurrentStudents >= slot.MaxStudents {
			return errors.New("this class is full or no longer available")
		}
		slot.CurrentStudents++
		if slot.CurrentStudents >= slot.MaxStudents {
			if slot.MaxStudents > 1 {
				slot.Status = "full"
			} else {
				slot.Status = "booked"
			}
		}
		if err := tx.Save(&slot).Error; err != nil {
			return err
		}

		booking = models.Booking{
			StudentID: studentID, TeacherID: slot.TeacherID, AvailabilitySlotID: slot.ID,
			Price: slot.Language.PricePerSession, Currency: slot.Language.Currency, Status: "pending_payment",
		}
		if err := tx.Create(&booking).Error; err != nil {
			return err
		}

		payment = models.Payment{
			BookingID: &booking.ID, Amount: price, Currency: currency,
			Provider: req.PaymentProvider, Status: "pending",
		}
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Student").Preload("Teacher").First(&booking, "id = ?", booking.ID)

	if req.PaymentProvider == "mpesa" {
		if req.MpesaPhoneNumber == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "M-Pesa phone number is required"})
		}

		stkResponse, err := payments.InitiateMpesaSTKPush(price, req.MpesaPhoneNumber, payment.ID.String())
		if err != nil {
			log.Printf("🔥 CRITICAL: InitiateMpesaSTKPush failed: %v", err)
			if err.Error() == "invalid M-Pesa phone number format" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Payment could not be initiated, please try again."})
		}

		payment.MerchantRequestID = &stkResponse.Response.MerchantRequestID
		database.DB.Save(&payment)

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"booking":          booking,
			"customer_message": stkResponse.Response.CustomerMessage,
		})
	}

	if req.PaymentProvider == "paystack" {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"booking": booking, "payment_id": payment.ID})
	}

	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payment provider specified for external payment"})
}

type ReviewRequest struct {
	Rating  int    `json:"rating" validate:"required,min=1,max=5"`
	Comment string `json:"comment"`
}

func CreateReview(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var req ReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var newReview models.Review
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var booking models.Booking
		if err := tx.First(&booking, "id = ?", bookingID).Error; err != nil {
			return errors.New("booking not found")
		}
		if booking.StudentID != studentID {
			return errors.New("you are not the student for this booking")
		}
		if booking.Status != "completed" {
			return errors.New("reviews can only be submitted for completed bookings")
		}

		var existingReview models.Review
		if err := tx.Where("booking_id = ?", bookingID).First(&existingReview).Error; err == nil {
			return errors.New("a review for this booking has already been submitted")
		}

		newReview = models.Review{
			BookingID: booking.ID,
			StudentID: studentID,
			TeacherID: booking.TeacherID,
			Rating:    req.Rating,
			Comment:   req.Comment,
		}
		if err := tx.Create(&newReview).Error; err != nil {
			return err
		}

		var result struct {
			Avg float64
		}
		tx.Model(&models.Review{}).Where("teacher_id = ?", booking.TeacherID).Select("avg(rating) as avg").Scan(&result)

		if err := tx.Model(&models.Teacher{}).Where("user_id = ?", booking.TeacherID).Update("avg_rating", result.Avg).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var student models.User
	database.DB.First(&student, "id = ?", studentID)
	go services.CreateNotification(newReview.TeacherID, "review", "New review from "+student.FullName,
		"You received a "+strconv.Itoa(newReview.Rating)+"-star review.", "/teacher/reviews")

	return c.Status(fiber.StatusCreated).JSON(newReview)
}

func MarkBookingAsComplete(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var booking models.Booking
	if err := database.DB.
		Preload("AvailabilitySlot.Language").
		Preload("Student").
		Preload("Teacher").
		First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}

	if booking.TeacherID != teacherID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not the teacher for this booking"})
	}
	if booking.Status != "confirmed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Only confirmed bookings can be marked as complete"})
	}
	if booking.AvailabilitySlot.EndTime.After(time.Now()) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot mark a class as complete before it has ended"})
	}

	if err := services.CompleteBooking(booking.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to complete booking"})
	}

	return c.JSON(fiber.Map{"message": "Booking marked as complete and earnings have been credited."})
}

type ReportNoShowRequest struct {
	Reason string `json:"reason" validate:"required,min=5"`
}

func ReportNoShow(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var req ReportNoShowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var booking models.Booking
	if err := database.DB.Preload("Student").Preload("Teacher").First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}

	if booking.StudentID != userID && booking.TeacherID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not have access to this booking"})
	}
	if booking.Status != "completed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No-shows can only be reported for completed classes"})
	}

	reporter := "Student"
	if booking.TeacherID == userID {
		reporter = "Teacher"
	}

	go services.NotifyAllAdmins("no_show_reported", "No-show reported",
		reporter+" reported a no-show for the "+booking.Student.FullName+" / "+booking.Teacher.FullName+" class: "+req.Reason,
		"/admin/bookings")

	return c.JSON(fiber.Map{"message": "No-show reported. An admin will review it."})
}

type TeacherFeedbackRequest struct {
	Feedback string `json:"feedback" validate:"required,min=10"`
}

func SubmitTeacherFeedback(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var req TeacherFeedbackRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var booking models.Booking
	if err := database.DB.First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}

	if booking.TeacherID != teacherID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not the teacher for this booking"})
	}
	if booking.Status != "completed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Feedback can only be submitted for completed bookings"})
	}

	booking.TeacherFeedback = &req.Feedback
	if err := database.DB.Save(&booking).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save feedback"})
	}

	return c.JSON(fiber.Map{"message": "Feedback submitted successfully"})
}

type SubmitStudentRatingRequest struct {
	Rating  int    `json:"rating" validate:"required,min=1,max=5"`
	Comment string `json:"comment"`
}

func SubmitStudentRating(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var req SubmitStudentRatingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var booking models.Booking
	if err := database.DB.First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Booking not found",
		})
	}

	if booking.TeacherID != teacherID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not the teacher for this booking"})
	}
	if booking.Status != "completed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Ratings can only be submitted for completed bookings"})
	}
	if booking.StudentRating != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "A rating for this booking has already been submitted"})
	}

	booking.StudentRating = &req.Rating
	booking.StudentComment = &req.Comment
	if err := database.DB.Save(&booking).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save rating"})
	}

	go services.CreateNotification(booking.StudentID, "rating", "Your teacher rated your class",
		"You received a "+strconv.Itoa(req.Rating)+"-star rating.", "/dashboard/my-classes")

	return c.JSON(booking)
}

func GetPendingStudentRatings(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var bookings []models.Booking
	database.DB.
		Preload("Student").
		Preload("AvailabilitySlot.Language").
		Where("teacher_id = ? AND status = ? AND student_rating IS NULL", teacherID, "completed").
		Order("updated_at desc").
		Find(&bookings)

	return c.JSON(bookings)
}

type RefundRequest struct {
	Reason string `json:"reason" validate:"required"`
}

func RequestRefund(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var req RefundRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var booking models.Booking
	if err := database.DB.Preload("AvailabilitySlot").First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}
	if booking.StudentID != studentID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "This is not your booking"})
	}
	if booking.AvailabilitySlot.StartTime.Before(time.Now()) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot request a refund for a class that has already started or finished"})
	}

	var payment models.Payment
	database.DB.First(&payment, "booking_id = ?", bookingID)

	refundStatus := "requested"
	payment.RefundStatus = &refundStatus
	payment.RefundReason = &req.Reason
	database.DB.Save(&payment)

	go services.NotifyAllAdmins("refund_requested", "New refund request",
		"A student has requested a refund: "+req.Reason, "/admin/refunds")

	return c.JSON(fiber.Map{"message": "Refund request submitted successfully. An admin will review it shortly."})
}

type RescheduleRequest struct {
	NewStartTime string `json:"new_start_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	NewEndTime   string `json:"new_end_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
}

func RequestReschedule(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var req RescheduleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var booking models.Booking
	if err := database.DB.Preload("Teacher").First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}
	if booking.StudentID != studentID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "This is not your booking"})
	}

	newStartTime, _ := time.Parse(time.RFC3339, req.NewStartTime)
	newEndTime, _ := time.Parse(time.RFC3339, req.NewEndTime)
	if newStartTime.Before(time.Now()) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Proposed reschedule time cannot be in the past"})
	}

	booking.Status = "reschedule_requested"
	booking.ProposedStartTime = &newStartTime
	booking.ProposedEndTime = &newEndTime
	database.DB.Save(&booking)

	go func() {
		subject, html := notifications.RescheduleRequestTemplate(booking.Teacher.FullName)
		notifications.SendEmail(booking.Teacher.FullName, booking.Teacher.Email, subject, html)
	}()
	go services.CreateNotification(booking.TeacherID, "reschedule_requested", "Reschedule request",
		"A student has requested to reschedule a class.", "/teacher/reschedules")

	return c.JSON(fiber.Map{"message": "Reschedule request sent to the teacher."})
}

func generateICS(events []struct {
	UID         string
	Start       time.Time
	End         time.Time
	Summary     string
	Description string
	Location    string
}) string {
	const icsTimeFormat = "20060102T150405Z"
	now := time.Now().UTC().Format(icsTimeFormat)

	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\r\n")
	sb.WriteString("VERSION:2.0\r\n")
	sb.WriteString("PRODID:-//Language Tutor//EN\r\n")
	sb.WriteString("CALSCALE:GREGORIAN\r\n")

	for _, event := range events {
		sb.WriteString("BEGIN:VEVENT\r\n")
		sb.WriteString("UID:" + event.UID + "@languagetutor\r\n")
		sb.WriteString("DTSTAMP:" + now + "\r\n")
		sb.WriteString("DTSTART:" + event.Start.UTC().Format(icsTimeFormat) + "\r\n")
		sb.WriteString("DTEND:" + event.End.UTC().Format(icsTimeFormat) + "\r\n")
		sb.WriteString("SUMMARY:" + event.Summary + "\r\n")
		if event.Description != "" {
			sb.WriteString("DESCRIPTION:" + event.Description + "\r\n")
		}
		if event.Location != "" {
			sb.WriteString("LOCATION:" + event.Location + "\r\n")
		}
		sb.WriteString("END:VEVENT\r\n")
	}

	sb.WriteString("END:VCALENDAR\r\n")
	return sb.String()
}

func GetBookingCalendar(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var booking models.Booking
	if err := database.DB.
		Preload("Student").
		Preload("Teacher").
		Preload("AvailabilitySlot.Language").
		First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}

	if booking.StudentID != userID && booking.TeacherID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not have access to this booking"})
	}

	meetingLink := ""
	if booking.MeetingLink != nil {
		meetingLink = *booking.MeetingLink
	}

	ics := generateICS([]struct {
		UID         string
		Start       time.Time
		End         time.Time
		Summary     string
		Description string
		Location    string
	}{
		{
			UID:         booking.ID.String(),
			Start:       booking.AvailabilitySlot.StartTime,
			End:         booking.AvailabilitySlot.EndTime,
			Summary:     booking.AvailabilitySlot.Language.Name + " class with " + booking.Teacher.FullName,
			Description: "Language Tutor class with " + booking.Teacher.FullName + " and " + booking.Student.FullName,
			Location:    meetingLink,
		},
	})

	c.Set("Content-Type", "text/calendar; charset=utf-8")
	c.Set("Content-Disposition", "attachment; filename=\"class.ics\"")
	return c.SendString(ics)
}

func GetMyCalendar(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	role := claims["role"].(string)

	var bookings []models.Booking
	query := database.DB.
		Preload("Student").
		Preload("Teacher").
		Preload("AvailabilitySlot.Language").
		Where("bookings.status = ? AND availability_slots.start_time > ?", "confirmed", time.Now()).
		Joins("JOIN availability_slots on bookings.availability_slot_id = availability_slots.id")

	if role == "teacher" {
		query = query.Where("bookings.teacher_id = ?", userID)
	} else {
		query = query.Where("bookings.student_id = ?", userID)
	}
	query.Find(&bookings)

	events := make([]struct {
		UID         string
		Start       time.Time
		End         time.Time
		Summary     string
		Description string
		Location    string
	}, 0, len(bookings))

	for _, booking := range bookings {
		meetingLink := ""
		if booking.MeetingLink != nil {
			meetingLink = *booking.MeetingLink
		}
		events = append(events, struct {
			UID         string
			Start       time.Time
			End         time.Time
			Summary     string
			Description string
			Location    string
		}{
			UID:         booking.ID.String(),
			Start:       booking.AvailabilitySlot.StartTime,
			End:         booking.AvailabilitySlot.EndTime,
			Summary:     booking.AvailabilitySlot.Language.Name + " class with " + booking.Teacher.FullName,
			Description: "Language Tutor class with " + booking.Teacher.FullName + " and " + booking.Student.FullName,
			Location:    meetingLink,
		})
	}

	ics := generateICS(events)

	c.Set("Content-Type", "text/calendar; charset=utf-8")
	c.Set("Content-Disposition", "attachment; filename=\"my-classes.ics\"")
	return c.SendString(ics)
}

func GetMyBookings(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))

	var bookings []models.Booking
	database.DB.
		Preload("Teacher.User").
		Preload("AvailabilitySlot.Language").
		Where("student_id = ?", studentID).
		Order("availability_slots.start_time desc").
		Joins("JOIN availability_slots on bookings.availability_slot_id = availability_slots.id").
		Find(&bookings)

	return c.JSON(bookings)
}

func GetMyTeacherBookings(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))

	var bookings []models.Booking
	database.DB.
		Preload("Student").
		Preload("AvailabilitySlot.Language").
		Where("bookings.teacher_id = ?", teacherID).
		Order("availability_slots.start_time desc").
		Joins("JOIN availability_slots on bookings.availability_slot_id = availability_slots.id").
		Find(&bookings)

	return c.JSON(bookings)
}
