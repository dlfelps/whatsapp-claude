// Package chat — handler.go implements the WebSocket handler that manages
// real-time client connections, message routing, and the command/event protocol.
//
// LEARNING: This file demonstrates real-time WebSocket programming in Go using
// the gorilla/websocket library (github.com/gorilla/websocket), the most
// widely-used WebSocket library in the Go ecosystem. Key concepts:
//
//   - HTTP-to-WebSocket upgrade handshake
//   - Per-connection goroutines for concurrent I/O
//   - Ping/pong keep-alive for dead connection detection
//   - Command dispatch pattern (client sends commands, server sends events)
//   - Online/offline message routing with inbox queuing
package chat

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"whatsapp/model"
	"whatsapp/store"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// LEARNING: These unexported constants (lowercase) configure the WebSocket
// connection behavior. They're private to the chat package.
//
// The ping/pong mechanism detects dead connections (e.g., client crashed,
// network dropped). The server sends pings every pingPeriod; if the client
// doesn't respond with a pong within pongWait, the connection is considered
// dead. pingPeriod MUST be less than pongWait — otherwise we'd declare the
// connection dead before the client has time to respond.
const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 50 * time.Second
	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512KB
)

// Handler handles WebSocket connections.
//
// LEARNING: The websocket.Upgrader performs the HTTP-to-WebSocket protocol
// upgrade. When a client sends an HTTP request with "Upgrade: websocket"
// headers, the Upgrader responds with HTTP 101 Switching Protocols and
// converts the TCP connection to a WebSocket connection.
//
// CheckOrigin controls CORS for WebSocket connections. In production, you'd
// validate the Origin header against allowed domains. Returning true for
// all origins is only acceptable for development/prototyping.
type Handler struct {
	store    *store.Store
	service  *Service
	upgrader websocket.Upgrader
}

// NewHandler creates a new WebSocket handler.
func NewHandler(s *store.Store, svc *Service) *Handler {
	return &Handler{
		store:   s,
		service: svc,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for prototype
			},
		},
	}
}

// HandleWebSocket handles WebSocket upgrade and connection lifecycle.
// This is registered as an http.HandlerFunc for the "/ws" route.
//
// LEARNING: The connection lifecycle is:
//  1. Upgrade HTTP request to WebSocket (h.upgrader.Upgrade)
//  2. Assign a unique userID and clientID
//  3. Register the client (replacing any existing connection for the user)
//  4. Send a "connected" event with the assigned IDs
//  5. Deliver any queued inbox messages (offline messages)
//  6. Launch a goroutine to handle the read loop (handleConnection)
//
// Each WebSocket connection gets its own goroutine for reading. This is
// the standard Go concurrency model: one goroutine per connection.
// Go's scheduler efficiently handles thousands of goroutines.
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Generate user ID and client ID
	userID := uuid.New().String()
	clientID := uuid.New().String()

	client := &model.Client{
		ClientID: clientID,
		UserID:   userID,
		Conn:     conn,
	}

	// Register client (this may replace an existing connection)
	_, oldClient := h.store.RegisterClient(userID, client)
	if oldClient != nil {
		// LEARNING: When a user reconnects, we gracefully close the old
		// connection by sending a WebSocket Close frame before calling Close().
		// WriteControl sends a control frame (close, ping, or pong) with a
		// deadline. FormatCloseMessage creates the close frame payload with
		// a status code and human-readable reason.
		log.Printf("Replacing connection for user %s", userID)
		oldClient.Conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "replaced by new connection"),
			time.Now().Add(writeWait),
		)
		oldClient.Conn.Close()
	}

	// Ensure user exists
	h.store.GetOrCreateUser(userID)

	log.Printf("Client connected: userID=%s, clientID=%s", userID, clientID)

	// Send connected event
	h.sendEvent(client, "connected", model.ConnectedPayload{
		UserID:   userID,
		ClientID: clientID,
	})

	// Deliver any queued inbox messages
	h.deliverInbox(client)

	// Handle connection
	go h.handleConnection(client)
}

