package messages

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-go/models/common"
)

// processAttachments reads the --attach files into SDK attachments.
func processAttachments(filePaths []string) ([]common.Attachment, error) {
	files, err := cmdutil.ReadAttachments(filePaths)
	if err != nil {
		return nil, err
	}
	var attachments []common.Attachment
	for _, file := range files {
		attachments = append(attachments, common.Attachment{
			FileName:    filepath.Base(file.Path),
			ContentType: detectMIMEType(file.Path, file.Data),
			Data:        base64.StdEncoding.EncodeToString(file.Data),
			Base64:      true,
		})
	}
	return attachments, nil
}

// maxRequestBytes is the largest request body the API takes, 30 MB: it
// refuses a larger one with HTTP 413.
const maxRequestBytes = 30_000_000

// requestMarginBytes is the room, 1 MB, a send request keeps for what it
// holds besides its attachments: its recipients, content, headers and
// options.
const requestMarginBytes = 1_000_000

// checkAttachmentsFit returns a validation error when the attachments, which
// every send request carries, leave less than requestMarginBytes of a
// request under maxRequestBytes. Their size is that of their JSON in the
// request: the base64 data, file names and content types.
func checkAttachmentsFit(attachments []common.Attachment) error {
	if len(attachments) == 0 {
		return nil
	}
	encoded, err := json.Marshal(attachments)
	if err != nil {
		return fmt.Errorf("failed to encode the --attach files: %w", err)
	}
	if room := maxRequestBytes - requestMarginBytes; len(encoded) > room {
		return errors.NewValidationError(fmt.Sprintf(
			"the --attach files take %s encoded, more than the %s a send request has room for: the API refuses a request over %s",
			formatBytes(len(encoded)), formatBytes(room), formatBytes(maxRequestBytes)), nil)
	}
	return nil
}

// formatBytes formats a size in bytes with its digits in groups of three,
// such as "29,000,001 bytes", so sizes a byte apart read apart.
func formatBytes(size int) string {
	digits := strconv.Itoa(size)
	var grouped strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}
	return grouped.String() + " bytes"
}

// detectMIMEType detects the MIME type of a file
func detectMIMEType(filePath string, content []byte) string {
	// First try to detect by file extension
	if mimeType := mime.TypeByExtension(filepath.Ext(filePath)); mimeType != "" {
		return mimeType
	}

	// Fall back to content-based detection for common types
	if len(content) == 0 {
		return "application/octet-stream"
	}

	// Check for common file signatures
	switch {
	case len(content) >= 4 && string(content[:4]) == "\x89PNG":
		return "image/png"
	case len(content) >= 3 && string(content[:3]) == "\xFF\xD8\xFF":
		return "image/jpeg"
	case len(content) >= 4 && string(content[:4]) == "GIF8":
		return "image/gif"
	case len(content) >= 4 && string(content[:4]) == "%PDF":
		return "application/pdf"
	case len(content) >= 2 && string(content[:2]) == "PK":
		// Could be ZIP, DOCX, XLSX, etc.
		if strings.HasSuffix(strings.ToLower(filePath), ".docx") {
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}
		if strings.HasSuffix(strings.ToLower(filePath), ".xlsx") {
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
		return "application/zip"
	default:
		// Check if content is text
		if isTextContent(content) {
			return "text/plain"
		}
		return "application/octet-stream"
	}
}

// isTextContent checks if content appears to be text
func isTextContent(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	// Sample first 512 bytes to check for text
	sample := content
	if len(content) > 512 {
		sample = content[:512]
	}

	// Count printable characters
	printable := 0
	for _, b := range sample {
		if (b >= 0x20 && b <= 0x7E) || b == '\t' || b == '\n' || b == '\r' {
			printable++
		}
	}

	// If more than 95% of characters are printable, consider it text
	return float64(printable)/float64(len(sample)) > 0.95
}
