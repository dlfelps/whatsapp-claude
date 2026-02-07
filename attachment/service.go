// Package attachment implements the file attachment service — upload and
// download of files associated with chat messages.
//
// LEARNING: This package follows the same service layer pattern as the chat
// package: a Service struct with injected dependencies, sentinel errors for
// error handling, and clean separation from the HTTP handler layer.
//
// This file demonstrates:
//   - Constant expressions for byte sizes (10 * 1024 * 1024 = 10MB)
//   - Map as a set (map[string]bool) for allowed values lookup
//   - Input validation at the service boundary
//   - Type conversion (int64(len(data))) for comparing different numeric types
package attachment

import (
	"errors"

	"whatsapp/model"
	"whatsapp/store"

	"github.com/google/uuid"
)

// LEARNING: Go's const expressions are evaluated at compile time and support
// arithmetic. Writing "10 * 1024 * 1024" is more readable than "10485760"
// and makes the intent clear: 10 megabytes. The compiler computes the final
// value, so there's no runtime cost.
const (
	// MaxAttachmentSize is the maximum size of an attachment (10MB)
	MaxAttachmentSize = 10 * 1024 * 1024
)

var (
	ErrFileTooLarge       = errors.New("file exceeds maximum size of 10MB")
	ErrAttachmentNotFound = errors.New("attachment not found")
	ErrInvalidContentType = errors.New("invalid content type")
)

// AllowedContentTypes defines the allowed MIME types for attachments.
//
// LEARNING: Using a map[string]bool as a set is a common Go pattern. To check
// membership, just do: if AllowedContentTypes[contentType] { ... }
// If the key exists, you get true; if not, you get the zero value (false).
// This gives O(1) lookup time, much better than scanning a slice.
//
// MIME types (also called "media types" or "content types") are standard
// identifiers for file formats, defined by IANA. The format is "type/subtype",
// e.g., "image/jpeg", "application/pdf". "application/octet-stream" is the
// generic fallback for unknown binary data.
var AllowedContentTypes = map[string]bool{
	"image/jpeg":               true,
	"image/png":                true,
	"image/gif":                true,
	"image/webp":               true,
	"video/mp4":                true,
	"video/webm":               true,
	"audio/mpeg":               true,
	"audio/ogg":                true,
	"audio/wav":                true,
	"application/pdf":          true,
	"text/plain":               true,
	"application/octet-stream": true, // Generic binary
}

// Service handles attachment business logic.
type Service struct {
	store *store.Store
}

// NewService creates a new attachment service.
func NewService(s *store.Store) *Service {
	return &Service{store: s}
}

// Upload uploads a new attachment after validating size and content type.
//
// LEARNING: int64(len(data)) is a type conversion. len() returns an int,
// but MaxAttachmentSize is compared as int64 (matching model.Attachment.Size).
// Go requires explicit type conversions — it never converts numeric types
// implicitly. This prevents subtle bugs from silent truncation or sign changes
// that plague C/C++ code.
//
// The default content type "application/octet-stream" is assigned when the
// client doesn't specify one. This follows the HTTP spec — unknown binary
// data should use the octet-stream MIME type.
func (s *Service) Upload(data []byte, contentType, filename string) (*model.Attachment, error) {
	// Validate size
	if int64(len(data)) > MaxAttachmentSize {
		return nil, ErrFileTooLarge
	}

	// Validate content type (allow if not in blocked list, for flexibility)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	att := &model.Attachment{
		ID:          uuid.New().String(),
		Data:        data,
		ContentType: contentType,
		Filename:    filename,
		Size:        int64(len(data)),
	}

	if err := s.store.CreateAttachment(att); err != nil {
		return nil, err
	}

	return att, nil
}

// Get retrieves an attachment by ID.
//
// LEARNING: This method wraps the store error with a domain-specific error
// (ErrAttachmentNotFound). This is called "error translation" — the handler
// layer doesn't need to know about store.ErrNotFound, only about attachment
// errors. This decouples the layers.
func (s *Service) Get(id string) (*model.Attachment, error) {
	att, err := s.store.GetAttachment(id)
	if err != nil {
		return nil, ErrAttachmentNotFound
	}
	return att, nil
}