// handleConnection manages the connection lifecycle — the main read loop.
//
// LEARNING: The defer block at the top is a cleanup pattern. When this
// function returns (for any reason: error, normal close, panic), the
// deferred function runs in LIFO order:
//   1. Unregister the client from the store
//   2. Close the underlying WebSocket connection
//   3. Log the disconnection
//
// This ensures resources are always cleaned up, even on unexpected errors.
//
// The read loop pattern is:
//   for { message := conn.ReadMessage(); process(message) }
// This blocks on ReadMessage until data arrives or an error occurs.
// When the connection closes, ReadMessage returns an error and the loop exits.
func (h *Handler) handleConnection(client *model.Client) {
	defer func() {
		h.store.UnregisterClient(client.UserID, client)
		client.Conn.Close()
		log.Printf("Client disconnected: userID=%s, clientID=%s", client.UserID, client.ClientID)
	}()

	// LEARNING: SetReadLimit prevents a malicious client from sending
	// enormous messages that exhaust server memory. If a message exceeds
	// this limit, ReadMessage returns an error.
	client.Conn.SetReadLimit(maxMessageSize)

	// LEARNING: SetReadDeadline sets an absolute time after which reads
	// will fail. Combined with the pong handler below, this implements
	// connection health checking:
	//   - Set initial deadline to pongWait from now
	//   - Each time a pong arrives, reset the deadline
	//   - If no pong arrives within pongWait, ReadMessage returns a timeout error
	client.Conn.SetReadDeadline(time.Now().Add(pongWait))

	// LEARNING: SetPongHandler registers a callback for when pong frames arrive.
	// The WebSocket protocol defines ping/pong as a keep-alive mechanism.
	// The callback resets the read deadline, keeping the connection alive
	// as long as the client responds to pings.
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start ping goroutine
	go h.pingLoop(client)

	// Read loop — blocks on ReadMessage, processes commands sequentially
	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			// LEARNING: IsUnexpectedCloseError filters out "expected" close
			// scenarios (user navigated away, server shutting down) from real
			// errors. This prevents noisy logging for normal disconnections.
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		// LEARNING: json.Unmarshal parses raw JSON bytes into a Go struct.
		// It's the inverse of json.Marshal. The WSCommand struct uses
		// json.RawMessage for Payload, so only the "type" field is parsed
		// here — the payload is kept as raw bytes for later dispatching.
		var cmd model.WSCommand
		if err := json.Unmarshal(message, &cmd); err != nil {
			h.sendError(client, "invalid command format")
			continue
		}

		h.dispatchCommand(client, &cmd)
	}
}

// pingLoop sends periodic pings to detect dead connections.
//
// LEARNING: time.NewTicker sends a value on its channel (ticker.C) at
// regular intervals. "for range ticker.C" iterates over each tick — it
// blocks until the next tick arrives, then executes the loop body.
//
// If writing the ping fails (connection closed), the goroutine returns.
// The client.WriteMessage call uses the mutex-protected method on Client,
// ensuring ping writes don't interleave with data writes.
func (h *Handler) pingLoop(client *model.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for range ticker.C {
		if err := client.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
	}
}

// dispatchCommand routes commands to appropriate handlers based on type.
//
// LEARNING: This is the "command dispatch" pattern — a switch statement
// that routes different command types to different handler functions.
// Each handler receives the raw JSON payload and is responsible for
// unmarshaling it into the correct payload type.
//
// This is a simpler alternative to using interfaces or a command registry map.
// For a small number of command types, a switch is clear and efficient.
func (h *Handler) dispatchCommand(client *model.Client, cmd *model.WSCommand) {
	switch cmd.Type {
	case "createChat":
		h.handleCreateChat(client, cmd.Payload)
	case "sendMessage":
		h.handleSendMessage(client, cmd.Payload)
	case "ack":
		h.handleAck(client, cmd.Payload)
	case "modifyParticipants":
		h.handleModifyParticipants(client, cmd.Payload)
	default:
		h.sendError(client, "unknown command type: "+cmd.Type)
	}
}

// handleCreateChat handles the "createChat" command.
//
// LEARNING: Each command handler follows the same pattern:
//  1. Unmarshal the payload into the correct struct
//  2. Call the service layer method
//  3. On error, send an error event to the client
//  4. On success, broadcast the result to relevant participants
//
// This separation keeps handlers thin — they only do I/O marshaling.
// Business logic lives in the service layer.
func (h *Handler) handleCreateChat(client *model.Client, payload json.RawMessage) {
	var p model.CreateChatPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		h.sendError(client, "invalid createChat payload")
		return
	}

	chat, err := h.service.CreateChat(client.UserID, p.Name, p.Participants)
	if err != nil {
		h.sendError(client, err.Error())
		return
	}

	// Notify all participants about the new chat
	h.broadcastToParticipants(chat.Participants, "chatUpdate", chat)
}

