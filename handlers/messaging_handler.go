package handlers

import (
	"strconv"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

func GetUserConversations(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))
	offset := (page - 1) * pageSize

	var user models.User
	if err := database.DB.
		Preload("Conversations.Participants").
		Where("id = ?", userID).
		Limit(pageSize).
		Offset(offset).
		First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	type ConversationWithUnread struct {
		models.Conversation
		UnreadCount int64 `json:"unread_count"`
	}

	result := make([]ConversationWithUnread, 0, len(user.Conversations))
	for _, convo := range user.Conversations {
		var unreadCount int64
		database.DB.Model(&models.Message{}).
			Where("conversation_id = ? AND sender_id != ? AND read_at IS NULL", convo.ID, userID).
			Count(&unreadCount)
		result = append(result, ConversationWithUnread{Conversation: *convo, UnreadCount: unreadCount})
	}

	return c.JSON(result)
}

func GetConversationMessages(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	conversationID := c.Params("conversationId")

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "50"))
	offset := (page - 1) * pageSize

	var messages []models.Message
	if err := database.DB.
		Where("conversation_id = ?", conversationID).
		Order("created_at asc").
		Limit(pageSize).
		Offset(offset).
		Find(&messages).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch messages"})
	}

	now := time.Now()
	database.DB.Model(&models.Message{}).
		Where("conversation_id = ? AND sender_id != ? AND read_at IS NULL", conversationID, userID).
		Update("read_at", now)

	return c.JSON(messages)
}

type SendMessageRequest struct {
	Content string `json:"content" validate:"required"`
}

func SendMessage(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID, _ := uuid.Parse(claims["user_id"].(string))
	conversationID, err := uuid.Parse(c.Params("conversationId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid conversation ID"})
	}

	var req SendMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var participantCount int64
	database.DB.Table("conversation_participants").
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Count(&participantCount)
	if participantCount == 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not a participant in this conversation"})
	}

	message := models.Message{
		ConversationID: conversationID,
		SenderID:       userID,
		Content:        req.Content,
	}
	if err := database.DB.Create(&message).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to send message"})
	}

	var sender models.User
	database.DB.First(&sender, "id = ?", userID)

	var recipientIDs []uuid.UUID
	database.DB.Table("conversation_participants").
		Where("conversation_id = ? AND user_id != ?", conversationID, userID).
		Pluck("user_id", &recipientIDs)

	for _, recipientID := range recipientIDs {
		go services.CreateNotification(recipientID, "message", "New message from "+sender.FullName, req.Content, "/messages")
	}

	return c.Status(fiber.StatusCreated).JSON(message)
}

func CreateOrGetConversation(c *fiber.Ctx) error {
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID1, _ := uuid.Parse(claims["user_id"].(string))

	type Request struct {
		RecipientID string `json:"recipient_id" validate:"required,uuid"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	userID2, _ := uuid.Parse(req.RecipientID)

	var conversation models.Conversation
	err := database.DB.
		Joins("JOIN conversation_participants cp1 ON cp1.conversation_id = conversations.id AND cp1.user_id = ?", userID1).
		Joins("JOIN conversation_participants cp2 ON cp2.conversation_id = conversations.id AND cp2.user_id = ?", userID2).
		First(&conversation).Error

	if err == nil {
		return c.JSON(conversation)
	}

	var user1, user2 models.User
	if err := database.DB.First(&user1, "id = ?", userID1).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}
	if err := database.DB.First(&user2, "id = ?", userID2).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Recipient not found"})
	}
	newConversation := models.Conversation{Participants: []*models.User{&user1, &user2}}
	if err := database.DB.Create(&newConversation).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create conversation"})
	}

	return c.Status(fiber.StatusCreated).JSON(newConversation)
}
