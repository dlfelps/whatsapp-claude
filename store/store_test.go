// Package store — store_test.go contains unit tests for the in-memory store,
// including concurrent access testing.
//
// LEARNING: This file demonstrates several important Go testing patterns:
//
//   - Testing CRUD operations (Create, Read, Update, Delete)
//   - Verifying error conditions (duplicate keys, not found)
//   - Concurrent access testing with sync.WaitGroup and goroutines
//   - Testing with the -race flag: go test -race ./store/
//
// The -race flag enables Go's built-in race detector, which instruments
// memory accesses at compile time and reports data races at runtime.
// It's essential for testing concurrent code — always run "go test -race"
// before shipping concurrent code.
package store

import (
	"sync"
	"testing"
	"time"

	"whatsapp/model"

	"github.com/gorilla/websocket"
)

// TestNewStore verifies that the constructor returns a non-nil store.
func TestNewStore(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
}

// TestUserCRUD tests the complete user lifecycle: create, read, and
// error handling for duplicates and missing users.
//
// LEARNING: Testing all CRUD operations in one test function is acceptable
// when the operations are closely related and the test is readable. Each
// section tests a different aspect: success, duplicate error, not found error.
//
// t.Fatal vs t.Error:
//   - t.Fatal/t.Fatalf: Stops the test immediately (use when failure makes
//     subsequent checks meaningless or would panic)
//   - t.Error/t.Errorf: Records the failure but continues (use for independent
//     checks where other assertions are still valid)
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

// TestGetOrCreateUser tests the upsert behavior — creating new users and
// returning existing ones.
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

// TestChatCRUD tests chat creation, retrieval, and update operations.
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

// TestMessageCRUD tests message creation and retrieval.
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

// TestClientRegistration tests client registration, replacement, and
// unregistration — including the safety check that prevents stale goroutines
// from unregistering newer connections.
//
// LEARNING: &websocket.Conn{} creates a zero-valued WebSocket connection.
// This works for testing the store's registration logic without needing
// a real WebSocket server, because the store only cares about pointer
// identity and ClientID — it never calls methods on the connection.
// This is a lightweight alternative to mocking.
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

// TestAttachmentCRUD tests attachment creation and retrieval.
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

// TestInboxOperations tests the inbox lifecycle: add, get, remove, and clear.
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

// TestPurgeExpiredInboxEntries tests TTL-based purging of inbox entries.
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

// TestConcurrentAccess tests that the store handles concurrent goroutine
// access safely without data races.
//
// LEARNING: sync.WaitGroup is Go's mechanism for waiting on multiple
// goroutines to finish. The pattern is:
//
//	var wg sync.WaitGroup
//	for i := 0; i < n; i++ {
//	    wg.Add(1)           // Increment counter before launching goroutine
//	    go func() {
//	        defer wg.Done() // Decrement counter when goroutine finishes
//	        // ... do work
//	    }()
//	}
//	wg.Wait()               // Block until counter reaches zero
//
// wg.Add(1) MUST be called before "go func()" — if called inside the
// goroutine, there's a race between the goroutine starting and Wait()
// being called.
//
// Run this test with the race detector: go test -race ./store/
// The race detector will catch any unprotected concurrent map access.
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
