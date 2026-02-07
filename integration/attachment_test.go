// Package integration — attachment_test.go contains end-to-end integration
// tests for the file attachment HTTP API.
//
// LEARNING: These tests demonstrate HTTP API testing in Go using:
//
//   - httptest.NewServer for running a real HTTP server
//   - mime/multipart for constructing file upload requests
//   - http.DefaultClient.Do for sending custom HTTP requests
//   - Testing HTTP status codes and response bodies
//   - Testing different content types and edge cases
//
// The mime/multipart package constructs the same multipart/form-data body
// that a browser sends when submitting a <form> with <input type="file">.
// Understanding multipart encoding is essential for building file upload APIs.
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"whatsapp/attachment"
	"whatsapp/store"
)

// AttachmentTestHelper provides a test server configured for attachment testing.
type AttachmentTestHelper struct {
	Store   *store.Store
	Service *attachment.Service
	Handler *attachment.Handler
	Server  *httptest.Server
}

// NewAttachmentTestHelper creates a fully wired test server for attachments.
func NewAttachmentTestHelper() *AttachmentTestHelper {
	s := store.NewStore()
	svc := attachment.NewService(s)
	h := attachment.NewHandler(svc)

	mux := http.NewServeMux()
	mux.Handle("/attachments", h)
	mux.Handle("/attachments/", h)

	server := httptest.NewServer(mux)

	return &AttachmentTestHelper{
		Store:   s,
		Service: svc,
		Handler: h,
		Server:  server,
	}
}

// Close shuts down the test server.
func (th *AttachmentTestHelper) Close() {
	th.Server.Close()
}

// TestAttachmentUploadDownload tests the complete upload-then-download cycle.
//
// LEARNING: Constructing a multipart file upload in Go:
//
//  1. Create a bytes.Buffer as the request body
//  2. Create a multipart.Writer wrapping the buffer
//  3. Call writer.CreateFormFile("fieldName", "filename") to add a file part
//  4. Write file content to the returned io.Writer
//  5. Close the writer (this writes the closing boundary)
//  6. Set the request Content-Type to writer.FormDataContentType()
//
// writer.FormDataContentType() returns something like:
//
//	"multipart/form-data; boundary=abc123..."
//
// The boundary is a unique string that separates parts in the multipart body.
// The multipart.Writer generates it automatically.
func TestAttachmentUploadDownload(t *testing.T) {
	th := NewAttachmentTestHelper()
	defer th.Close()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	testContent := "Hello, this is test content!"
	part.Write([]byte(testContent))
	writer.Close()

	// Upload
	req, _ := http.NewRequest("POST", th.Server.URL+"/attachments", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// LEARNING: http.DefaultClient.Do(req) sends a custom HTTP request.
	// It's more flexible than http.Get or http.Post because you can set
	// any method, headers, and body. Always close resp.Body when done.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(body))
	}

	// LEARNING: json.NewDecoder(resp.Body).Decode(&v) streams JSON from
	// the response body into a struct. This is the standard way to parse
	// JSON HTTP responses in Go.
	var uploadResp attachment.UploadResponse
	json.NewDecoder(resp.Body).Decode(&uploadResp)

	if uploadResp.ID == "" {
		t.Error("Upload response should have ID")
	}
	if uploadResp.Filename != "test.txt" {
		t.Errorf("Expected filename 'test.txt', got '%s'", uploadResp.Filename)
	}
	if uploadResp.Size != int64(len(testContent)) {
		t.Errorf("Expected size %d, got %d", len(testContent), uploadResp.Size)
	}

	// Download — http.Get is a convenience wrapper for simple GET requests
	downloadResp, err := http.Get(th.Server.URL + "/attachments/" + uploadResp.ID)
	if err != nil {
		t.Fatalf("Download request failed: %v", err)
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", downloadResp.StatusCode)
	}

	downloadedContent, _ := io.ReadAll(downloadResp.Body)
	if string(downloadedContent) != testContent {
		t.Errorf("Downloaded content mismatch: expected '%s', got '%s'", testContent, string(downloadedContent))
	}

	// Check headers
	contentDisp := downloadResp.Header.Get("Content-Disposition")
	if contentDisp == "" {
		t.Error("Content-Disposition header should be set")
	}
}

