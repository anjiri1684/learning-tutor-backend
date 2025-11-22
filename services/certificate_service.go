package services

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
)

const certificateCompletionCount = 10

func CheckAndGenerateCertificate(booking models.Booking) {
	var completedCount int64
	database.DB.Model(&models.Booking{}).
		Joins("JOIN availability_slots on bookings.availability_slot_id = availability_slots.id").
		Where("bookings.student_id = ? AND bookings.teacher_id = ? AND availability_slots.language_id = ? AND bookings.status = ?",
			booking.StudentID, booking.TeacherID, booking.AvailabilitySlot.LanguageID, "completed").
		Count(&completedCount)

	if completedCount < certificateCompletionCount {
		return 
	}

	courseTitle := fmt.Sprintf("%s with %s - %d Sessions", booking.AvailabilitySlot.Language.Name, booking.Teacher.FullName, certificateCompletionCount)

	var existingCert models.Certificate
	if err := database.DB.Where("student_id = ? AND course_title = ?", booking.StudentID, courseTitle).First(&existingCert).Error; err == nil {
		return 
	}

	htmlData, err := generateCertificateHTML(booking.Student.FullName, booking.Teacher.FullName, courseTitle)
	if err != nil {
		log.Printf("🔥 Failed to generate certificate HTML: %v", err)
		return
	}

	pdfBytes, err := generatePDFFromHTML(htmlData)
	if err != nil {
		log.Printf("🔥 Failed to generate PDF: %v", err)
		return
	}

	uploadURL, err := uploadToCloudinary(pdfBytes, booking.StudentID.String())
	if err != nil {
		log.Printf("🔥 Failed to upload certificate to Cloudinary: %v", err)
		return
	}

	newCertificate := models.Certificate{
		StudentID:      booking.StudentID,
		TeacherID:      booking.TeacherID,
		LanguageID:     *booking.AvailabilitySlot.LanguageID,
		CourseTitle:    courseTitle,
		CompletionDate: time.Now(),
		CertificateURL: uploadURL,
	}

	if err := database.DB.Create(&newCertificate).Error; err != nil {
		log.Printf("🔥 Failed to create certificate record for student %s: %v", booking.StudentID, err)
	} else {
		log.Printf("✅ Generated and uploaded certificate '%s' for student %s.", courseTitle, booking.StudentID)
	}
}

func generateCertificateHTML(studentName, teacherName, courseTitle string) (string, error) {
	tmpl, err := template.ParseFiles("templates/certificate.html")
	if err != nil {
		return "", err
	}

	data := struct {
		StudentName    string
		TeacherName    string
		CourseTitle    string
		CompletionDate string
	}{
		StudentName:    studentName,
		TeacherName:    teacherName,
		CourseTitle:    courseTitle,
		CompletionDate: time.Now().Format("January 2, 2006"),
	}

	var renderedHTML bytes.Buffer
	if err := tmpl.Execute(&renderedHTML, data); err != nil {
		return "", err
	}
	return renderedHTML.String(), nil
}

func generatePDFFromHTML(htmlContent string) ([]byte, error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var pdfBuffer []byte
	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frameTree.Frame.ID, htmlContent).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			pdf, _, err := page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			if err != nil {
				return err
			}
			pdfBuffer = pdf
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	return pdfBuffer, nil
}

func uploadToCloudinary(fileBytes []byte, studentID string) (string, error) {
	cld, err := cloudinary.NewFromURL(config.Config("CLOUDINARY_URL"))
	if err != nil {
		return "", err
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	uploadParams := uploader.UploadParams{
		PublicID: fmt.Sprintf("certificates/%s_%s", studentID, uuid.New().String()),
		Folder:   "language_tutor_certificates",
		ResourceType: "raw",
	}

	uploadResult, err := cld.Upload.Upload(ctx, bytes.NewReader(fileBytes), uploadParams)
	if err != nil {
		return "", err
	}

	return uploadResult.SecureURL, nil
}