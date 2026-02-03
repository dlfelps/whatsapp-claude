package attachment

import (
	"errors"

	"whatsapp/model"
	"whatsapp/store"

	"github.com/google/uuid"
)

const (
	// MaxAttachmentSize is the maximum size of an attachment (10MB)
	MaxAttachmentSize = 10 * 1024 * 1024
)

var (
	ErrFileTooLarge      = errors.New("file exceeds maximum size of 10MB")
	ErrAttachmentNotFound = errors.New("attachment not found")
	ErrInvalidContentType = errors.New("invalid content type")
)

// AllowedContentTypes defines the allowed MIME types for attachments
var AllowedContentTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"video/mp4":       true,
	"video/webm":      true,
	"audio/mpeg":      true,
	"audio/ogg":       true,
	"audio/wav":       true,
	"application/pdf": true,
	"text/plain":      true,
	"application/octet-stream": true, // Generic binary
}

// Service handles attachment business logic
type Service struct {
	store *store.Store
}

// NewService creates a new attachment service
func NewService(s *store.Store) *Service {
	return &Service{store: s}
}

// Upload uploads a new attachment
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

// Get retrieves an attachment by ID
func (s *Service) Get(id string) (*model.Attachment, error) {
	att, err := s.store.GetAttachment(id)
	if err != nil {
		return nil, ErrAttachmentNotFound
	}
	return att, nil
}