// handleSendMessage handles the "sendMessage" command.
func (h *Handler) handleSendMessage(client *model.Client, payload json.RawMessage) {
	var p model.SendMessagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		h.sendError(client, "invalid sendMessage payload")
		return
	}

	msg, err := h.service.SendMessage(client.UserID, p.ChatID, p.Body, p.Attachments)
	if err != nil {
		h.sendError(client, err.Error())
		return
	}

	// Deliver message to participants
	h.deliverMessage(msg)
}

// handleAck handles the "ack" (acknowledge) command. When a user acknowledges
// a message, the message is removed from their inbox and the original sender
// is notified (if online).
func (h *Handler) handleAck(client *model.Client, payload json.RawMessage) {
	var p model.AckPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		h.sendError(client, "invalid ack payload")
		return
	}

	// Get the message to find the sender
	msg, err := h.store.GetMessage(p.MessageID)
	if err != nil {
		// Message not found - still remove from inbox (idempotent)
		h.service.AcknowledgeMessage(client.UserID, p.MessageID)
		return
	}

	if err := h.service.AcknowledgeMessage(client.UserID, p.MessageID); err != nil {
		h.sendError(client, err.Error())
		return
	}

	// Notify the sender that their message was acknowledged
	if senderClient, online := h.store.GetClient(msg.SenderID); online {
		h.sendEvent(senderClient, "messageAcked", model.MessageAckedPayload{
			MessageID: p.MessageID,
			AckedBy:   client.UserID,
		})
	}
}

// handleModifyParticipants handles the "modifyParticipants" command.
func (h *Handler) handleModifyParticipants(client *model.Client, payload json.RawMessage) {
	var p model.ModifyParticipantsPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		h.sendError(client, "invalid modifyParticipants payload")
		return
	}

	chat, err := h.service.ModifyParticipants(client.UserID, p.ChatID, p.Action, p.UserIDs)
	if err != nil {
		h.sendError(client, err.Error())
		return
	}

	// Notify all participants (including new ones if added) about the update
	h.broadcastToParticipants(chat.Participants, "chatUpdate", chat)
}

// deliverMessage delivers a message to all chat participants.
//
// LEARNING: This implements the "fan-out" pattern — one message is delivered
// to multiple recipients. For each participant:
//   - Online → send immediately via WebSocket
//   - Offline → queue to inbox for delivery when they reconnect
//
// The sender is excluded (you don't need to receive your own messages).
// This online/offline routing is a core pattern in messaging systems.
func (h *Handler) deliverMessage(msg *model.Message) {
	participants, err := h.service.GetChatParticipants(msg.ChatID)
	if err != nil {
		log.Printf("Error getting participants for message delivery: %v", err)
		return
	}

	for _, userID := range participants {
		if userID == msg.SenderID {
			// Don't deliver to sender
			continue
		}

		client, online := h.store.GetClient(userID)
		if online {
			// Deliver immediately
			h.sendEvent(client, "newMessage", msg)
		} else {
			// Queue for offline delivery
			h.store.AddToInbox(userID, msg)
		}
	}
}

// deliverInbox delivers queued messages to a newly connected client.
// This is called right after a client connects, before the read loop starts.
func (h *Handler) deliverInbox(client *model.Client) {
	entries := h.store.GetInbox(client.UserID)
	for _, entry := range entries {
		h.sendEvent(client, "newMessage", entry.Message)
	}
}

// broadcastToParticipants sends an event to all online participants.
// Offline participants simply don't receive the event — this is used
// for non-critical events like chat updates (not messages, which get queued).
func (h *Handler) broadcastToParticipants(participants []string, eventType string, payload interface{}) {
	for _, userID := range participants {
		client, online := h.store.GetClient(userID)
		if online {
			h.sendEvent(client, eventType, payload)
		}
	}
}

// sendEvent sends a typed event to a client via WebSocket.
//
// LEARNING: The WSEvent struct wraps every outgoing message with a "type"
// field, creating a uniform envelope for all server-to-client communication.
// This makes client-side routing simple: read the type, parse the payload.
//
// client.WriteJSON handles JSON marshaling and thread-safe writing
// (via the mutex defined in model.Client).
func (h *Handler) sendEvent(client *model.Client, eventType string, payload interface{}) {
	event := model.WSEvent{
		Type:    eventType,
		Payload: payload,
	}
	if err := client.WriteJSON(event); err != nil {
		log.Printf("Error sending event to client %s: %v", client.ClientID, err)
	}
}

// sendError sends an error event to a client. Errors are sent as regular
// events with type "error" — the client handles them like any other event.
func (h *Handler) sendError(client *model.Client, message string) {
	h.sendEvent(client, "error", model.ErrorPayload{Message: message})
}
