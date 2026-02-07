// Package chat — service_test.go contains unit tests for the chat service.
//
// LEARNING: Go has built-in testing support via the "testing" package. Key rules:
//   - Test files must end with _test.go (e.g., service_test.go)
//   - Test functions must start with "Test" and accept *testing.T
//   - Test files live alongside the code they test (same package)
//   - Run tests with: go test ./chat/ (or go test ./... for all packages)
//
// This file demonstrates:
//   - Basic unit test structure (arrange → act → assert)
//   - Testing both happy paths and error cases
//   - Sentinel error comparison for validating error conditions
//   - t.Fatalf vs t.Errorf (stop vs continue on failure)
//   - Testing idempotent operations
package chat

import (
	"testing"

	"whatsapp/model"
	"whatsapp/store"
)

// TestCreateChat tests the basic chat creation flow.
//
// LEARNING: The standard test structure in Go follows "arrange, act, assert":
//  1. Arrange: Set up dependencies (store, service)
//  2. Act: Call the function under test
//  3. Assert: Check the results with t.Fatalf/t.Errorf
//
// t.Fatalf stops the test immediately — use it when a failure makes subsequent
// checks meaningless (e.g., if CreateChat returns nil, checking chat.Name panics).
// t.Errorf logs the failure but continues — use it for non-fatal checks.
func TestCreateChat(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create a chat
	chat, err := svc.CreateChat("creator1", "Test Chat", []string{"user1", "user2"})
	if err != nil {
		t.Fatalf("CreateChat failed: %v", err)
	}

	if chat.Name != "Test Chat" {
		t.Errorf("Expected name 'Test Chat', got '%s'", chat.Name)
	}

	// Creator should be included in participants
	found := false
	for _, p := range chat.Participants {
		if p == "creator1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Creator should be in participants list")
	}
}

// TestCreateChat_MaxParticipants tests the participant limit enforcement.
//
// LEARNING: Test naming convention in Go uses underscores to describe the
// scenario: TestFunctionName_Scenario. This makes test output readable:
//   --- FAIL: TestCreateChat_MaxParticipants
//
// Testing boundary conditions (max, min, zero, empty) is crucial. Here we
// test that exceeding MaxParticipants returns the correct sentinel error.
func TestCreateChat_MaxParticipants(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create participants exceeding limit
	participants := make([]string, MaxParticipants+1)
	for i := range participants {
		participants[i] = "user" + string(rune('a'+i%26))
	}

	_, err := svc.CreateChat("creator1", "Too Big Chat", participants)
	if err != ErrMaxParticipants {
		t.Errorf("Expected ErrMaxParticipants, got %v", err)
	}
}

