package chat

import (
	"testing"

	"whatsapp/model"
	"whatsapp/store"
)

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

func TestSendMessage_ChatNotFound(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	_, err := svc.SendMessage("user1", "nonexistent", "Hello", nil)
	if err != ErrChatNotFound {
		t.Errorf("Expected ErrChatNotFound, got %v", err)
	}
}

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
