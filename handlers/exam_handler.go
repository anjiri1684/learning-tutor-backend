package handlers

import (
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"gorm.io/gorm"
)


type QuestionRequest struct {
	QuestionText  string `json:"question_text" validate:"required"`
	QuestionType  string `json:"question_type" validate:"required"`
	Options       string `json:"options"` 
	CorrectAnswer string `json:"correct_answer" validate:"required"`
}

func CreateQuestion(c *fiber.Ctx) error {
	var req QuestionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	question := models.Question{
		QuestionText:  req.QuestionText,
		QuestionType:  req.QuestionType,
		Options:       req.Options,
		CorrectAnswer: req.CorrectAnswer,
	}

	if err := database.DB.Create(&question).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create question"})
	}

	return c.Status(fiber.StatusCreated).JSON(question)
}

func ListQuestions(c *fiber.Ctx) error {
	var questions []models.Question
	database.DB.Find(&questions)
	return c.JSON(questions)
}

func GetQuestion(c *fiber.Ctx) error {
	questionID := c.Params("questionId")
	var question models.Question
	if err := database.DB.First(&question, "id = ?", questionID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Question not found"})
	}
	return c.JSON(question)
}

func UpdateQuestion(c *fiber.Ctx) error {
	questionID := c.Params("questionId")
	var question models.Question
	if err := database.DB.First(&question, "id = ?", questionID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Question not found"})
	}

	var req QuestionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	question.QuestionText = req.QuestionText
	question.QuestionType = req.QuestionType
	question.Options = req.Options
	question.CorrectAnswer = req.CorrectAnswer
	database.DB.Save(&question)

	return c.JSON(question)
}

func DeleteQuestion(c *fiber.Ctx) error {
	questionID := c.Params("questionId")
	result := database.DB.Delete(&models.Question{}, "id = ?", questionID)

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete question"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Question not found"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}


type MockTestRequest struct {
	Title           string   `json:"title" validate:"required"`
	Description     string   `json:"description"`
	DurationMinutes int      `json:"duration_minutes" validate:"required,gt=0"`
	QuestionIDs     []string `json:"question_ids" validate:"required,min=1"`
}

func CreateMockTest(c *fiber.Ctx) error {
	var req MockTestRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var questions []*models.Question
	if err := database.DB.Where("id IN ?", req.QuestionIDs).Find(&questions).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to find questions"})
	}
	if len(questions) != len(req.QuestionIDs) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "One or more provided question IDs are invalid"})
	}

	mockTest := models.MockTest{
		Title:           req.Title,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		Questions:       questions,
	}
	
	if err := database.DB.Create(&mockTest).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create mock test"})
	}

	return c.Status(fiber.StatusCreated).JSON(mockTest)
}

func ListMockTests(c *fiber.Ctx) error {
	var tests []models.MockTest
	database.DB.Preload("Questions").Find(&tests)
	return c.JSON(tests)
}

func GetMockTest(c *fiber.Ctx) error {
	testID := c.Params("testId")
	var test models.MockTest
	if err := database.DB.Preload("Questions").First(&test, "id = ?", testID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Mock test not found"})
	}
	return c.JSON(test)
}

func UpdateMockTest(c *fiber.Ctx) error {
	testID := c.Params("testId")
	var req MockTestRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var mockTest models.MockTest
	if err := database.DB.First(&mockTest, "id = ?", testID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Mock test not found"})
	}

	var newQuestions []*models.Question
	if err := database.DB.Where("id IN ?", req.QuestionIDs).Find(&newQuestions).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to find new questions"})
	}
	if len(newQuestions) != len(req.QuestionIDs) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "One or more new question IDs are invalid"})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		mockTest.Title = req.Title
		mockTest.Description = req.Description
		mockTest.DurationMinutes = req.DurationMinutes
		
		if err := tx.Save(&mockTest).Error; err != nil { return err }

		if err := tx.Model(&mockTest).Association("Questions").Replace(newQuestions); err != nil {
			return err
		}
		
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update mock test"})
	}

	return c.Status(fiber.StatusOK).JSON(mockTest)
}

