// Package store provides a thread-safe, in-memory data store for the
// messaging server. It acts as the persistence layer, storing users, chats,
// messages, attachments, and offline inbox entries.
//
// LEARNING: This package demonstrates one of Go's most important concurrency
// patterns: protecting shared state with sync.RWMutex. In a web server,
// many goroutines (one per HTTP request or WebSocket connection) access the
// same data simultaneously. Without synchronization, concurrent reads and
// writes to maps cause data races — undefined behavior that can crash your
// program or corrupt data silently.
//
// Key concepts demonstrated:
//   - sync.RWMutex for reader/writer locking (multiple readers OR one writer)
//   - Sentinel error variables (ErrNotFound, ErrAlreadyExists)
//   - The map[string]*T pattern for key-value stores
//   - Defensive copying to prevent data races on returned slices
package store

import (
	"errors"
	"sync"
	"time"

	"whatsapp/model"

	"github.com/google/uuid"
)

// LEARNING: Package-level sentinel errors are a Go convention. By declaring
// errors as package variables, callers can compare error values directly:
//
//	_, err := store.GetUser("id")
//	if err == store.ErrNotFound { ... }
//
// This is preferred over string-based error matching because it's type-safe
// and won't break if the error message wording changes. Since Go 1.13, you
// can also use errors.Is() for wrapped errors: errors.Is(err, ErrNotFound).
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

// Store is a thread-safe in-memory data store.
//
// LEARNING: sync.RWMutex is a reader/writer mutex. It allows either:
//   - Many concurrent readers (RLock/RUnlock) — reads don't block each other
//   - One exclusive writer (Lock/Unlock) — writers block all readers and writers
//
// This is more efficient than sync.Mutex when reads far outnumber writes,
// which is typical for data stores. Each data collection has its own mutex
// to minimize contention — locking users doesn't block access to chats.
//
// Why separate mutexes per collection? If we used a single mutex, a write
// to the users map would block reads on chats, messages, and everything else.
// Fine-grained locking allows maximum concurrency.
type Store struct {
	users   map[string]*model.User
	usersMu sync.RWMutex

	chats   map[string]*model.Chat
	chatsMu sync.RWMutex

	messages   map[string]*model.Message
	messagesMu sync.RWMutex

	clients   map[string]*model.Client // keyed by userID
	clientsMu sync.RWMutex

	attachments   map[string]*model.Attachment
	attachmentsMu sync.RWMutex

	inbox   map[string][]*model.InboxEntry // keyed by userID
	inboxMu sync.RWMutex
}

// NewStore creates a new in-memory store with all maps initialized.
//
// LEARNING: In Go, maps must be initialized before use with make().
// A nil map can be read (returns zero values) but writing to a nil map panics.
// The constructor pattern (NewXxx) is idiomatic Go for initialization.
//
// Unlike languages with constructors (Java, Python), Go uses plain functions
// that return an initialized struct. The "New" prefix is a naming convention,
// not a language feature.
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

// --- User operations ---

// CreateUser creates a new user. Returns ErrAlreadyExists if the ID is taken.
//
// LEARNING: The Lock/defer Unlock pattern is the standard way to protect
// critical sections in Go. "defer" guarantees Unlock runs even if the code
// panics, preventing deadlocks from unhandled errors.
//
// We use Lock() (exclusive write lock) here because we're modifying the map.
// The check-then-set pattern (check if exists, then insert) must be atomic —
// if we released the lock between checking and inserting, another goroutine
// could insert the same key in between (a "time-of-check to time-of-use" bug).
func (s *Store) CreateUser(user *model.User) error {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()

	if _, exists := s.users[user.ID]; exists {
		return ErrAlreadyExists
	}
	s.users[user.ID] = user
	return nil
}

// GetUser retrieves a user by ID. Returns ErrNotFound if no user exists.
//
// LEARNING: RLock (read lock) allows multiple goroutines to read simultaneously.
// This is safe because reading a map doesn't modify it. Using RLock instead
// of Lock gives much better throughput when many goroutines are reading.
//
// The "comma ok" idiom (value, exists := map[key]) is Go's way to check
// if a key exists in a map. If the key is missing, "exists" is false and
// "value" is the zero value of the map's value type.
func (s *Store) GetUser(id string) (*model.User, error) {
	s.usersMu.RLock()
	defer s.usersMu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, ErrNotFound
	}
	return user, nil
}

