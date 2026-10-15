package printer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-go/api"
)

// jsonHandler handles JSON output formatting with complete type safety
type jsonHandler struct {
	handlerBase
}

// GetFormat returns the format name
func (h *jsonHandler) GetFormat() string {
	return "json"
}

// HandleError prints err to stdout, as the API's error body or the CLI's
// error object, and returns it. When stdout cannot be written, the error goes
// to stderr instead, and the returned error says why it could not be printed.
func (h *jsonHandler) HandleError(err error) error {
	if err == nil {
		return nil
	}
	if printErr := h.printError(err); printErr != nil {
		fmt.Fprintf(h.errWriter, "Error: %v\n", err)
		return fmt.Errorf("failed to print the error as JSON: %w (original error: %v)", printErr, err)
	}
	return err
}

// printError prints err to stdout.
func (h *jsonHandler) printError(err error) error {
	// A successful response the CLI could not read is printed as the API
	// sent it, and stderr says what failed.
	var unreadable *clierrors.UnreadableResponseError
	if errors.As(err, &unreadable) {
		if printErr := h.printRaw(unreadable.Body); printErr != nil {
			return printErr
		}
		fmt.Fprintf(h.errWriter, "Error: %v\n", err)
		return nil
	}

	// Print an API error's body unchanged when it is JSON
	if body, ok := apiErrorBody(err); ok {
		return h.printRaw(body)
	}
	return h.printJSON(errorObject(err))
}

// apiErrorBody returns the body of the API error, or of the unreadable
// response, in err's chain when that body is JSON.
func apiErrorBody(err error) ([]byte, bool) {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && json.Valid(apiErr.Raw) {
		return apiErr.Raw, true
	}
	var unreadable *clierrors.UnreadableResponseError
	if errors.As(err, &unreadable) && json.Valid(unreadable.Body) {
		return unreadable.Body, true
	}
	return nil, false
}

// errorObject is the JSON object printed for an error that carries no JSON
// API body. For an API error it adds the status and the request ID.
func errorObject(err error) map[string]interface{} {
	errorOutput := map[string]interface{}{
		"error":   true,
		"message": err.Error(),
	}

	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		errorOutput = map[string]interface{}{
			"error":       true,
			"message":     apiErr.Message,
			"status_code": apiErr.StatusCode,
			"request_id":  apiErr.RequestID,
		}
	}

	return errorOutput
}

func (h *jsonHandler) HandleEmpty(message string) error {
	result := map[string]interface{}{
		"empty":   true,
		"message": message,
	}
	return h.printJSON(result)
}

// printJSON prints value, an object the CLI built or an API response with no
// recorded body, as indented JSON.
func (h *jsonHandler) printJSON(value any) error {
	encoded, err := encodeJSON(value, "  ")
	if err != nil {
		return err
	}
	_, err = h.writer.Write(append(encoded, '\n'))
	return err
}

// printAPIResponse prints the body the API returned for value, or value's
// JSON encoding when no body was recorded. It prints nothing when the API
// sent no body: value is nil, or the recorded body is empty.
func (h *jsonHandler) printAPIResponse(value any) error {
	if isNilPointer(value) {
		return nil
	}
	if body, ok := apiresponse.Body(value); ok {
		return h.printRaw(body)
	}
	return h.printJSON(value)
}

// isNilPointer reports whether value is nil or a nil pointer.
func isNilPointer(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// printRaw prints an API body with two-space indentation. Every token is the
// API's own: key order, unknown keys, number literals and string escapes. A
// body that is not valid JSON is written unchanged, followed by a newline
// when it does not end with one; an empty body prints nothing.
func (h *jsonHandler) printRaw(body []byte) error {
	var buffer bytes.Buffer
	if err := json.Indent(&buffer, bytes.TrimRight(body, " \t\r\n"), "", "  "); err != nil {
		buffer.Reset()
		buffer.Write(body)
		if len(body) > 0 && !bytes.HasSuffix(body, []byte("\n")) {
			buffer.WriteByte('\n')
		}
		_, writeErr := h.writer.Write(buffer.Bytes())
		return writeErr
	}
	buffer.WriteByte('\n')
	_, err := h.writer.Write(buffer.Bytes())
	return err
}
