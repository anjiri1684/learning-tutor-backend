package handlers

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	configs "github.com/anjiri1684/language_tutor/configs"
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/websocket"        
	websocketcontrib "github.com/gofiber/contrib/websocket" 
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

    return c.JSON(user.Conversations)
}

func GetConversationMessages(c *fiber.Ctx) error {
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

    return c.JSON(messages)
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
func ServeWs(c *websocketcontrib.Conn) {
    var userID uuid.UUID

    type AuthMessage struct {
        Type  string `json:"type"`
        Token string `json:"token"`
    }
    var authMsg AuthMessage
    if err := c.ReadJSON(&authMsg); err != nil || authMsg.Type != "auth" {
        log.Printf("WebSocket auth failed: invalid or missing auth message, error: %v, received: %+v", err, authMsg)
        _ = c.WriteJSON(fiber.Map{"error": "Invalid or missing auth message"})
        c.Close()
        return
    }

    log.Printf("Received auth message with token: %s", authMsg.Token) // Debug token
    claims, err := parseToken(authMsg.Token)
    if err != nil {
        log.Printf("WebSocket auth failed: invalid token, error: %v", err)
        _ = c.WriteJSON(fiber.Map{"error": "Invalid token"})
        c.Close()
        return
    }

    userID, err = uuid.Parse(claims["user_id"].(string))
    if err != nil {
        log.Printf("WebSocket auth failed: invalid user_id, error: %v, user_id: %v", err, claims["user_id"])
        _ = c.WriteJSON(fiber.Map{"error": "Invalid user ID"})
        c.Close()
        return
    }

    log.Printf("WebSocket client authenticated and registered: %s", userID)
    client := &websocket.Client{UserID: userID, Conn: c}
    websocket.Register <- client
    defer func() {
        log.Printf("Unregistering client: %s", userID)
        websocket.Unregister <- client
        c.Close()
    }()

    for {
        var msg websocket.MessagePayload
        if err := c.ReadJSON(&msg); err != nil {
            if websocketcontrib.IsCloseError(err, websocketcontrib.CloseGoingAway, websocketcontrib.CloseAbnormalClosure) {
                log.Printf("WebSocket closed for client %s: %v", userID, err)
            } else {
                log.Printf("WebSocket read error for client %s: %v", userID, err)
            }
            break
        }

        convID, err := uuid.Parse(msg.ConversationID)
        if err != nil {
            log.Printf("Invalid conversation ID for client %s: %v", userID, err)
            _ = c.WriteJSON(fiber.Map{"error": "Invalid conversation ID"})
            continue
        }
        dbMessage := models.Message{
            ConversationID: convID,
            SenderID:       userID,
            Content:        msg.Content,
        }
        if err := database.DB.Create(&dbMessage).Error; err != nil {
            log.Printf("Failed to save message for client %s: %v", userID, err)
            _ = c.WriteJSON(fiber.Map{"error": "Failed to save message"})
            continue
        }
        log.Printf("Broadcasting message from client %s: %+v", userID, dbMessage)
        websocket.Broadcast <- &dbMessage
    }
}
func parseToken(tokenString string) (jwt.MapClaims, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(configs.Config("JWT_SECRET")), nil
    })
    if err != nil {
        return nil, err
    }
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        return claims, nil
    }
    return nil, errors.New("invalid token")
}