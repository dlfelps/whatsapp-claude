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

// Handler handles WebSocket connections
type Handler struct {
	store    *store.Store
	service  *Service
	upgrader websocket.Upgrader
}

// NewHandler creates a new WebSocket handler
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

// HandleWebSocket handles WebSocket upgrade and connection lifecycle
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
		// Close old connection gracefully
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

// handleConnection manages the connection lifecycle
func (h *Handler) handleConnection(client *model.Client) {
	defer func() {
		h.store.UnregisterClient(client.UserID, client)
		client.Conn.Close()
		log.Printf("Client disconnected: userID=%s, clientID=%s", client.UserID, client.ClientID)
	}()

	client.Conn.SetReadLimit(maxMessageSize)
	client.Conn.SetReadDeadline(time.Now().Add(pongWait))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start ping goroutine
	go h.pingLoop(client)

	// Read loop
	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		var cmd model.WSCommand
		if err := json.Unmarshal(message, &cmd); err != nil {
			h.sendError(client, "invalid command format")
			continue
		}

		h.dispatchCommand(client, &cmd)
	}
}

// pingLoop sends periodic pings to detect dead connections
func (h *Handler) pingLoop(client *model.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for range ticker.C {
		if err := client.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
	}
}

// dispatchCommand routes commands to appropriate handlers
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

// handleCreateChat handles chat creation
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

// handleSendMessage handles message sending
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

// handleAck handles message acknowledgment
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

// handleModifyParticipants handles participant modifications
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

// deliverMessage delivers a message to all chat participants
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

// deliverInbox delivers queued messages to a newly connected client
func (h *Handler) deliverInbox(client *model.Client) {
	entries := h.store.GetInbox(client.UserID)
	for _, entry := range entries {
		h.sendEvent(client, "newMessage", entry.Message)
	}
}

// broadcastToParticipants sends an event to all online participants
func (h *Handler) broadcastToParticipants(participants []string, eventType string, payload interface{}) {
	for _, userID := range participants {
		client, online := h.store.GetClient(userID)
		if online {
			h.sendEvent(client, eventType, payload)
		}
	}
}

// sendEvent sends an event to a client
func (h *Handler) sendEvent(client *model.Client, eventType string, payload interface{}) {
	event := model.WSEvent{
		Type:    eventType,
		Payload: payload,
	}
	if err := client.WriteJSON(event); err != nil {
		log.Printf("Error sending event to client %s: %v", client.ClientID, err)
	}
}

// sendError sends an error event to a client
func (h *Handler) sendError(client *model.Client, message string) {
	h.sendEvent(client, "error", model.ErrorPayload{Message: message})
}
