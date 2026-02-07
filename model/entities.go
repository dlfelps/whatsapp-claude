// Package model defines the domain entities (data structures) used throughout
// the application. In Go, it's idiomatic to group related types in a dedicated
// package, keeping them free of business logic.
//
// LEARNING: Go doesn't have classes or inheritance. Instead, it uses structs
// to define data types and interfaces to define behavior contracts. This
// composition-over-inheritance approach is a core Go philosophy.
//
// This file demonstrates:
//   - Struct definitions with JSON tags for serialization
//   - sync.Mutex for thread-safe WebSocket writes
//   - Method receivers on structs (value vs pointer receivers)
//   - The encoding/json package and struct tag syntax
package model

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// User represents a user in the system.
//
// LEARNING: Struct tags (the backtick strings after field types) control how
// the struct is serialized/deserialized. The `json:"id"` tag tells
// encoding/json to use "id" as the JSON key instead of "ID".
//
// The `omitempty` option means the field is omitted from JSON output when
// it has its zero value (empty string for strings, 0 for numbers, nil for
// pointers/slices). This keeps API responses clean — no "email": "" clutter.
//
// Common tag format: `json:"fieldName,option1,option2"`
// Special values: `json:"-"` excludes a field from JSON entirely.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Chat represents a chat room with participants.
//
// LEARNING: time.Time is Go's standard type for timestamps. It includes
// timezone information and supports nanosecond precision. The encoding/json
// package marshals time.Time to RFC 3339 format ("2024-01-15T10:30:00Z")
// by default, which is the standard for JSON APIs.
type Chat struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Participants []string  `json:"participants"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Message represents a message in a chat.
//
// LEARNING: []string (slice of strings) is Go's dynamic array type. Unlike
// arrays, slices can grow and shrink. The `omitempty` tag on a slice means
// it's omitted from JSON when nil or empty (length 0). This is useful for
// optional fields like Attachments — most messages won't have any.
type Message struct {
	ID          string    `json:"id"`
	ChatID      string    `json:"chatId"`
	SenderID    string    `json:"senderId"`
	Body        string    `json:"body"`
	Attachments []string  `json:"attachments,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Client represents a connected WebSocket client.
//
// LEARNING: sync.Mutex is Go's mutual exclusion lock for protecting shared
// state from concurrent access. WebSocket connections are NOT safe for
// concurrent writes — if two goroutines write at the same time, the messages
// can interleave and corrupt the data stream. The WriteMu mutex ensures only
// one goroutine writes at a time.
//
// Note: Mutex fields should never be copied. That's why Client is typically
// passed around as a pointer (*Client), not by value.
//
// Notice there are no json tags here — Client is an internal type used only
// on the server side, never serialized to/from JSON.
type Client struct {
	ClientID string
	UserID   string
	Conn     *websocket.Conn
	WriteMu  sync.Mutex
}

// WriteJSON safely writes JSON to the WebSocket connection.
//
// LEARNING: This is a method with a pointer receiver (c *Client). The pointer
// receiver is required here because:
//   1. We need to mutate the mutex state (Lock/Unlock)
//   2. sync.Mutex must not be copied (copying a locked mutex leads to deadlocks)
//
// The Lock/Unlock pattern with "defer" is idiomatic Go:
//   - Lock() acquires the mutex (blocks if another goroutine holds it)
//   - defer Unlock() guarantees the mutex is released when the function returns,
//     even if Conn.WriteJSON panics. "defer" runs at function exit, always.
//
// This pattern is called a "mutex guard" — it's the Go equivalent of
// Java's synchronized block or C++'s std::lock_guard.
func (c *Client) WriteJSON(v interface{}) error {
	c.WriteMu.Lock()
	defer c.WriteMu.Unlock()
	return c.Conn.WriteJSON(v)
}