// TestSendMessage tests basic message sending.
//
// LEARNING: Tests often need to set up prerequisite state. Here we create
// a chat before sending a message to it. The underscore _ discards the
// error return from CreateChat because we trust it works (it's tested
// separately). This keeps the test focused on the function being tested.
func TestSendMessage(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat first
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2"})

	// Send message
	msg, err := svc.SendMessage("user1", chat.ID, "Hello", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if msg.Body != "Hello" {
		t.Errorf("Expected body 'Hello', got '%s'", msg.Body)
	}
	if msg.SenderID != "user1" {
		t.Errorf("Expected senderID 'user1', got '%s'", msg.SenderID)
	}
}

// TestSendMessage_NotParticipant tests that non-participants can't send messages.
//
// LEARNING: Testing error cases is just as important as testing happy paths.
// Here we verify that the service correctly rejects a message from a user
// who isn't in the chat. We compare the returned error against the sentinel
// error ErrNotParticipant directly with ==.
func TestSendMessage_NotParticipant(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2"})

	// Try to send message from non-participant
	_, err := svc.SendMessage("user3", chat.ID, "Hello", nil)
	if err != ErrNotParticipant {
		t.Errorf("Expected ErrNotParticipant, got %v", err)
	}
}

// TestSendMessage_ChatNotFound tests sending to a non-existent chat.
func TestSendMessage_ChatNotFound(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	_, err := svc.SendMessage("user1", "nonexistent", "Hello", nil)
	if err != ErrChatNotFound {
		t.Errorf("Expected ErrChatNotFound, got %v", err)
	}
}

// TestSendMessage_WithAttachments tests sending a message with file attachments.
func TestSendMessage_WithAttachments(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create attachment
	att := &model.Attachment{ID: "att1", Data: []byte("test")}
	s.CreateAttachment(att)

	// Create chat
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2"})

	// Send message with attachment
	msg, err := svc.SendMessage("user1", chat.ID, "Check this out", []string{"att1"})
	if err != nil {
		t.Fatalf("SendMessage with attachment failed: %v", err)
	}

	if len(msg.Attachments) != 1 || msg.Attachments[0] != "att1" {
		t.Error("Message should have attachment")
	}
}

// TestSendMessage_AttachmentNotFound tests that referencing a non-existent
// attachment is rejected.
func TestSendMessage_AttachmentNotFound(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2"})

	// Try to send with non-existent attachment
	_, err := svc.SendMessage("user1", chat.ID, "Hello", []string{"nonexistent"})
	if err != ErrAttachmentNotFound {
		t.Errorf("Expected ErrAttachmentNotFound, got %v", err)
	}
}

// TestModifyParticipants_Add tests adding participants to a chat.
func TestModifyParticipants_Add(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2"})

	// Add participant
	updated, err := svc.ModifyParticipants("user1", chat.ID, "add", []string{"user3"})
	if err != nil {
		t.Fatalf("ModifyParticipants failed: %v", err)
	}

	found := false
	for _, p := range updated.Participants {
		if p == "user3" {
			found = true
			break
		}
	}
	if !found {
		t.Error("user3 should be in participants after add")
	}
}

// TestModifyParticipants_Remove tests removing participants from a chat.
func TestModifyParticipants_Remove(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2", "user3"})

	// Remove participant
	updated, err := svc.ModifyParticipants("user1", chat.ID, "remove", []string{"user3"})
	if err != nil {
		t.Fatalf("ModifyParticipants failed: %v", err)
	}

	for _, p := range updated.Participants {
		if p == "user3" {
			t.Error("user3 should not be in participants after remove")
		}
	}
}

// TestModifyParticipants_CannotRemoveSelf tests that users can't remove
// themselves from a chat.
func TestModifyParticipants_CannotRemoveSelf(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2"})

	// Try to remove self
	_, err := svc.ModifyParticipants("user1", chat.ID, "remove", []string{"user1"})
	if err != ErrCannotRemoveSelf {
		t.Errorf("Expected ErrCannotRemoveSelf, got %v", err)
	}
}

// TestModifyParticipants_NotParticipant tests that non-participants can't
// modify the participant list.
func TestModifyParticipants_NotParticipant(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2"})

	// Try to modify as non-participant
	_, err := svc.ModifyParticipants("user3", chat.ID, "add", []string{"user4"})
	if err != ErrNotParticipant {
		t.Errorf("Expected ErrNotParticipant, got %v", err)
	}
}

// TestModifyParticipants_InvalidAction tests that invalid actions are rejected.
func TestModifyParticipants_InvalidAction(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2"})

	// Try invalid action
	_, err := svc.ModifyParticipants("user1", chat.ID, "invalid", []string{"user3"})
	if err != ErrInvalidAction {
		t.Errorf("Expected ErrInvalidAction, got %v", err)
	}
}

// TestModifyParticipants_ExceedsMax tests that adding participants beyond
// the maximum is rejected.
func TestModifyParticipants_ExceedsMax(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat with many participants
	participants := make([]string, MaxParticipants-2)
	for i := range participants {
		participants[i] = "user" + string(rune('a'+i%26)) + string(rune('0'+i/26))
	}
	chat, _ := svc.CreateChat("creator", "Big Chat", participants)

	// Try to add too many more
	newUsers := make([]string, 10)
	for i := range newUsers {
		newUsers[i] = "newuser" + string(rune('a'+i))
	}
	_, err := svc.ModifyParticipants("creator", chat.ID, "add", newUsers)
	if err != ErrMaxParticipants {
		t.Errorf("Expected ErrMaxParticipants, got %v", err)
	}
}

// TestAcknowledgeMessage tests message acknowledgment and idempotency.
//
// LEARNING: Testing idempotency means verifying that calling the same
// operation twice produces the same result without errors. This is critical
// for operations that may be retried due to network failures.
func TestAcknowledgeMessage(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Add message to inbox
	msg := &model.Message{ID: "msg1", ChatID: "chat1", Body: "Hello"}
	s.AddToInbox("user1", msg)

	// Acknowledge
	err := svc.AcknowledgeMessage("user1", "msg1")
	if err != nil {
		t.Fatalf("AcknowledgeMessage failed: %v", err)
	}

	// Verify removed from inbox
	entries := s.GetInbox("user1")
	if len(entries) != 0 {
		t.Error("Message should be removed from inbox after ack")
	}

	// Idempotent - should not error
	err = svc.AcknowledgeMessage("user1", "msg1")
	if err != nil {
		t.Error("Duplicate ack should not error")
	}
}

// TestGetChatParticipants tests retrieving the participant list for a chat.
func TestGetChatParticipants(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create chat
	chat, _ := svc.CreateChat("user1", "Test Chat", []string{"user2", "user3"})

	// Get participants
	participants, err := svc.GetChatParticipants(chat.ID)
	if err != nil {
		t.Fatalf("GetChatParticipants failed: %v", err)
	}

	if len(participants) != 3 { // user1, user2, user3
		t.Errorf("Expected 3 participants, got %d", len(participants))
	}
}