// GetOrCreateUser gets an existing user or creates a new one.
//
// LEARNING: This is a "get or create" pattern (also called "upsert"). It uses
// Lock() (not RLock) because it may need to write. Even though it sometimes
// only reads, we need the write lock because we can't upgrade from RLock to
// Lock atomically — there's a gap between RUnlock and Lock where another
// goroutine could sneak in.
//
// The truncated ID as a name (id[:8]) is a simple way to generate human-readable
// names from UUIDs. This is just for prototyping — a real app would require
// proper user registration.
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

// --- Chat operations ---

// CreateChat creates a new chat. Returns ErrAlreadyExists if the ID is taken.
func (s *Store) CreateChat(chat *model.Chat) error {
	s.chatsMu.Lock()
	defer s.chatsMu.Unlock()

	if _, exists := s.chats[chat.ID]; exists {
		return ErrAlreadyExists
	}
	s.chats[chat.ID] = chat
	return nil
}

// GetChat retrieves a chat by ID.
func (s *Store) GetChat(id string) (*model.Chat, error) {
	s.chatsMu.RLock()
	defer s.chatsMu.RUnlock()

	chat, exists := s.chats[id]
	if !exists {
		return nil, ErrNotFound
	}
	return chat, nil
}

// UpdateChat updates a chat. Returns ErrNotFound if the chat doesn't exist.
//
// LEARNING: This function replaces the entire Chat value in the map.
// The check-then-replace must be atomic (inside one Lock) to prevent
// concurrent updates from overwriting each other.
func (s *Store) UpdateChat(chat *model.Chat) error {
	s.chatsMu.Lock()
	defer s.chatsMu.Unlock()

	if _, exists := s.chats[chat.ID]; !exists {
		return ErrNotFound
	}
	s.chats[chat.ID] = chat
	return nil
}

// --- Message operations ---

// CreateMessage creates a new message.
func (s *Store) CreateMessage(msg *model.Message) error {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

	if _, exists := s.messages[msg.ID]; exists {
		return ErrAlreadyExists
	}
	s.messages[msg.ID] = msg
	return nil
}

// GetMessage retrieves a message by ID.
func (s *Store) GetMessage(id string) (*model.Message, error) {
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()

	msg, exists := s.messages[id]
	if !exists {
		return nil, ErrNotFound
	}
	return msg, nil
}

// --- Client operations ---

// RegisterClient registers a new client, returning the new client and any
// old client that was replaced. This supports "connection replacement" — when
// a user reconnects, the new connection replaces the old one.
//
// LEARNING: Returning multiple values is idiomatic Go. This function returns
// (newClient, oldClient). If there was no previous client, oldClient is nil.
// Go's multiple return values eliminate the need for output parameters or
// complex result objects common in other languages.
//
// The nil check on the returned oldClient tells the caller whether a
// replacement happened, so they can close the old connection gracefully.
func (s *Store) RegisterClient(userID string, client *model.Client) (*model.Client, *model.Client) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	oldClient := s.clients[userID]
	s.clients[userID] = client
	return client, oldClient
}

// UnregisterClient removes a client if it matches the given client.
// Returns true if the client was removed. This prevents a stale goroutine
// from unregistering a newer connection that replaced it.
//
// LEARNING: The "compare then delete" pattern prevents race conditions
// during connection replacement. When user U reconnects:
//  1. New client C2 is registered, replacing old client C1
//  2. C1's goroutine detects the closed connection and calls UnregisterClient
//  3. Without the ClientID check, C1 would delete C2's registration!
//
// By comparing ClientIDs, only the current client can unregister itself.
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

// GetClient retrieves a client by user ID. Returns (client, true) if found,
// or (nil, false) if the user is offline.
//
// LEARNING: The (value, bool) return pattern is called the "comma ok" idiom
// in Go. It mirrors map lookups (val, ok := m[key]) and type assertions
// (val, ok := x.(Type)). This is more explicit than returning nil/null and
// checking — the boolean makes the "not found" case impossible to ignore.
func (s *Store) GetClient(userID string) (*model.Client, bool) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	client, exists := s.clients[userID]
	return client, exists
}

