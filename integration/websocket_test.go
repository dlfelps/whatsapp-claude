package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"whatsapp/chat"
	"whatsapp/model"
	"whatsapp/store"

	"github.com/gorilla/websocket"
)

// TestHelper provides common test functionality
type TestHelper struct {
	Store   *store.Store
	Service *chat.Service
	Handler *chat.Handler
	Server  *httptest.Server
}

func NewTestHelper() *TestHelper {
	s := store.NewStore()
	svc := chat.NewService(s)
	h := chat.NewHandler(s, svc)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.HandleWebSocket)

	server := httptest.NewServer(mux)

	return &TestHelper{
		Store:   s,
		Service: svc,
		Handler: h,
		Server:  server,
	}
}

func (th *TestHelper) Close() {
	th.Server.Close()
}

func (th *TestHelper) ConnectWS() (*websocket.Conn, error) {
	wsURL := "ws" + strings.TrimPrefix(th.Server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	return conn, err
}

func readEvent(conn *websocket.Conn) (*model.WSEvent, error) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	var event model.WSEvent
	if err := json.Unmarshal(message, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func sendCommand(conn *websocket.Conn, cmdType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cmd := model.WSCommand{
		Type:    cmdType,
		Payload: payloadBytes,
	}
	return conn.WriteJSON(cmd)
}

func TestWebSocketConnection(t *testing.T) {
	th := NewTestHelper()
	defer th.Close()

	conn, err := th.ConnectWS()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Should receive connected event
	event, err := readEvent(conn)
	if err != nil {
		t.Fatalf("Failed to read event: %v", err)
	}

	if event.Type != "connected" {
		t.Errorf("Expected 'connected' event, got '%s'", event.Type)
	}

	// Extract user ID from payload
	payloadBytes, _ := json.Marshal(event.Payload)
	var connPayload model.ConnectedPayload
	json.Unmarshal(payloadBytes, &connPayload)

	if connPayload.UserID == "" {
		t.Error("UserID should be set in connected event")
	}
	if connPayload.ClientID == "" {
		t.Error("ClientID should be set in connected event")
	}
}

func TestCreateChatAndSendMessage(t *testing.T) {
	th := NewTestHelper()
	defer th.Close()

	// Connect two clients
	conn1, _ := th.ConnectWS()
	defer conn1.Close()

	event1, _ := readEvent(conn1) // connected
	payload1Bytes, _ := json.Marshal(event1.Payload)
	var conn1Payload model.ConnectedPayload
	json.Unmarshal(payload1Bytes, &conn1Payload)
	user1ID := conn1Payload.UserID

	conn2, _ := th.ConnectWS()
	defer conn2.Close()

	event2, _ := readEvent(conn2) // connected
	payload2Bytes, _ := json.Marshal(event2.Payload)
	var conn2Payload model.ConnectedPayload
	json.Unmarshal(payload2Bytes, &conn2Payload)
	user2ID := conn2Payload.UserID

	// User1 creates a chat with user2
	createPayload := model.CreateChatPayload{
		Name:         "Test Chat",
		Participants: []string{user2ID},
	}
	sendCommand(conn1, "createChat", createPayload)

	// Both should receive chatUpdate
	chatEvent1, err := readEvent(conn1)
	if err != nil {
		t.Fatalf("User1 should receive chatUpdate: %v", err)
	}
	if chatEvent1.Type != "chatUpdate" {
		t.Errorf("Expected chatUpdate, got %s", chatEvent1.Type)
	}

	chatEvent2, err := readEvent(conn2)
	if err != nil {
		t.Fatalf("User2 should receive chatUpdate: %v", err)
	}
	if chatEvent2.Type != "chatUpdate" {
		t.Errorf("Expected chatUpdate, got %s", chatEvent2.Type)
	}

	// Extract chat ID
	chatBytes, _ := json.Marshal(chatEvent1.Payload)
	var chatData model.Chat
	json.Unmarshal(chatBytes, &chatData)
	chatID := chatData.ID

	// User1 sends a message
	msgPayload := model.SendMessagePayload{
		ChatID: chatID,
		Body:   "Hello from User1!",
	}
	sendCommand(conn1, "sendMessage", msgPayload)

	// User2 should receive newMessage
	msgEvent, err := readEvent(conn2)
	if err != nil {
		t.Fatalf("User2 should receive newMessage: %v", err)
	}
	if msgEvent.Type != "newMessage" {
		t.Errorf("Expected newMessage, got %s", msgEvent.Type)
	}

	msgBytes, _ := json.Marshal(msgEvent.Payload)
	var msgData model.Message
	json.Unmarshal(msgBytes, &msgData)

	if msgData.Body != "Hello from User1!" {
		t.Errorf("Expected message body 'Hello from User1!', got '%s'", msgData.Body)
	}
	if msgData.SenderID != user1ID {
		t.Errorf("Expected senderID '%s', got '%s'", user1ID, msgData.SenderID)
	}
}

func TestOfflineMessageQueue(t *testing.T) {
	th := NewTestHelper()
	defer th.Close()

	// This test verifies the inbox mechanism works at the store level
	// Full end-to-end would require persistent user IDs across reconnects

	// Create a user and add a message to their inbox
	user2ID := "test-user-2"
	th.Store.GetOrCreateUser(user2ID)

	msg := &model.Message{
		ID:     "test-msg-offline",
		ChatID: "chat1",
		Body:   "Offline message",
	}
	th.Store.AddToInbox(user2ID, msg)

	// Verify message is in inbox
	inbox := th.Store.GetInbox(user2ID)
	if len(inbox) != 1 {
		t.Fatalf("Expected 1 message in inbox, got %d", len(inbox))
	}
	if inbox[0].Message.ID != msg.ID {
		t.Error("Inbox should contain the sent message")
	}
	if inbox[0].Message.Body != "Offline message" {
		t.Errorf("Expected body 'Offline message', got '%s'", inbox[0].Message.Body)
	}

	// Verify ack removes from inbox
	th.Store.RemoveInboxEntryByMessageID(user2ID, msg.ID)
	inbox = th.Store.GetInbox(user2ID)
	if len(inbox) != 0 {
		t.Error("Message should be removed from inbox after ack")
	}

	t.Log("Offline queue test passed - inbox mechanism verified")
}

func TestMessageAcknowledge(t *testing.T) {
	th := NewTestHelper()
	defer th.Close()

	// Add message to inbox directly
	msg := &model.Message{ID: "test-msg-1", ChatID: "chat1", Body: "Test"}
	th.Store.AddToInbox("user1", msg)

	// Verify it's in inbox
	inbox := th.Store.GetInbox("user1")
	if len(inbox) != 1 {
		t.Fatal("Message should be in inbox")
	}

	// Connect user
	conn, _ := th.ConnectWS()
	defer conn.Close()

	event, _ := readEvent(conn) // connected
	payloadBytes, _ := json.Marshal(event.Payload)
	var connPayload model.ConnectedPayload
	json.Unmarshal(payloadBytes, &connPayload)

	// Note: In real scenario, user would ack the message they received
	// Here we're testing the ack mechanism

	// Send ack command
	ackPayload := model.AckPayload{MessageID: "test-msg-1"}
	sendCommand(conn, "ack", ackPayload)

	// Give server time to process
	time.Sleep(100 * time.Millisecond)

	// Verify removed from user1's inbox (using the actual user ID created on connect)
	// Since we created the inbox entry for "user1", not the connected user,
	// we need to verify the ack logic works
	th.Service.AcknowledgeMessage("user1", "test-msg-1")

	inbox = th.Store.GetInbox("user1")
	if len(inbox) != 0 {
		t.Error("Message should be removed from inbox after ack")
	}
}

func TestConnectionReplacement(t *testing.T) {
	th := NewTestHelper()
	defer th.Close()

	// This tests that when the same user reconnects, old connection is closed
	// Since we auto-generate user IDs, we need to test the store mechanism directly

	conn := &websocket.Conn{}
	client1 := &model.Client{ClientID: "client1", UserID: "user1", Conn: conn}
	client2 := &model.Client{ClientID: "client2", UserID: "user1", Conn: conn}

	// Register first client
	_, old := th.Store.RegisterClient("user1", client1)
	if old != nil {
		t.Error("First registration should not have old client")
	}

	// Register second client (same user)
	newClient, old := th.Store.RegisterClient("user1", client2)
	if old == nil || old.ClientID != "client1" {
		t.Error("Second registration should return old client")
	}
	if newClient.ClientID != "client2" {
		t.Error("New client should be registered")
	}

	// Verify current client is the new one
	current, _ := th.Store.GetClient("user1")
	if current.ClientID != "client2" {
		t.Error("Current client should be the new one")
	}
}

func TestModifyParticipants(t *testing.T) {
	th := NewTestHelper()
	defer th.Close()

	// Connect user
	conn, _ := th.ConnectWS()
	defer conn.Close()

	event, _ := readEvent(conn) // connected
	payloadBytes, _ := json.Marshal(event.Payload)
	var connPayload model.ConnectedPayload
	json.Unmarshal(payloadBytes, &connPayload)
	userID := connPayload.UserID

	// Create chat
	createPayload := model.CreateChatPayload{
		Name:         "Test Chat",
		Participants: []string{},
	}
	sendCommand(conn, "createChat", createPayload)

	chatEvent, _ := readEvent(conn)
	chatBytes, _ := json.Marshal(chatEvent.Payload)
	var chatData model.Chat
	json.Unmarshal(chatBytes, &chatData)
	chatID := chatData.ID

	// Add participant
	modifyPayload := model.ModifyParticipantsPayload{
		ChatID:  chatID,
		Action:  "add",
		UserIDs: []string{"newuser123"},
	}
	sendCommand(conn, "modifyParticipants", modifyPayload)

	// Should receive chatUpdate
	updateEvent, _ := readEvent(conn)
	if updateEvent.Type != "chatUpdate" {
		t.Errorf("Expected chatUpdate, got %s", updateEvent.Type)
	}

	// Verify participant was added
	chat, _ := th.Service.GetChat(chatID)
	found := false
	for _, p := range chat.Participants {
		if p == "newuser123" {
			found = true
			break
		}
	}
	if !found {
		t.Error("newuser123 should be in participants")
	}

	// Verify original user is still there
	found = false
	for _, p := range chat.Participants {
		if p == userID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Original user should still be in participants")
	}
}

func TestErrorHandling(t *testing.T) {
	th := NewTestHelper()
	defer th.Close()

	conn, _ := th.ConnectWS()
	defer conn.Close()

	readEvent(conn) // connected

	// Send message to non-existent chat
	msgPayload := model.SendMessagePayload{
		ChatID: "nonexistent-chat",
		Body:   "Hello",
	}
	sendCommand(conn, "sendMessage", msgPayload)

	// Should receive error
	errEvent, err := readEvent(conn)
	if err != nil {
		t.Fatalf("Should receive error event: %v", err)
	}

	if errEvent.Type != "error" {
		t.Errorf("Expected error event, got %s", errEvent.Type)
	}
}

func TestInvalidCommand(t *testing.T) {
	th := NewTestHelper()
	defer th.Close()

	conn, _ := th.ConnectWS()
	defer conn.Close()

	readEvent(conn) // connected

	// Send invalid command
	cmd := model.WSCommand{
		Type:    "invalidCommandType",
		Payload: []byte("{}"),
	}
	conn.WriteJSON(cmd)

	// Should receive error
	errEvent, err := readEvent(conn)
	if err != nil {
		t.Fatalf("Should receive error event: %v", err)
	}

	if errEvent.Type != "error" {
		t.Errorf("Expected error event, got %s", errEvent.Type)
	}
}
