package cmdutil

import (
	stderrors "errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// MaxAttachmentSize is the largest file --attach accepts, in bytes, which
// AttachSizeLimit gives as help and errors do.
const MaxAttachmentSize = 10 * 1024 * 1024

// AttachSizeLimit is MaxAttachmentSize in the words of help and errors: in
// bytes, as the limit of all attachments together is given, and in MiB.
const AttachSizeLimit = "10,485,760 bytes (10 MiB)"

// Attachment is a file read for --attach.
type Attachment struct {
	// Path is the path given to --attach.
	Path string
	Data []byte
}

// ReadAttachPaths returns the paths of the repeatable --attach flag, as
// ReadRepeatable does. A path that does not exist and came from a value the
// flag split at a comma, as --attach 'inv,2026.txt' splits into inv and
// 2026.txt, is a validation error that says to quote a file name with a
// comma.
func ReadAttachPaths(cmd *cobra.Command) ([]string, error) {
	paths, err := ReadRepeatable(cmd, "attach", "")
	if err != nil {
		return nil, err
	}
	for i, path := range paths {
		if _, err := os.Stat(path); stderrors.Is(err, fs.ErrNotExist) && fromSplitValue(cmd, "attach", i) {
			return nil, validation.InvalidValueError("--attach", path,
				"no such file; "+commaHint("attach", "a file name", `a,b.pdf`))
		}
	}
	return paths, nil
}

// ReadAttachments reads every --attach file, so a send fails before it
// connects when one cannot be attached. A missing or unreadable file, a
// directory or other file that is not a regular one, such as a device or a
// named pipe, and a file over MaxAttachmentSize are validation errors.
func ReadAttachments(paths []string) ([]Attachment, error) {
	var attachments []Attachment
	for _, path := range paths {
		data, err := readAttachment(path)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, Attachment{Path: path, Data: data})
	}
	return attachments, nil
}

// readAttachment reads the --attach file at path. It reads at most one byte
// over MaxAttachmentSize, so a file that grows after it was checked cannot
// make it read without bound.
func readAttachment(path string) ([]byte, error) {
	// Stat does not open the file: opening a named pipe waits for a writer.
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.NewValidationError("failed to read --attach", err)
	}
	if info.IsDir() {
		return nil, errors.NewValidationError(fmt.Sprintf("--attach %s is a directory", path), nil)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.NewValidationError(fmt.Sprintf("--attach %s is not a regular file", path), nil)
	}
	if info.Size() > MaxAttachmentSize {
		return nil, attachmentTooLarge(path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.NewValidationError("failed to read --attach", err)
	}
	defer file.Close()
	return readAttachmentData(path, file)
}

// readAttachmentData reads r, the open --attach file at path, up to one byte
// over MaxAttachmentSize: more is a validation error.
func readAttachmentData(path string, r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxAttachmentSize+1))
	if err != nil {
		return nil, errors.NewValidationError("failed to read --attach", err)
	}
	if len(data) > MaxAttachmentSize {
		return nil, attachmentTooLarge(path)
	}
	return data, nil
}

// attachmentTooLarge is the error of an --attach file at path larger than
// MaxAttachmentSize.
func attachmentTooLarge(path string) error {
	return errors.NewValidationError(fmt.Sprintf("--attach file %s is larger than %s", path, AttachSizeLimit), nil)
}
