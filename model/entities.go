package model

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// User represents a user in the system
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Chat represents a chat room with participants
type Chat struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Participants []string  `json:"participants"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Message represents a message in a chat
type Message struct {
	ID          string    `json:"id"`
	ChatID      string    `json:"chatId"`
	SenderID    string    `json:"senderId"`
	Body        string    `json:"body"`
	Attachments []string  `json:"attachments,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Client represents a connected WebSocket client
type Client struct {
	ClientID string
	UserID   string
	Conn     *websocket.Conn
	WriteMu  sync.Mutex
}

// WriteJSON safely writes JSON to the WebSocket connection
func (c *Client) WriteJSON(v interface{}) error {
	c.WriteMu.Lock()
	defer c.WriteMu.Unlock()
	return c.Conn.WriteJSON(v)
}

// WriteMessage safely writes a message to the WebSocket connection
func (c *Client) WriteMessage(messageType int, data []byte) error {
	c.WriteMu.Lock()
	defer c.WriteMu.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

// Attachment represents a file attachment
type Attachment struct {
	ID          string `json:"id"`
	Data        []byte `json:"-"`
	ContentType string `json:"contentType"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
}

// InboxEntry represents a queued message for offline users
type InboxEntry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Message   *Message  `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

// WSCommand represents a command from client to server
type WSCommand struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// WSEvent represents an event from server to client
type WSEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Command payload types

// CreateChatPayload is the payload for createChat command
type CreateChatPayload struct {
	Name         string   `json:"name"`
	Participants []string `json:"participants"`
}

// SendMessagePayload is the payload for sendMessage command
type SendMessagePayload struct {
	ChatID      string   `json:"chatId"`
	Body        string   `json:"body"`
	Attachments []string `json:"attachments,omitempty"`
}

// AckPayload is the payload for ack command
type AckPayload struct {
	MessageID string `json:"messageId"`
}

// ModifyParticipantsPayload is the payload for modifyParticipants command
type ModifyParticipantsPayload struct {
	ChatID  string   `json:"chatId"`
	Action  string   `json:"action"` // "add" or "remove"
	UserIDs []string `json:"userIds"`
}

// Event payload types

// ConnectedPayload is the payload for connected event
type ConnectedPayload struct {
	UserID   string `json:"userId"`
	ClientID string `json:"clientId"`
}

// ErrorPayload is the payload for error event
type ErrorPayload struct {
	Message string `json:"message"`
}

// MessageAckedPayload is the payload for messageAcked event
type MessageAckedPayload struct {
	MessageID string `json:"messageId"`
	AckedBy   string `json:"ackedBy"`
}
