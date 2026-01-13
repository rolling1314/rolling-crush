package http

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/rolling1314/rolling-crush/sandbox"
	"github.com/rolling1314/rolling-crush/store/storage"
	"github.com/gin-gonic/gin"
)

// handleGetFiles handles getting file tree from sandbox
func (s *Server) handleGetFiles(c *gin.Context) {
	// Get request parameters
	targetPath := c.DefaultQuery("path", ".")
	sessionID := c.Query("session_id")

	slog.Info("handleGetFiles request", "session_id", sessionID, "path", targetPath, "query", c.Request.URL.RawQuery)

	// session_id is required
	if sessionID == "" {
		slog.Warn("Missing session_id parameter", "path", targetPath)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "session_id is required. Usage: /api/files?session_id=xxx&path=/sandbox/project"})
		return
	}

	slog.Info("Fetching file tree from sandbox", "session_id", sessionID, "path", targetPath)

	// Get file tree through sandbox client
	resp, err := s.sandboxClient.GetFileTree(c.Request.Context(), sandbox.FileTreeRequest{
		SessionID: sessionID,
		Path:      targetPath,
	})

	if err != nil {
		slog.Error("Failed to get file tree from sandbox", "error", err, "session_id", sessionID)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to get file tree: %v", err)})
		return
	}

	// Return file tree
	c.JSON(http.StatusOK, resp.Tree)
}

// ImageUploadResponse represents the response from an image upload.
type ImageUploadResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// handleUploadImage handles image upload to MinIO storage.
func (s *Server) handleUploadImage(c *gin.Context) {
	// Get the file from the request
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		slog.Error("Failed to get file from request", "error", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "No image file provided"})
		return
	}
	defer file.Close()

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		slog.Error("Failed to read file content", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to read file"})
		return
	}

	// Detect content type
	contentType := http.DetectContentType(data)
	
	// Also check the file extension for more accurate type detection
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	}

	// Validate image type
	if !storage.IsValidImageType(contentType) {
		slog.Warn("Invalid image type", "content_type", contentType, "filename", header.Filename)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("Invalid image type: %s. Supported types: jpeg, png, gif, webp", contentType),
		})
		return
	}

	// Get MinIO client
	minioClient := storage.GetMinIOClient()
	if minioClient == nil {
		slog.Error("MinIO client not initialized")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Storage service unavailable"})
		return
	}

	// Upload to MinIO
	result, err := minioClient.UploadFile(c.Request.Context(), header.Filename, data, contentType)
	if err != nil {
		slog.Error("Failed to upload file to MinIO", "error", err, "filename", header.Filename)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to upload image"})
		return
	}

	// Return the result
	c.JSON(http.StatusOK, ImageUploadResponse{
		URL:      result.URL,
		Filename: result.Filename,
		MimeType: result.MimeType,
		Size:     result.Size,
	})
}
