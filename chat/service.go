package chat

import (
	"errors"
	"time"

	"whatsapp/model"
	"whatsapp/store"

	"github.com/google/uuid"
)

const (
	// MaxParticipants is the maximum number of participants in a chat
	MaxParticipants = 100
)

var (
	ErrMaxParticipants    = errors.New("maximum participants exceeded")
	ErrNotParticipant     = errors.New("user is not a participant of this chat")
	ErrChatNotFound       = errors.New("chat not found")
	ErrInvalidAction      = errors.New("invalid action, must be 'add' or 'remove'")
	ErrCannotRemoveSelf   = errors.New("cannot remove yourself from chat")
	ErrUserAlreadyInChat  = errors.New("user is already a participant")
	ErrUserNotInChat      = errors.New("user is not in chat")
	ErrAttachmentNotFound = errors.New("attachment not found")
)

// Service handles chat business logic
type Service struct {
	store *store.Store
}

// NewService creates a new chat service
func NewService(s *store.Store) *Service {
	return &Service{store: s}
}

// CreateChat creates a new chat with the given participants
func (s *Service) CreateChat(creatorID, name string, participantIDs []string) (*model.Chat, error) {
	// Ensure creator is in participant list
	participants := make([]string, 0, len(participantIDs)+1)
	creatorIncluded := false
	for _, id := range participantIDs {
		if id == creatorID {
			creatorIncluded = true
		}
		participants = append(participants, id)
	}
	if !creatorIncluded {
		participants = append(participants, creatorID)
	}

	// Check max participants
	if len(participants) > MaxParticipants {
		return nil, ErrMaxParticipants
	}

	// Ensure all participants exist as users
	for _, id := range participants {
		s.store.GetOrCreateUser(id)
	}

	chat := &model.Chat{
		ID:           uuid.New().String(),
		Name:         name,
		Participants: participants,
		CreatedAt:    time.Now(),
	}

	if err := s.store.CreateChat(chat); err != nil {
		return nil, err
	}

	return chat, nil
}

// SendMessage sends a message to a chat
func (s *Service) SendMessage(senderID, chatID, body string, attachmentIDs []string) (*model.Message, error) {
	// Get the chat
	chat, err := s.store.GetChat(chatID)
	if err != nil {
		return nil, ErrChatNotFound
	}

	// Verify sender is a participant
	if !isParticipant(chat, senderID) {
		return nil, ErrNotParticipant
	}

	// Verify all attachments exist
	for _, attID := range attachmentIDs {
		if _, err := s.store.GetAttachment(attID); err != nil {
			return nil, ErrAttachmentNotFound
		}
	}

	msg := &model.Message{
		ID:          uuid.New().String(),
		ChatID:      chatID,
		SenderID:    senderID,
		Body:        body,
		Attachments: attachmentIDs,
		Timestamp:   time.Now(),
	}

	if err := s.store.CreateMessage(msg); err != nil {
		return nil, err
	}

	return msg, nil
}

// ModifyParticipants adds or removes participants from a chat
func (s *Service) ModifyParticipants(requesterID, chatID, action string, userIDs []string) (*model.Chat, error) {
	// Get the chat
	chat, err := s.store.GetChat(chatID)
	if err != nil {
		return nil, ErrChatNotFound
	}

	// Verify requester is a participant
	if !isParticipant(chat, requesterID) {
		return nil, ErrNotParticipant
	}

	switch action {
	case "add":
		return s.addParticipants(chat, userIDs)
	case "remove":
		return s.removeParticipants(chat, requesterID, userIDs)
	default:
		return nil, ErrInvalidAction
	}
}

func (s *Service) addParticipants(chat *model.Chat, userIDs []string) (*model.Chat, error) {
	newParticipants := make([]string, len(chat.Participants))
	copy(newParticipants, chat.Participants)

	for _, id := range userIDs {
		if isInSlice(newParticipants, id) {
			continue // Skip already added users
		}
		newParticipants = append(newParticipants, id)
		// Ensure user exists
		s.store.GetOrCreateUser(id)
	}

	if len(newParticipants) > MaxParticipants {
		return nil, ErrMaxParticipants
	}

	chat.Participants = newParticipants
	if err := s.store.UpdateChat(chat); err != nil {
		return nil, err
	}

	return chat, nil
}

func (s *Service) removeParticipants(chat *model.Chat, requesterID string, userIDs []string) (*model.Chat, error) {
	// Check if requester is trying to remove themselves
	for _, id := range userIDs {
		if id == requesterID {
			return nil, ErrCannotRemoveSelf
		}
	}

	newParticipants := make([]string, 0, len(chat.Participants))
	for _, p := range chat.Participants {
		if !isInSlice(userIDs, p) {
			newParticipants = append(newParticipants, p)
		}
	}

	chat.Participants = newParticipants
	if err := s.store.UpdateChat(chat); err != nil {
		return nil, err
	}

	return chat, nil
}

// AcknowledgeMessage marks a message as acknowledged by a user
func (s *Service) AcknowledgeMessage(userID, messageID string) error {
	// Idempotent - just remove from inbox if exists
	s.store.RemoveInboxEntryByMessageID(userID, messageID)
	return nil
}

// GetChat retrieves a chat by ID
func (s *Service) GetChat(chatID string) (*model.Chat, error) {
	return s.store.GetChat(chatID)
}

// GetChatParticipants returns the list of participant IDs for a chat
func (s *Service) GetChatParticipants(chatID string) ([]string, error) {
	chat, err := s.store.GetChat(chatID)
	if err != nil {
		return nil, err
	}
	return chat.Participants, nil
}

// Helper functions

func isParticipant(chat *model.Chat, userID string) bool {
	return isInSlice(chat.Participants, userID)
}

func isInSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
