// Package attachment — service_test.go contains unit tests for the
// attachment service.
//
// LEARNING: This file demonstrates the "table-driven test" pattern, one of
// Go's most important testing idioms. Table-driven tests use a slice of
// test cases, each with inputs and expected outputs, then loop over them
// calling t.Run() for each case. Benefits:
//
//   - Easy to add new test cases (just add a struct to the slice)
//   - Each case gets its own subtest name in output
//   - DRY — shared test logic isn't duplicated
//   - Failing subtests are individually identifiable
//
// Run table-driven tests: go test -run TestUpload_VariousContentTypes
// Run a specific subtest: go test -run TestUpload_VariousContentTypes/image/jpeg
package attachment

import (
	"testing"

	"whatsapp/store"
)

// TestUpload tests basic file upload functionality.
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

// TestUpload_EmptyContentType tests that an empty content type defaults
// to "application/octet-stream".
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

// TestUpload_TooLarge tests that files exceeding the size limit are rejected.
//
// LEARNING: make([]byte, N) creates a byte slice of length N, filled with
// zero bytes. This is an efficient way to create test data of a specific
// size without caring about the content. The +1 ensures we're exactly one
// byte over the limit — testing boundary conditions precisely.
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

// TestUpload_ExactlyMaxSize tests that files exactly at the size limit succeed.
//
// LEARNING: Boundary testing is critical. This test verifies that the size
// check uses > (greater than) not >= (greater than or equal), allowing files
// at exactly the maximum size. Off-by-one errors are common bugs.
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

// TestGet tests retrieving an uploaded attachment.
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

// TestGet_NotFound tests retrieving a non-existent attachment.
func TestGet_NotFound(t *testing.T) {
	s := store.NewStore()
	svc := NewService(s)

	_, err := svc.Get("nonexistent")
	if err != ErrAttachmentNotFound {
		t.Errorf("Expected ErrAttachmentNotFound, got %v", err)
	}
}

// TestUpload_VariousContentTypes tests uploading files with different MIME types.
//
// LEARNING: This is a "table-driven test" — the most idiomatic Go testing
// pattern. The structure is:
//
//  1. Define a slice of anonymous structs, each representing a test case.
//     Each struct has fields for inputs and expected outputs.
//
//  2. Loop over the test cases with range.
//
//  3. Use t.Run(name, func(t *testing.T) {...}) to create a subtest for
//     each case. Subtests:
//     - Appear individually in test output: "TestUpload.../image/jpeg"
//     - Can be run individually: go test -run "VariousContentTypes/image"
//     - Have their own t, so t.Fatalf only stops that subtest
//     - Run sequentially by default, but can be parallelized with t.Parallel()
//
// The anonymous struct syntax `[]struct{ field1 Type1; field2 Type2 }{...}`
// defines the struct and its values inline. This avoids creating a named
// type that's only used in one place.
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
