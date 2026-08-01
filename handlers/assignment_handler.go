package handlers

import (
	"fmt"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type CreateAssignmentRequest struct {
	Title        string `json:"title" validate:"required,min=2"`
	Instructions string `json:"instructions"`
	DueDate      string `json:"due_date" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
}

func CreateAssignment(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var req CreateAssignmentRequest
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

	assignment := models.Assignment{
		BookingID:    booking.ID,
		TeacherID:    teacherID,
		StudentID:    booking.StudentID,
		Title:        req.Title,
		Instructions: req.Instructions,
	}
	if req.DueDate != "" {
		dueDate, err := time.Parse(time.RFC3339, req.DueDate)
		if err == nil {
			assignment.DueDate = &dueDate
		}
	}

	if err := database.DB.Create(&assignment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create assignment"})
	}

	go services.CreateNotification(booking.StudentID, "assignment_created", "New assignment: "+req.Title,
		"Your teacher has assigned new homework.", "/dashboard/my-classes")

	return c.Status(fiber.StatusCreated).JSON(assignment)
}

func ListBookingAssignments(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	bookingID := c.Params("bookingId")

	var booking models.Booking
	if err := database.DB.First(&booking, "id = ?", bookingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Booking not found"})
	}
	if booking.StudentID != userID && booking.TeacherID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not have access to this booking"})
	}

	var assignments []models.Assignment
	database.DB.Preload("Submission").Where("booking_id = ?", bookingID).Order("created_at desc").Find(&assignments)

	return c.JSON(assignments)
}

func ListMyAssignments(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	role := claims["role"].(string)

	var assignments []models.Assignment
	query := database.DB.Preload("Submission").Order("created_at desc")
	if role == "teacher" {
		query = query.Where("teacher_id = ?", userID)
	} else {
		query = query.Where("student_id = ?", userID)
	}
	query.Find(&assignments)

	return c.JSON(assignments)
}

type SubmitAssignmentRequest struct {
	SubmissionText string `json:"submission_text"`
	SubmissionLink string `json:"submission_link" validate:"omitempty,url"`
}

func SubmitAssignment(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	assignmentID := c.Params("assignmentId")

	var assignment models.Assignment
	if err := database.DB.First(&assignment, "id = ?", assignmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Assignment not found"})
	}
	if assignment.StudentID != studentID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "This is not your assignment"})
	}

	var submissionText, submissionLink string
	contentType := c.Get("Content-Type")

	var fileObjectKey, fileName *string

	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		submissionText = c.FormValue("submission_text")
		submissionLink = c.FormValue("submission_link")

		if file, err := c.FormFile("file"); err == nil && file.Size <= maxLibraryResourceFileSize {
			fileHandle, ferr := file.Open()
			if ferr == nil {
				defer fileHandle.Close()
				key := fmt.Sprintf("assignments/%s/%s", assignmentID, file.Filename)
				ct := file.Header.Get("Content-Type")
				if ct == "" {
					ct = "application/octet-stream"
				}
				if uerr := services.UploadObject(key, fileHandle, ct); uerr == nil {
					fileObjectKey = &key
					fileName = &file.Filename
				}
			}
		}
	} else {
		var req SubmitAssignmentRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
		}
		submissionText = req.SubmissionText
		submissionLink = req.SubmissionLink
	}

	if submissionText == "" && submissionLink == "" && fileObjectKey == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Provide text, a link, or a file"})
	}

	var submission models.AssignmentSubmission
	err := database.DB.Where("assignment_id = ?", assignmentID).First(&submission).Error

	now := time.Now()
	if err == nil {
		if submissionText != "" {
			submission.SubmissionText = &submissionText
		}
		if submissionLink != "" {
			submission.SubmissionLink = &submissionLink
		}
		if fileObjectKey != nil {
			submission.FileObjectKey = fileObjectKey
			submission.FileName = fileName
		}
		submission.SubmittedAt = now
		submission.Grade = nil
		submission.Feedback = nil
		submission.GradedAt = nil
		database.DB.Save(&submission)
	} else {
		submission = models.AssignmentSubmission{
			AssignmentID: assignment.ID,
			SubmittedAt:  now,
		}
		if submissionText != "" {
			submission.SubmissionText = &submissionText
		}
		if submissionLink != "" {
			submission.SubmissionLink = &submissionLink
		}
		if fileObjectKey != nil {
			submission.FileObjectKey = fileObjectKey
			submission.FileName = fileName
		}
		database.DB.Create(&submission)
	}

	go services.CreateNotification(assignment.TeacherID, "assignment_submitted", "Assignment submitted: "+assignment.Title,
		"A student has submitted their assignment.", "/teacher/classes")

	return c.Status(fiber.StatusCreated).JSON(submission)
}

type GradeAssignmentRequest struct {
	Grade    float64 `json:"grade" validate:"required,min=0,max=100"`
	Feedback string  `json:"feedback"`
}

func GradeAssignment(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	teacherID, _ := uuid.Parse(claims["user_id"].(string))
	assignmentID := c.Params("assignmentId")

	var assignment models.Assignment
	if err := database.DB.First(&assignment, "id = ?", assignmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Assignment not found"})
	}
	if assignment.TeacherID != teacherID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "This is not your assignment"})
	}

	var req GradeAssignmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var submission models.AssignmentSubmission
	if err := database.DB.Where("assignment_id = ?", assignmentID).First(&submission).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No submission found for this assignment"})
	}

	now := time.Now()
	submission.Grade = &req.Grade
	submission.Feedback = &req.Feedback
	submission.GradedAt = &now
	database.DB.Save(&submission)

	go services.CreateNotification(assignment.StudentID, "assignment_graded", "Assignment graded: "+assignment.Title,
		fmt.Sprintf("You received a grade of %.0f/100.", req.Grade), "/dashboard/my-classes")

	return c.JSON(submission)
}

func ViewAssignmentSubmissionFile(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	assignmentID := c.Params("assignmentId")

	var assignment models.Assignment
	if err := database.DB.First(&assignment, "id = ?", assignmentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Assignment not found"})
	}
	if assignment.StudentID != userID && assignment.TeacherID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not have access to this assignment"})
	}

	var submission models.AssignmentSubmission
	if err := database.DB.Where("assignment_id = ?", assignmentID).First(&submission).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No submission found"})
	}
	if submission.FileObjectKey == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No file was submitted"})
	}

	stream, contentType, err := services.GetObjectStream(*submission.FileObjectKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve file from storage"})
	}

	fileName := "submission"
	if submission.FileName != nil {
		fileName = *submission.FileName
	}

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fileName))
	c.Set("Cache-Control", "no-store")

	return c.SendStream(stream)
}
