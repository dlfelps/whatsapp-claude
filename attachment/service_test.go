package attachment

import (
	"testing"

	"whatsapp/store"
)

func TestUpload(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	data := []byte("test file content")
	att, err := svc.Upload(data, "text/plain", "test.txt")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if att.ID == "" {
		t.Error("Attachment should have ID")
	}
	if att.ContentType != "text/plain" {
		t.Errorf("Expected content type 'text/plain', got '%s'", att.ContentType)
	}
	if att.Filename != "test.txt" {
		t.Errorf("Expected filename 'test.txt', got '%s'", att.Filename)
	}
	if att.Size != int64(len(data)) {
		t.Errorf("Expected size %d, got %d", len(data), att.Size)
	}
}

func TestUpload_EmptyContentType(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	data := []byte("test file content")
	att, err := svc.Upload(data, "", "test.txt")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if att.ContentType != "application/octet-stream" {
		t.Errorf("Expected default content type, got '%s'", att.ContentType)
	}
}

func TestUpload_TooLarge(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create data larger than max size
	data := make([]byte, MaxAttachmentSize+1)
	_, err := svc.Upload(data, "application/octet-stream", "large.bin")
	if err != ErrFileTooLarge {
		t.Errorf("Expected ErrFileTooLarge, got %v", err)
	}
}

func TestUpload_ExactlyMaxSize(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Create data exactly at max size - should succeed
	data := make([]byte, MaxAttachmentSize)
	att, err := svc.Upload(data, "application/octet-stream", "maxsize.bin")
	if err != nil {
		t.Fatalf("Upload at max size should succeed: %v", err)
	}

	if att.Size != MaxAttachmentSize {
		t.Errorf("Expected size %d, got %d", MaxAttachmentSize, att.Size)
	}
}

func TestGet(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	// Upload first
	data := []byte("test file content")
	uploaded, _ := svc.Upload(data, "text/plain", "test.txt")

	// Get it back
	retrieved, err := svc.Get(uploaded.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved.Data) != "test file content" {
		t.Errorf("Expected data 'test file content', got '%s'", string(retrieved.Data))
	}
}

func TestGet_NotFound(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	_, err := svc.Get("nonexistent")
	if err != ErrAttachmentNotFound {
		t.Errorf("Expected ErrAttachmentNotFound, got %v", err)
	}
}

func TestUpload_VariousContentTypes(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	testCases := []struct {
		contentType string
		filename    string
	}{
		{"image/jpeg", "photo.jpg"},
		{"image/png", "screenshot.png"},
		{"video/mp4", "video.mp4"},
		{"audio/mpeg", "song.mp3"},
		{"application/pdf", "document.pdf"},
	}

	for _, tc := range testCases {
		t.Run(tc.contentType, func(t *testing.T) {
			data := []byte("fake " + tc.contentType + " data")
			att, err := svc.Upload(data, tc.contentType, tc.filename)
			if err != nil {
				t.Fatalf("Upload failed for %s: %v", tc.contentType, err)
			}

			if att.ContentType != tc.contentType {
				t.Errorf("Expected content type '%s', got '%s'", tc.contentType, att.ContentType)
			}
		})
	}
}
