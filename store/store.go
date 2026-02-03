package store

import (
	"errors"
	"sync"
	"time"

	"whatsapp/model"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

// Store is a thread-safe in-memory data store
type Store struct {
	users       map[string]*model.User
	usersMu     sync.RWMutex

	chats       map[string]*model.Chat
	chatsMu     sync.RWMutex

	messages    map[string]*model.Message
	messagesMu  sync.RWMutex

	clients     map[string]*model.Client // keyed by userID
	clientsMu   sync.RWMutex

	attachments    map[string]*model.Attachment
	attachmentsMu  sync.RWMutex

	inbox      map[string][]*model.InboxEntry // keyed by userID
	inboxMu    sync.RWMutex
}

// NewStore creates a new in-memory store
func NewStore() *Store {
	return &Store{
		users:       make(map[string]*model.User),
		chats:       make(map[string]*model.Chat),
		messages:    make(map[string]*model.Message),
		clients:     make(map[string]*model.Client),
		attachments: make(map[string]*model.Attachment),
		inbox:       make(map[string][]*model.InboxEntry),
	}
}

// User operations

// CreateUser creates a new user
func (s *Store) CreateUser(user *model.User) error {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()

	if _, exists := s.users[user.ID]; exists {
		return ErrAlreadyExists
	}
	s.users[user.ID] = user
	return nil
}

// GetUser retrieves a user by ID
func (s *Store) GetUser(id string) (*model.User, error) {
	s.usersMu.RLock()
	defer s.usersMu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, ErrNotFound
	}
	return user, nil
}

// GetOrCreateUser gets an existing user or creates a new one
func (s *Store) GetOrCreateUser(id string) *model.User {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()

	if user, exists := s.users[id]; exists {
		return user
	}
	name := id
	if len(id) > 8 {
		name = id[:8]
	}
	user := &model.User{
		ID:   id,
		Name: "User-" + name,
	}
	s.users[id] = user
	return user
}

// Chat operations

// CreateChat creates a new chat
func (s *Store) CreateChat(chat *model.Chat) error {
	s.chatsMu.Lock()
	defer s.chatsMu.Unlock()

	if _, exists := s.chats[chat.ID]; exists {
		return ErrAlreadyExists
	}
	s.chats[chat.ID] = chat
	return nil
}

// GetChat retrieves a chat by ID
func (s *Store) GetChat(id string) (*model.Chat, error) {
	s.chatsMu.RLock()
	defer s.chatsMu.RUnlock()

	chat, exists := s.chats[id]
	if !exists {
		return nil, ErrNotFound
	}
	return chat, nil
}

// UpdateChat updates a chat
func (s *Store) UpdateChat(chat *model.Chat) error {
	s.chatsMu.Lock()
	defer s.chatsMu.Unlock()

	if _, exists := s.chats[chat.ID]; !exists {
		return ErrNotFound
	}
	s.chats[chat.ID] = chat
	return nil
}

// Message operations

// CreateMessage creates a new message
func (s *Store) CreateMessage(msg *model.Message) error {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

	if _, exists := s.messages[msg.ID]; exists {
		return ErrAlreadyExists
	}
	s.messages[msg.ID] = msg
	return nil
}

// GetMessage retrieves a message by ID
func (s *Store) GetMessage(id string) (*model.Message, error) {
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()

	msg, exists := s.messages[id]
	if !exists {
		return nil, ErrNotFound
	}
	return msg, nil
}

// Client operations

// RegisterClient registers a new client, returning the new client and any old client that was replaced
func (s *Store) RegisterClient(userID string, client *model.Client) (*model.Client, *model.Client) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	oldClient := s.clients[userID]
	s.clients[userID] = client
	return client, oldClient
}

// UnregisterClient removes a client if it matches the given client
func (s *Store) UnregisterClient(userID string, client *model.Client) bool {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	current, exists := s.clients[userID]
	if !exists || current.ClientID != client.ClientID {
		return false
	}
	delete(s.clients, userID)
	return true
}

// GetClient retrieves a client by user ID
func (s *Store) GetClient(userID string) (*model.Client, bool) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	client, exists := s.clients[userID]
	return client, exists
}

// GetAllClients returns all connected clients
func (s *Store) GetAllClients() []*model.Client {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	clients := make([]*model.Client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	return clients
}

// Attachment operations

// CreateAttachment creates a new attachment
func (s *Store) CreateAttachment(att *model.Attachment) error {
	s.attachmentsMu.Lock()
	defer s.attachmentsMu.Unlock()

	if _, exists := s.attachments[att.ID]; exists {
		return ErrAlreadyExists
	}
	s.attachments[att.ID] = att
	return nil
}

// GetAttachment retrieves an attachment by ID
func (s *Store) GetAttachment(id string) (*model.Attachment, error) {
	s.attachmentsMu.RLock()
	defer s.attachmentsMu.RUnlock()

	att, exists := s.attachments[id]
	if !exists {
		return nil, ErrNotFound
	}
	return att, nil
}

// Inbox operations

// AddToInbox adds a message to a user's inbox
func (s *Store) AddToInbox(userID string, msg *model.Message) {
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()

	entry := &model.InboxEntry{
		ID:        uuid.New().String(),
		UserID:    userID,
		Message:   msg,
		CreatedAt: time.Now(),
	}
	s.inbox[userID] = append(s.inbox[userID], entry)
}

// GetInbox retrieves all inbox entries for a user
func (s *Store) GetInbox(userID string) []*model.InboxEntry {
	s.inboxMu.RLock()
	defer s.inboxMu.RUnlock()

	entries := s.inbox[userID]
	if entries == nil {
		return []*model.InboxEntry{}
	}
	// Return a copy to avoid race conditions
	result := make([]*model.InboxEntry, len(entries))
	copy(result, entries)
	return result
}

// RemoveInboxEntryByMessageID removes an inbox entry by message ID (idempotent)
func (s *Store) RemoveInboxEntryByMessageID(userID, messageID string) bool {
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()

	entries := s.inbox[userID]
	for i, entry := range entries {
		if entry.Message.ID == messageID {
			s.inbox[userID] = append(entries[:i], entries[i+1:]...)
			return true
		}
	}
	return false
}

// PurgeExpiredInboxEntries removes inbox entries older than maxAge
func (s *Store) PurgeExpiredInboxEntries(maxAge time.Duration) int {
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	purged := 0

	for userID, entries := range s.inbox {
		kept := make([]*model.InboxEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.CreatedAt.After(cutoff) {
				kept = append(kept, entry)
			} else {
				purged++
			}
		}
		if len(kept) == 0 {
			delete(s.inbox, userID)
		} else {
			s.inbox[userID] = kept
		}
	}
	return purged
}

// ClearInbox clears all inbox entries for a user
func (s *Store) ClearInbox(userID string) {
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()

	delete(s.inbox, userID)
}
