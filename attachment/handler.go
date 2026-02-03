package attachment

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Handler handles attachment HTTP requests
type Handler struct {
	service *Service
}

// NewHandler creates a new attachment handler
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// UploadResponse is the response for attachment upload
type UploadResponse struct {
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
}

// ErrorResponse is the response for errors
type ErrorResponse struct {
	Error string `json:"error"`
}

// HandleUpload handles POST /attachments
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

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UploadResponse{
		ID:          att.ID,
		ContentType: att.ContentType,
		Filename:    att.Filename,
		Size:        att.Size,
	})
}

// HandleDownload handles GET /attachments/{id}
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

// ServeHTTP routes requests to appropriate handlers
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

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}