// GetAllClients returns all connected clients.
//
// LEARNING: make(slice, 0, len) pre-allocates capacity without setting length.
// This is an optimization: we know exactly how many elements we'll append,
// so we allocate once instead of growing the slice repeatedly (which copies
// the underlying array each time it grows).
//
// Pre-allocation is a common Go performance pattern for loops where the
// final size is known or can be estimated.
func (s *Store) GetAllClients() []*model.Client {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	clients := make([]*model.Client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	return clients
}

// --- Attachment operations ---

// CreateAttachment creates a new attachment.
func (s *Store) CreateAttachment(att *model.Attachment) error {
	s.attachmentsMu.Lock()
	defer s.attachmentsMu.Unlock()

	if _, exists := s.attachments[att.ID]; exists {
		return ErrAlreadyExists
	}
	s.attachments[att.ID] = att
	return nil
}

// GetAttachment retrieves an attachment by ID.
func (s *Store) GetAttachment(id string) (*model.Attachment, error) {
	s.attachmentsMu.RLock()
	defer s.attachmentsMu.RUnlock()

	att, exists := s.attachments[id]
	if !exists {
		return nil, ErrNotFound
	}
	return att, nil
}

// --- Inbox operations ---

// AddToInbox adds a message to a user's offline inbox.
//
// LEARNING: The append() built-in grows slices dynamically. When a slice's
// underlying array is full, append allocates a new, larger array and copies
// the elements. The growth factor is roughly 2x for small slices, decreasing
// for larger ones.
//
// s.inbox[userID] may be nil if the user has no inbox entries yet. append()
// handles nil slices gracefully — append(nil, elem) returns a new slice
// containing elem.
//
// LEARNING: github.com/google/uuid is a widely-used library for generating
// UUIDs (Universally Unique Identifiers). uuid.New() generates a random
// UUID v4, which is suitable for database primary keys because collisions
// are practically impossible (2^122 possible values).
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

// GetInbox retrieves all inbox entries for a user.
//
// LEARNING: This function returns a defensive copy of the slice. Without
// copying, the caller would hold a reference to the same underlying array
// as the store. If the store later modifies the slice (adding/removing entries),
// the caller's slice could see unexpected changes, or worse, concurrent
// modification without lock protection.
//
// copy(dst, src) copies min(len(dst), len(src)) elements. We create dst
// with the exact length needed, so it copies everything.
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

// RemoveInboxEntryByMessageID removes an inbox entry by message ID.
// Returns true if an entry was found and removed. This operation is
// idempotent — calling it again for the same message returns false harmlessly.
//
// LEARNING: The slice deletion pattern in Go uses append to concatenate
// the portions before and after the deleted element:
//
//	s = append(s[:i], s[i+1:]...)
//
// s[:i] is everything before index i, s[i+1:] is everything after.
// The ... unpacks the second slice into individual arguments for append.
// This modifies the original slice's backing array, which is fine here
// since we're under a write lock.
//
// Note: This is O(n) because it shifts elements. For frequent deletions
// from large slices, consider a linked list or map-based structure.
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

// PurgeExpiredInboxEntries removes inbox entries older than maxAge.
// Returns the number of entries purged. This is called periodically by
// the TTL purger to prevent unbounded growth of the inbox.
//
// LEARNING: time.Duration is Go's type for time intervals. It's a named
// type based on int64, representing nanoseconds. You create durations with
// time constants: 30 * 24 * time.Hour = 30 days. The time.Now().Add(-maxAge)
// pattern computes a cutoff timestamp: "maxAge ago from now".
//
// The algorithm builds a new "kept" slice for each user, filtering out
// expired entries. If all entries are expired, delete() removes the map
// key entirely to free memory. This is cleaner than repeatedly removing
// elements from the middle of a slice.
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

// ClearInbox clears all inbox entries for a user.
//
// LEARNING: delete() is a built-in function that removes a key from a map.
// It's a no-op if the key doesn't exist (won't panic). After deletion,
// the garbage collector will reclaim the memory used by the slice and its
// InboxEntry pointers, assuming no other references exist.
func (s *Store) ClearInbox(userID string) {
	s.inboxMu.Lock()
	defer s.inboxMu.Unlock()

	delete(s.inbox, userID)
}
