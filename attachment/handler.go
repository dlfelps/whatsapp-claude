// Package attachment — handler.go implements the HTTP handler for file
// uploads and downloads. It demonstrates how to build a REST-style API
// with Go's standard net/http package.
//
// LEARNING: This file shows several important Go HTTP patterns:
//   - Implementing the http.Handler interface (ServeHTTP method)
//   - Manual URL routing within a handler
//   - Multipart form parsing for file uploads
//   - JSON response encoding with proper HTTP status codes
//   - Request body size limiting for security
//   - Content-Type detection and Content-Disposition headers
package attachment

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Handler handles attachment HTTP requests.
type Handler struct {
	service *Service
}

// NewHandler creates a new attachment handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// UploadResponse is the JSON response returned after a successful upload.
type UploadResponse struct {
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
}

// ErrorResponse is the JSON response returned for errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

// HandleUpload handles POST /attachments — file upload via multipart form.
//
// LEARNING: Multipart form data is the standard way to upload files over HTTP.
// The client sends a request with Content-Type: multipart/form-data, which
// allows mixing binary file data with form fields in a single request.
//
// Key functions used:
//   - http.MaxBytesReader: Wraps the request body to enforce a size limit.
//     If the client sends more data than the limit, reads will fail with an error.
//     This prevents memory exhaustion from oversized uploads.
//
//   - r.ParseMultipartForm: Parses the multipart form data, buffering up to
//     maxMemory bytes in RAM and the rest in temporary files.
//
//   - r.FormFile: Returns the first file for the given form key ("file"),
//     along with a header containing the filename and content type.
//
//   - io.ReadAll: Reads the entire file into memory as a []byte. For large
//     files in production, you'd stream to disk instead.
//
//   - http.DetectContentType: Uses the file's magic bytes (first 512 bytes)
//     to detect the MIME type. This is a fallback when the client doesn't
//     specify a Content-Type.
func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, MaxAttachmentSize+1024) // Extra for headers

	// Parse multipart form
	if err := r.ParseMultipartForm(MaxAttachmentSize); err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Determine content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Upload attachment
	att, err := h.service.Upload(data, contentType, header.Filename)
	if err != nil {
		// LEARNING: This switch on error values demonstrates how sentinel
		// errors enable clean error-to-HTTP-status mapping. Each domain error
		// maps to a specific HTTP status code, giving clients meaningful feedback.
		switch err {
		case ErrFileTooLarge:
			h.writeError(w, http.StatusRequestEntityTooLarge, err.Error())
		case ErrInvalidContentType:
			h.writeError(w, http.StatusUnsupportedMediaType, err.Error())
		default:
			h.writeError(w, http.StatusInternalServerError, "upload failed")
		}
		return
	}

	// LEARNING: Setting headers MUST happen before WriteHeader/Write.
	// Go's http.ResponseWriter buffers headers until the first Write call
	// (or explicit WriteHeader). Once data is written, headers are sealed.
	//
	// http.StatusCreated (201) is the correct HTTP status for "resource created"
	// — more semantically accurate than 200 OK for POST operations that
	// create new resources.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UploadResponse{
		ID:          att.ID,
		ContentType: att.ContentType,
		Filename:    att.Filename,
		Size:        att.Size,
	})
}

// HandleDownload handles GET /attachments/{id} — file download by ID.
//
// LEARNING: strings.TrimPrefix is used for simple URL parameter extraction.
// Given path "/attachments/abc-123", TrimPrefix("/attachments/") yields "abc-123".
// This is a lightweight alternative to using a URL router library.
//
// For production APIs with complex routing needs, consider libraries like
// gorilla/mux or chi, which support parameterized routes like "/attachments/{id}".
//
// Content-Disposition: attachment tells the browser to download the file
// rather than display it inline. The filename parameter suggests a filename
// for the save dialog.
func (h *Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract ID from path
	path := r.URL.Path
	id := strings.TrimPrefix(path, "/attachments/")
	if id == "" || id == path {
		h.writeError(w, http.StatusBadRequest, "attachment ID is required")
		return
	}

	// Get attachment
	att, err := h.service.Get(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "attachment not found")
		return
	}

	// Set headers and return data
	w.Header().Set("Content-Type", att.ContentType)
	if att.Filename != "" {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+att.Filename+"\"")
	}
	w.WriteHeader(http.StatusOK)
	w.Write(att.Data)
}

// ServeHTTP routes requests to appropriate handlers.
//
// LEARNING: By implementing the ServeHTTP(http.ResponseWriter, *http.Request)
// method, Handler satisfies the http.Handler interface. This is Go's key
// HTTP abstraction — any type with a ServeHTTP method can be used with
// http.Handle(), middleware chains, and the standard HTTP server.
//
// This is an example of Go's implicit interface satisfaction: we never write
// "implements http.Handler" — the compiler checks it automatically when we
// pass Handler to http.Handle().
//
// The manual path matching here acts as a simple sub-router, dispatching
// /attachments to upload and /attachments/{id} to download.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if path == "/attachments" || path == "/attachments/" {
		h.HandleUpload(w, r)
		return
	}

	if strings.HasPrefix(path, "/attachments/") {
		h.HandleDownload(w, r)
		return
	}

	h.writeError(w, http.StatusNotFound, "not found")
}

// writeError writes a JSON error response with the given HTTP status code.
//
// LEARNING: json.NewEncoder(w).Encode(v) streams JSON directly to the
// http.ResponseWriter, which is more memory-efficient than json.Marshal
// (which creates an intermediate []byte). For HTTP responses, the streaming
// approach is preferred — the JSON is written directly to the network buffer.
func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}