// TestAttachmentDownloadNotFound tests that downloading a non-existent
// attachment returns 404.
func TestAttachmentDownloadNotFound(t *testing.T) {
	th := NewAttachmentTestHelper()
	defer th.Close()

	resp, err := http.Get(th.Server.URL + "/attachments/nonexistent-id")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

// TestAttachmentUploadNoFile tests that uploading without a file returns 400.
func TestAttachmentUploadNoFile(t *testing.T) {
	th := NewAttachmentTestHelper()
	defer th.Close()

	// Send request without file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.Close()

	req, _ := http.NewRequest("POST", th.Server.URL+"/attachments", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

// TestAttachmentUploadDifferentContentTypes tests uploading files with
// various MIME types using a table-driven approach.
//
// LEARNING: This test uses "magic bytes" — the first few bytes of a file
// that identify its format. For example:
//   - JPEG files start with 0xFF 0xD8 0xFF
//   - PNG files start with 0x89 0x50 0x4E 0x47 (which is ".PNG" in ASCII)
//   - PDF files start with 0x25 0x50 0x44 0x46 (which is "%PDF" in ASCII)
//
// writer.CreatePart(headers) creates a multipart part with custom headers,
// giving us control over both Content-Type and Content-Disposition. This is
// more flexible than CreateFormFile, which uses default headers.
func TestAttachmentUploadDifferentContentTypes(t *testing.T) {
	th := NewAttachmentTestHelper()
	defer th.Close()

	testCases := []struct {
		filename    string
		contentType string
		content     []byte
	}{
		{"image.jpg", "image/jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}}, // JPEG magic bytes
		{"image.png", "image/png", []byte{0x89, 0x50, 0x4E, 0x47}},  // PNG magic bytes
		{"doc.pdf", "application/pdf", []byte{0x25, 0x50, 0x44, 0x46}}, // PDF magic bytes
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			// Create form file with headers
			h := make(map[string][]string)
			h["Content-Disposition"] = []string{`form-data; name="file"; filename="` + tc.filename + `"`}
			h["Content-Type"] = []string{tc.contentType}
			part, _ := writer.CreatePart(h)
			part.Write(tc.content)
			writer.Close()

			req, _ := http.NewRequest("POST", th.Server.URL+"/attachments", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Upload request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(body))
			}

			var uploadResp attachment.UploadResponse
			json.NewDecoder(resp.Body).Decode(&uploadResp)

			if uploadResp.Filename != tc.filename {
				t.Errorf("Expected filename '%s', got '%s'", tc.filename, uploadResp.Filename)
			}
		})
	}
}

// TestAttachmentMethodNotAllowed tests that wrong HTTP methods return 405.
//
// LEARNING: Testing HTTP method restrictions is important for API correctness.
// HTTP 405 Method Not Allowed means the endpoint exists but doesn't support
// the requested method. This is different from 404 Not Found (endpoint
// doesn't exist) and 400 Bad Request (request was malformed).
func TestAttachmentMethodNotAllowed(t *testing.T) {
	th := NewAttachmentTestHelper()
	defer th.Close()

	// Try PUT on upload endpoint
	req, _ := http.NewRequest("PUT", th.Server.URL+"/attachments", nil)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for PUT, got %d", resp.StatusCode)
	}

	// Try POST on download endpoint
	req, _ = http.NewRequest("POST", th.Server.URL+"/attachments/some-id", nil)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for POST on download, got %d", resp.StatusCode)
	}
}

// TestAttachmentUploadTooLarge tests the file size limit enforcement.
func TestAttachmentUploadTooLarge(t *testing.T) {
	th := NewAttachmentTestHelper()
	defer th.Close()

	// Create a file larger than 10MB limit
	// We can't easily do this in a unit test without memory issues,
	// but we can test the service layer directly
	largeData := make([]byte, attachment.MaxAttachmentSize+1)
	_, err := th.Service.Upload(largeData, "application/octet-stream", "large.bin")
	if err != attachment.ErrFileTooLarge {
		t.Errorf("Expected ErrFileTooLarge, got %v", err)
	}
}

// TestAttachmentGetOnUploadEndpoint tests that GET on the upload endpoint
// returns 405 Method Not Allowed.
func TestAttachmentGetOnUploadEndpoint(t *testing.T) {
	th := NewAttachmentTestHelper()
	defer th.Close()

	// GET on upload endpoint should return method not allowed
	resp, err := http.Get(th.Server.URL + "/attachments/")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// /attachments/ routes to upload handler, which only accepts POST
	if resp.StatusCode != http.StatusMethodNotAllowed {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 405 for GET on upload endpoint, got %d: %s", resp.StatusCode, string(body))
	}
}
