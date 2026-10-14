package cmdutil

import (
	"bytes"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/AhaSend/ahasend-cli/internal/errors"
)

// ReadFlagFile returns the text of the file at path, which the flag named
// flag gives, as ReadInputFile reads it.
func ReadFlagFile(flag, path string) (string, error) {
	data, err := ReadInputFile(flag, path)
	return string(data), err
}

// utf8BOM is the byte order mark some programs, such as Excel saving
// "CSV UTF-8", put at the start of a UTF-8 file.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// The byte order marks of UTF-16, little and big endian, which Windows
// programs such as Notepad save "Unicode" text with.
var (
	utf16LEBOM = []byte{0xFF, 0xFE}
	utf16BEBOM = []byte{0xFE, 0xFF}
)

// ReadInputFile returns the content of the file at path, which the flag named
// flag gives, without a leading UTF-8 byte order mark; every text or data
// file a user gives is read with it. A missing or unreadable file, and a file
// that is not UTF-8 text, are validation errors: the API takes UTF-8, and
// sending other bytes would replace each character that is not UTF-8.
func ReadInputFile(flag, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.NewValidationError(fmt.Sprintf("failed to read --%s", flag), err)
	}
	if bytes.HasPrefix(data, utf16LEBOM) || bytes.HasPrefix(data, utf16BEBOM) {
		return nil, errors.NewValidationError(fmt.Sprintf("%s is UTF-16 text, not UTF-8; save it as UTF-8", path), nil)
	}
	data = bytes.TrimPrefix(data, utf8BOM)
	if !utf8.Valid(data) {
		return nil, errors.NewValidationError(fmt.Sprintf("%s is not UTF-8 text; save it as UTF-8", path), nil)
	}
	return data, nil
}
