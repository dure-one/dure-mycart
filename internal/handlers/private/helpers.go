package handlers

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/shurco/mycart/pkg/fsutil"
)

const (
	dirUploads  = "./lc_uploads"
	dirDigitals = "./lc_digitals"
)

var validImageMIMETypes = []string{"image/png", "image/jpeg"}

// validImageExtensions is the allowlist of file extensions accepted for
// product images. Everything else is rejected regardless of declared MIME.
var validImageExtensions = []string{"png", "jpg", "jpeg"}

// blockedDigitalExtensions are extensions that must never be stored as
// digital products: they execute or render actively in browsers/scripts and
// would turn the store into a malware/XSS hosting platform if leaked.
var blockedDigitalExtensions = []string{
	"html", "htm", "svg", "js", "mjs", "php", "phtml", "asp", "aspx", "jsp",
	"exe", "dll", "bat", "cmd", "com", "scr", "msi", "ps1", "sh", "vbs", "jar",
}

// saveFile atomically saves the uploaded file to a temporary file, then renames it.
func saveFile(file *multipart.FileHeader, filePath string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	tmpPath := filePath + ".tmp"
	if err := os.MkdirAll(filepath.Dir(filePath), 0o775); err != nil {
		return err
	}

	dst, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	if err := dst.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, filePath)
}

func validateImageMIME(mimeType string) bool {
	return slices.Contains(validImageMIMETypes, mimeType)
}

// sniffMIMEType detects the real content type from the first bytes of the
// upload instead of trusting the client-declared Content-Type header.
func sniffMIMEType(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = src.Close() }()

	buf := make([]byte, 512)
	n, err := src.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	return http.DetectContentType(buf[:n]), nil
}

// normalizeExt returns the lowercased extension of an uploaded file name.
func normalizeExt(originalName string) string {
	return strings.ToLower(fsutil.ExtName(originalName))
}

// generateFileName generates a unique file name with extension.
func generateFileName(originalName string) (fileUUID, fileExt, fileName string) {
	fileUUID = uuid.New().String()
	fileExt = fsutil.ExtName(originalName)
	fileName = fmt.Sprintf("%s.%s", fileUUID, fileExt)
	return fileUUID, fileExt, fileName
}