func DeleteMockTest(c *fiber.Ctx) error {
	testID := c.Params("testId")
	
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var mockTest models.MockTest
		if err := tx.Preload("Questions").First(&mockTest, "id = ?", testID).Error; err != nil {
			return err
		}
		if err := tx.Model(&mockTest).Association("Questions").Clear(); err != nil {
			return err
		}
		if err := tx.Delete(&mockTest).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete mock test"})
	}
	
	return c.SendStatus(fiber.StatusNoContent)
}

func StudentListMockTests(c *fiber.Ctx) error {
	var tests []models.MockTest
	database.DB.Select("id", "title", "description", "duration_minutes", "created_at").Find(&tests)
	return c.JSON(tests)
}

func StartTestAttempt(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	testID := c.Params("testId")

	var test models.MockTest
	if err := database.DB.Preload("Questions").First(&test, "id = ?", testID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Mock test not found"})
	}

	attempt := models.TestAttempt{
		StudentID:  studentID,
		MockTestID: test.ID,
		StartTime:  time.Now(),
	}
	if err := database.DB.Create(&attempt).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start test attempt"})
	}

	type QuestionForStudent struct {
		ID           uuid.UUID `json:"id"`
		QuestionText string    `json:"question_text"`
		QuestionType string    `json:"question_type"`
		Options      string    `json:"options"`
	}
	
	questionsForStudent := make([]QuestionForStudent, len(test.Questions))
	for i, q := range test.Questions {
		questionsForStudent[i] = QuestionForStudent{
			ID:           q.ID,
			QuestionText: q.QuestionText,
			QuestionType: q.QuestionType,
			Options:      q.Options,
		}
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"attempt_id":        attempt.ID,
		"test_title":        test.Title,
		"duration_minutes":  test.DurationMinutes,
		"questions":         questionsForStudent,
	})
}

type SubmitAnswersRequest struct {
	Answers []struct {
		QuestionID    string `json:"question_id" validate:"required"`
		SelectedAnswer string `json:"selected_answer" validate:"required"`
	} `json:"answers" validate:"required,min=1"`
}

func SubmitTestAttempt(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	studentID, _ := uuid.Parse(claims["user_id"].(string))
	attemptID := c.Params("attemptId")

	var req SubmitAnswersRequest
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	var attempt models.TestAttempt
	if err := database.DB.Preload("MockTest.Questions").First(&attempt, "id = ? AND student_id = ?", attemptID, studentID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Test attempt not found"})
	}
	
	if attempt.EndTime != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Test has already been submitted"})
	}
	
	correctCount := 0
	var attemptAnswers []models.AttemptAnswer

	correctAnswersMap := make(map[uuid.UUID]string)
	for _, q := range attempt.MockTest.Questions {
		correctAnswersMap[q.ID] = q.CorrectAnswer
	}

	for _, answer := range req.Answers {
		questionID, _ := uuid.Parse(answer.QuestionID)
		isCorrect := correctAnswersMap[questionID] == answer.SelectedAnswer
		if isCorrect {
			correctCount++
		}
		attemptAnswers = append(attemptAnswers, models.AttemptAnswer{
			TestAttemptID:  attempt.ID,
			QuestionID:     questionID,
			SelectedAnswer: answer.SelectedAnswer,
			IsCorrect:      isCorrect,
		})
	}
	
	score := (float64(correctCount) / float64(len(attempt.MockTest.Questions))) * 100
	now := time.Now()

	attempt.EndTime = &now
	attempt.Score = &score
	
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&attempt).Error; err != nil { return err }
		if err := tx.Create(&attemptAnswers).Error; err != nil { return err }
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save results"})
	}

	return c.JSON(fiber.Map{
		"message": "Test submitted successfully",
		"score":   score,
		"results": attemptAnswers,
	})
}