// WriteMessage safely writes a raw message to the WebSocket connection.
//
// LEARNING: The interface{} type (also written as "any" in Go 1.18+) is
// Go's empty interface — it can hold any value. WriteJSON above accepts
// interface{} so it can serialize any struct to JSON.
//
// WriteMessage takes a messageType int (e.g., websocket.PingMessage,
// websocket.TextMessage) and raw bytes. This lower-level method is used
// for control frames like pings that don't carry JSON data.
func (c *Client) WriteMessage(messageType int, data []byte) error {
	c.WriteMu.Lock()
	defer c.WriteMu.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

// Attachment represents a file attachment.
//
// LEARNING: The `json:"-"` tag on Data tells encoding/json to completely
// skip this field during marshaling and unmarshaling. Binary file data
// shouldn't appear in JSON API responses (it would be base64-encoded and
// bloat the response). Instead, attachments are downloaded via a separate
// HTTP endpoint that streams raw bytes.
//
// []byte is Go's type for raw binary data — a slice of bytes. It's used
// for file contents, network buffers, and any unstructured binary data.
type Attachment struct {
	ID          string `json:"id"`
	Data        []byte `json:"-"`
	ContentType string `json:"contentType"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
}

// InboxEntry represents a queued message for offline users.
// When a message is sent and the recipient isn't connected, the message
// is stored in their inbox and delivered when they reconnect.
//
// LEARNING: Using a pointer (*Message) here means InboxEntry doesn't own
// a copy of the message — it holds a reference to the same Message object.
// This is memory-efficient when the same message is queued for multiple
// offline recipients. Pointers are also required for optional fields that
// could be nil.
type InboxEntry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Message   *Message  `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

// WSCommand represents a command from client to server.
//
// LEARNING: json.RawMessage is a powerful type for handling polymorphic JSON.
// Instead of parsing the Payload into a specific struct immediately, we keep
// it as raw bytes. This lets us inspect the "type" field first, then unmarshal
// the Payload into the correct struct based on the command type.
//
// This is a common pattern for WebSocket protocols where a single connection
// carries multiple message types. Without RawMessage, you'd need an
// interface{} field and type assertions, which loses type safety.
//
// Flow: receive JSON -> unmarshal into WSCommand -> read Type ->
//       unmarshal Payload into CreateChatPayload/SendMessagePayload/etc.
type WSCommand struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// WSEvent represents an event from server to client.
//
// LEARNING: Compare WSEvent.Payload (interface{}) with WSCommand.Payload
// (json.RawMessage). The server creates events and knows the exact payload
// type at compile time, so interface{} works fine — encoding/json can
// serialize any concrete type behind an interface{}.
//
// For incoming data (WSCommand), we use json.RawMessage because we need to
// defer unmarshaling until we know the type. For outgoing data (WSEvent),
// we already know the type when constructing the event.
type WSEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// --- Command payload types ---
// LEARNING: Go uses separate struct types for each command/event payload
// rather than a single "catch-all" struct with optional fields. This gives
// compile-time type safety — you can't accidentally access a field that
// doesn't exist for a particular command type.

// CreateChatPayload is the payload for the createChat command.
type CreateChatPayload struct {
	Name         string   `json:"name"`
	Participants []string `json:"participants"`
}

// SendMessagePayload is the payload for the sendMessage command.
type SendMessagePayload struct {
	ChatID      string   `json:"chatId"`
	Body        string   `json:"body"`
	Attachments []string `json:"attachments,omitempty"`
}

// AckPayload is the payload for the ack (acknowledge) command.
type AckPayload struct {
	MessageID string `json:"messageId"`
}

// ModifyParticipantsPayload is the payload for the modifyParticipants command.
// Action must be "add" or "remove".
type ModifyParticipantsPayload struct {
	ChatID  string   `json:"chatId"`
	Action  string   `json:"action"` // "add" or "remove"
	UserIDs []string `json:"userIds"`
}

// --- Event payload types ---

// ConnectedPayload is the payload for the connected event, sent to clients
// immediately after they establish a WebSocket connection.
type ConnectedPayload struct {
	UserID   string `json:"userId"`
	ClientID string `json:"clientId"`
}

// ErrorPayload is the payload for error events sent to clients.
type ErrorPayload struct {
	Message string `json:"message"`
}

// MessageAckedPayload is the payload for the messageAcked event, sent to
// the original sender when a recipient acknowledges their message.
type MessageAckedPayload struct {
	MessageID string `json:"messageId"`
	AckedBy   string `json:"ackedBy"`
}
