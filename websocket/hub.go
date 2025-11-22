package websocket

import (
    "log"
    "sync"

    "github.com/anjiri1684/language_tutor/database"
    "github.com/anjiri1684/language_tutor/models"
    "github.com/gofiber/contrib/websocket"
    "github.com/google/uuid"
)

type Client struct {
    UserID uuid.UUID
    Conn   *websocket.Conn
}

type MessagePayload struct {
    ConversationID string `json:"conversation_id"`
    Content        string `json:"content"`
}

var clients = make(map[uuid.UUID]*websocket.Conn)
var clientsMu sync.RWMutex
var Register = make(chan *Client)
var Unregister = make(chan *Client)
var Broadcast = make(chan *models.Message)

func init() {
    go RunHub()
}

func RunHub() {
    for {
        select {
        case client := <-Register:
            log.Printf("Client registered: %s", client.UserID)
            clientsMu.Lock()
            clients[client.UserID] = client.Conn
            clientsMu.Unlock()
        case client := <-Unregister:
            log.Printf("Client unregistered: %s", client.UserID)
            clientsMu.Lock()
            if conn, ok := clients[client.UserID]; ok && conn == client.Conn {
                delete(clients, client.UserID)
            }
            clientsMu.Unlock()
        case message := <-Broadcast:
            var participantIDs []uuid.UUID
            err := database.DB.
                Table("conversation_participants").
                Where("conversation_id = ?", message.ConversationID).
                Pluck("user_id", &participantIDs).Error
            if err != nil {
                log.Printf("Error fetching participant IDs for conversation %s: %v", message.ConversationID, err)
                continue
            }

            clientsMu.RLock()
            for _, participantID := range participantIDs {
                if participantID == message.SenderID {
                    continue
                }
                if conn, ok := clients[participantID]; ok {
                    if err := conn.WriteJSON(message); err != nil {
                        log.Printf("Error sending message to client %s: %v", participantID, err)
                        conn.Close()
                        clientsMu.RUnlock()
                        clientsMu.Lock()
                        delete(clients, participantID)
                        clientsMu.Unlock()
                        clientsMu.RLock()
                    }
                }
            }
            clientsMu.RUnlock()
        }
    }
}