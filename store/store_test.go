package store

import (
	"sync"
	"testing"
	"time"

	"whatsapp/model"

	"github.com/gorilla/websocket"
)

func TestNewStore(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
}

func TestUserCRUD(t *testing.T) {
	s := NewStore()

	// Create user
	user := &model.User{ID: "user1", Name: "Test User", Email: "test@example.com"}
	err := s.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Get user
	retrieved, err := s.GetUser("user1")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved.Name != "Test User" {
		t.Errorf("Expected name 'Test User', got '%s'", retrieved.Name)
	}

	// Duplicate user
	err = s.CreateUser(user)
	if err != ErrAlreadyExists {
		t.Errorf("Expected ErrAlreadyExists, got %v", err)
	}

	// Get non-existent user
	_, err = s.GetUser("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestGetOrCreateUser(t *testing.T) {
	s := NewStore()

	// Create new user
	user1 := s.GetOrCreateUser("user1")
	if user1 == nil {
		t.Fatal("GetOrCreateUser returned nil")
	}
	if user1.ID != "user1" {
		t.Errorf("Expected ID 'user1', got '%s'", user1.ID)
	}

	// Get existing user
	user2 := s.GetOrCreateUser("user1")
	if user2.ID != user1.ID {
		t.Error("GetOrCreateUser should return same user")
	}
}

func TestChatCRUD(t *testing.T) {
	s := NewStore()

	// Create chat
	chat := &model.Chat{
		ID:           "chat1",
		Name:         "Test Chat",
		Participants: []string{"user1", "user2"},
		CreatedAt:    time.Now(),
	}
	err := s.CreateChat(chat)
	if err != nil {
		t.Fatalf("CreateChat failed: %v", err)
	}

	// Get chat
	retrieved, err := s.GetChat("chat1")
	if err != nil {
		t.Fatalf("GetChat failed: %v", err)
	}
	if retrieved.Name != "Test Chat" {
		t.Errorf("Expected name 'Test Chat', got '%s'", retrieved.Name)
	}

	// Update chat
	chat.Name = "Updated Chat"
	err = s.UpdateChat(chat)
	if err != nil {
		t.Fatalf("UpdateChat failed: %v", err)
	}

	retrieved, _ = s.GetChat("chat1")
	if retrieved.Name != "Updated Chat" {
		t.Errorf("Expected name 'Updated Chat', got '%s'", retrieved.Name)
	}

	// Update non-existent chat
	err = s.UpdateChat(&model.Chat{ID: "nonexistent"})
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestMessageCRUD(t *testing.T) {
	s := NewStore()

	// Create message
	msg := &model.Message{
		ID:        "msg1",
		ChatID:    "chat1",
		SenderID:  "user1",
		Body:      "Hello",
		Timestamp: time.Now(),
	}
	err := s.CreateMessage(msg)
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	// Get message
	retrieved, err := s.GetMessage("msg1")
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if retrieved.Body != "Hello" {
		t.Errorf("Expected body 'Hello', got '%s'", retrieved.Body)
	}
}

func TestClientRegistration(t *testing.T) {
	s := NewStore()

	// Create mock connection (we can't use real websocket in unit tests)
	client1 := &model.Client{
		ClientID: "client1",
		UserID:   "user1",
		Conn:     &websocket.Conn{},
	}

	// Register client
	newClient, oldClient := s.RegisterClient("user1", client1)
	if newClient != client1 {
		t.Error("RegisterClient should return the new client")
	}
	if oldClient != nil {
		t.Error("First registration should not have old client")
	}

	// Get client
	retrieved, exists := s.GetClient("user1")
	if !exists {
		t.Error("GetClient should find registered client")
	}
	if retrieved.ClientID != "client1" {
		t.Errorf("Expected clientID 'client1', got '%s'", retrieved.ClientID)
	}

	// Replace client
	client2 := &model.Client{
		ClientID: "client2",
		UserID:   "user1",
		Conn:     &websocket.Conn{},
	}
	newClient, oldClient = s.RegisterClient("user1", client2)
	if newClient != client2 {
		t.Error("RegisterClient should return the new client")
	}
	if oldClient == nil || oldClient.ClientID != "client1" {
		t.Error("RegisterClient should return old client when replacing")
	}

	// Unregister wrong client (should fail)
	unregistered := s.UnregisterClient("user1", client1)
	if unregistered {
		t.Error("Unregister should fail for non-matching client")
	}

	// Unregister correct client
	unregistered = s.UnregisterClient("user1", client2)
	if !unregistered {
		t.Error("Unregister should succeed for matching client")
	}

	// Verify client is gone
	_, exists = s.GetClient("user1")
	if exists {
		t.Error("Client should be unregistered")
	}
}

func TestAttachmentCRUD(t *testing.T) {
	s := NewStore()

	// Create attachment
	att := &model.Attachment{
		ID:          "att1",
		Data:        []byte("test data"),
		ContentType: "text/plain",
		Filename:    "test.txt",
		Size:        9,
	}
	err := s.CreateAttachment(att)
	if err != nil {
		t.Fatalf("CreateAttachment failed: %v", err)
	}

	// Get attachment
	retrieved, err := s.GetAttachment("att1")
	if err != nil {
		t.Fatalf("GetAttachment failed: %v", err)
	}
	if string(retrieved.Data) != "test data" {
		t.Errorf("Expected data 'test data', got '%s'", string(retrieved.Data))
	}
}

func TestInboxOperations(t *testing.T) {
	s := NewStore()

	msg1 := &model.Message{ID: "msg1", ChatID: "chat1", Body: "Hello"}
	msg2 := &model.Message{ID: "msg2", ChatID: "chat1", Body: "World"}

	// Add to inbox
	s.AddToInbox("user1", msg1)
	s.AddToInbox("user1", msg2)

	// Get inbox
	entries := s.GetInbox("user1")
	if len(entries) != 2 {
		t.Errorf("Expected 2 inbox entries, got %d", len(entries))
	}

	// Remove by message ID
	removed := s.RemoveInboxEntryByMessageID("user1", "msg1")
	if !removed {
		t.Error("RemoveInboxEntryByMessageID should return true")
	}

	entries = s.GetInbox("user1")
	if len(entries) != 1 {
		t.Errorf("Expected 1 inbox entry after removal, got %d", len(entries))
	}

	// Idempotent removal
	removed = s.RemoveInboxEntryByMessageID("user1", "msg1")
	if removed {
		t.Error("Duplicate removal should return false")
	}

	// Clear inbox
	s.ClearInbox("user1")
	entries = s.GetInbox("user1")
	if len(entries) != 0 {
		t.Errorf("Expected empty inbox after clear, got %d", len(entries))
	}
}

func TestPurgeExpiredInboxEntries(t *testing.T) {
	s := NewStore()

	// Add messages
	msg := &model.Message{ID: "msg1", ChatID: "chat1", Body: "Hello"}
	s.AddToInbox("user1", msg)

	// Purge with long TTL (should not purge)
	purged := s.PurgeExpiredInboxEntries(24 * time.Hour)
	if purged != 0 {
		t.Errorf("Expected 0 purged, got %d", purged)
	}

	// Purge with zero TTL (should purge all)
	purged = s.PurgeExpiredInboxEntries(0)
	if purged != 1 {
		t.Errorf("Expected 1 purged, got %d", purged)
	}

	entries := s.GetInbox("user1")
	if len(entries) != 0 {
		t.Errorf("Expected empty inbox after purge, got %d", len(entries))
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewStore()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent user creation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.GetOrCreateUser("user1")
		}(i)
	}
	wg.Wait()

	// Concurrent inbox operations
	msg := &model.Message{ID: "msg1", ChatID: "chat1", Body: "Hello"}
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.AddToInbox("user1", msg)
		}()
	}
	wg.Wait()

	entries := s.GetInbox("user1")
	if len(entries) != numGoroutines {
		t.Errorf("Expected %d inbox entries, got %d", numGoroutines, len(entries))
	}
}
