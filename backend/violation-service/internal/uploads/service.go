// Package uploads handles photo uploads for violations. See PHOTO_STORAGE.md.
//
// The upload pipeline:
//  1. OFFICER POSTs multipart/form-data with field `file` to
//     `/api/v1/uploads/violations`.
//  2. We validate: required, allowed MIME (jpg/jpeg/png/webp), max size
//     (STORAGE_PATH/MAX_UPLOAD_SIZE_MB).
//  3. We generate a UUID filename with the original extension and write
//     to `<STORAGE_PATH>/violations/<uuid>.<ext>`.
//  4. We return `{ file_name, photo_url }` so the officer can submit a
//     violation with that photo_url.
//
// The original filename is **never** used (security + collision avoidance).
package uploads

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxFileSize returns the max file size in bytes from the env-configured MB limit.
func MaxFileSize(maxMB int) int64 { return int64(maxMB) * 1024 * 1024 }

// Allowed MIME types (must match PHOTO_STORAGE.md).
var allowedMimes = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// UploadResult is the success response.
type UploadResult struct {
	FileName string `json:"file_name"`
	PhotoURL string `json:"photo_url"`
}

// Service handles file uploads. Stateless.
type Service struct {
	storagePath string
	publicBase  string // e.g. "/uploads"
	maxBytes    int64
}

// NewService constructs a Service.
//   - storagePath: local directory, e.g. "./storage" or "/app/storage" in Docker.
//   - publicBase: URL prefix the static file server uses, e.g. "/uploads".
//   - maxMB: max upload size in megabytes.
func NewService(storagePath, publicBase string, maxMB int) *Service {
	return &Service{
		storagePath: storagePath,
		publicBase:  strings.TrimRight(publicBase, "/"),
		maxBytes:    MaxFileSize(maxMB),
	}
}

// Save writes the uploaded file to disk and returns the URL the client can use.
// The caller (handler) is responsible for parsing the multipart form.
func (s *Service) Save(file *multipart.FileHeader) (UploadResult, error) {
	if file == nil {
		return UploadResult{}, errors.New("FILE_REQUIRED: no file")
	}
	if file.Size > s.maxBytes {
		return UploadResult{}, fmt.Errorf("FILE_TOO_LARGE: %d > %d", file.Size, s.maxBytes)
	}

	// Detect MIME by reading the first 512 bytes.
	src, err := file.Open()
	if err != nil {
		return UploadResult{}, fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()
	head := make([]byte, 512)
	n, _ := src.Read(head)
	mime := detectMime(head[:n])
	ext, ok := allowedMimes[mime]
	if !ok {
		return UploadResult{}, fmt.Errorf("INVALID_FILE_TYPE: %s", mime)
	}

	// Make sure the storage directory exists.
	dir := filepath.Join(s.storagePath, "violations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return UploadResult{}, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	// Generate UUID filename. NEVER use the original name.
	fname := uuid.NewString() + ext
	dstPath := filepath.Join(dir, fname)

	// Reset reader to the start (we already read 512 bytes for sniffing).
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return UploadResult{}, fmt.Errorf("seek: %w", err)
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return UploadResult{}, fmt.Errorf("create %s: %w", dstPath, err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return UploadResult{}, fmt.Errorf("copy: %w", err)
	}

	return UploadResult{
		FileName: fname,
		PhotoURL: s.publicBase + "/violations/" + fname,
	}, nil
}

// detectMime returns a MIME type from the leading bytes. Falls back to
// the file extension header if magic bytes don't match.
func detectMime(b []byte) string {
	if len(b) < 4 {
		return ""
	}
	// PNG: 89 50 4E 47
	if b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G' {
		return "image/png"
	}
	// JPEG: FF D8 FF
	if b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return "image/jpeg"
	}
	// WEBP: "RIFF" .... "WEBP"
	if len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return "image/webp"
	}
	// GIF: "GIF8"
	if len(b) >= 4 && string(b[0:4]) == "GIF8" {
		return "image/gif"
	}
	return ""
}

// PublicBase returns the public URL prefix (used by tests and main.go).
func (s *Service) PublicBase() string { return s.publicBase }

// StoragePath returns the local storage directory (used by main.go for sanity check).
func (s *Service) StoragePath() string { return s.storagePath }

// _ = time.Now keeps the import even if we add a timestamped filename later.
var _ = time.Now
