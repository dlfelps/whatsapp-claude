// Package chat implements the chat service layer — the business logic for
// creating chats, sending messages, and managing participants.
//
// LEARNING: Go projects commonly use a "service" layer (also called "use case"
// layer) that sits between the HTTP handlers and the data store. This layer
// contains business rules and validation, keeping handlers thin (just parse
// request, call service, format response) and stores simple (just CRUD).
//
// This file demonstrates:
//   - The service pattern with dependency injection
//   - Sentinel errors for domain-specific error handling
//   - Slice manipulation patterns (copy, filter, contains)
//   - Switch statements for action dispatch
//   - Private helper functions (unexported, lowercase names)
package chat

import (
	"errors"
	"time"

	"whatsapp/model"
	"whatsapp/store"

	"github.com/google/uuid"
)

// LEARNING: Constants in Go are defined with the "const" keyword. They must
// be computable at compile time (no function calls). Grouping related constants
// in a const() block is idiomatic. Exported constants (uppercase) are accessible
// from other packages.
const (
	// MaxParticipants is the maximum number of participants in a chat
	MaxParticipants = 100
)

// LEARNING: Package-level error variables (sentinel errors) define the error
// conditions this service can produce. Each is a distinct value that callers
// can compare against using == or errors.Is().
//
// Naming convention: errors start with "Err" (e.g., ErrNotFound, ErrChatNotFound).
// This is a strong Go convention — seeing "Err" prefix immediately tells you
// it's an error value.
//
// Grouping errors in a var() block is idiomatic. Using errors.New() creates
// a unique error value with the given message string.
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

// Service handles chat business logic. It depends on the store for data
// access and encapsulates all chat-related rules.
//
// LEARNING: The service struct has unexported fields (lowercase "store").
// In Go, lowercase names are package-private — they can only be accessed
// from within the same package. This enforces encapsulation: callers can't
// bypass the service and access the store directly.
type Service struct {
	store *store.Store
}

// NewService creates a new chat service.
//
// LEARNING: The constructor pattern NewXxx(*Dependency) *Xxx is the standard
// way to create initialized structs in Go. It enforces that all required
// dependencies are provided at creation time.
func NewService(s *store.Store) *Service {
	return &Service{store: s}
}

// CreateChat creates a new chat with the given participants. The creator
// is automatically added to the participant list if not already included.
//
// LEARNING: This function demonstrates several Go patterns:
//
//  1. make([]string, 0, len+1) — pre-allocates a slice with capacity for
//     the expected number of elements. The 0 is the initial length (empty),
//     and len+1 is the capacity (room for all participants plus the creator).
//
//  2. The &model.Chat{...} syntax creates a struct and returns a pointer to it.
//     This "composite literal" syntax is how Go initializes structs. The & takes
//     the address, giving us a *model.Chat pointer.
//
//  3. Multiple return values (*model.Chat, error) — the standard Go pattern
//     for functions that can fail. Callers must check the error before using
//     the result. This replaces exceptions in other languages.
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

// SendMessage sends a message to a chat. It validates that the sender is
// a participant and that all referenced attachments exist.
//
// LEARNING: Go's error handling philosophy is "check errors where they occur."
// Each operation that can fail returns an error, and we check it immediately.
// This explicit style replaces try/catch blocks and makes the error flow
// clear — you can trace every possible error path by reading top to bottom.
//
// The pattern: result, err := operation(); if err != nil { return ..., err }
// is the most common code pattern in Go. While it may seem verbose, it makes
// error handling explicit and impossible to accidentally skip.
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

// ModifyParticipants adds or removes participants from a chat.
//
// LEARNING: The switch statement in Go doesn't need "break" — each case
// automatically breaks. If you want fall-through behavior (rare), use the
// explicit "fallthrough" keyword. This default-break design prevents a
// common class of bugs found in C/Java.
//
// The default case handles invalid actions. Go's switch is more versatile
// than in most languages — it can switch on strings, compare expressions,
// and even work without a value (acting like if/else chains).
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

// addParticipants adds users to a chat's participant list.
//
// LEARNING: This is an unexported method (lowercase name). In Go, unexported
// names are only accessible within the same package. This is the primary
// encapsulation mechanism — there are no "private" or "protected" keywords.
//
// Notice copy(newParticipants, chat.Participants) — we work on a copy of
// the slice to avoid modifying the original until we're sure the operation
// succeeds. This prevents partial updates on error.
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

// removeParticipants removes users from a chat's participant list.
//
// LEARNING: The filter pattern here builds a new slice containing only
// elements that pass a condition. This is Go's approach to filtering —
// there's no built-in filter() function like in Python or JavaScript.
// Instead, you allocate a new slice and selectively append:
//
//	result := make([]T, 0, len(original))
//	for _, item := range original {
//	    if keepCondition(item) {
//	        result = append(result, item)
//	    }
//	}
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

// AcknowledgeMessage marks a message as acknowledged by a user.
// This is idempotent — calling it multiple times with the same arguments
// is safe and produces the same result.
//
// LEARNING: Idempotent operations are important in distributed systems
// because network retries can cause duplicate requests. If ack is called
// twice for the same message, the second call is a harmless no-op.
func (s *Service) AcknowledgeMessage(userID, messageID string) error {
	// Idempotent - just remove from inbox if exists
	s.store.RemoveInboxEntryByMessageID(userID, messageID)
	return nil
}

// GetChat retrieves a chat by ID. This thin wrapper delegates directly
// to the store — it exists to keep the handler layer decoupled from
// the store layer (handler -> service -> store).
func (s *Service) GetChat(chatID string) (*model.Chat, error) {
	return s.store.GetChat(chatID)
}

// GetChatParticipants returns the list of participant IDs for a chat.
func (s *Service) GetChatParticipants(chatID string) ([]string, error) {
	chat, err := s.store.GetChat(chatID)
	if err != nil {
		return nil, err
	}
	return chat.Participants, nil
}

// --- Helper functions ---

// LEARNING: Package-private helper functions (unexported, lowercase names)
// keep implementation details hidden. These functions are only used within
// the chat package and don't appear in the package's public API.
//
// In Go, there's no "contains" function for slices in the standard library
// (prior to Go 1.21's slices.Contains). Writing small helper functions like
// isInSlice is common and idiomatic. They're typically placed at the bottom
// of the file.

func isParticipant(chat *model.Chat, userID string) bool {
	return isInSlice(chat.Participants, userID)
}

// isInSlice checks if an item exists in a string slice.
// This is a linear search — O(n). For large slices, consider using a map
// for O(1) lookups instead.
func isInSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
