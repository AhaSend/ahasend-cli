package errors

import (
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-go/api"
)

// CLIError represents a CLI-specific error
type CLIError struct {
	Code    string
	Message string
	Cause   error
}

func (e *CLIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *CLIError) Unwrap() error {
	return e.Cause
}

// Error codes
const (
	ErrCodeAuth          = "AUTH_ERROR"
	ErrCodeConfig        = "CONFIG_ERROR"
	ErrCodeValidation    = "VALIDATION_ERROR"
	ErrCodeNetwork       = "NETWORK_ERROR"
	ErrCodeAPI           = "API_ERROR"
	ErrCodeFileOperation = "FILE_ERROR"
	ErrCodeTimeout       = "TIMEOUT_ERROR"
	ErrCodeRateLimit     = "RATE_LIMIT_ERROR"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodePermission    = "PERMISSION_ERROR"
)

// NewCLIError creates a new CLI error
func NewCLIError(code, message string, cause error) *CLIError {
	return &CLIError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewAuthError creates an authentication error
func NewAuthError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodeAuth, message, cause)
}

// NewConfigError creates a configuration error
func NewConfigError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodeConfig, message, cause)
}

// NewValidationError creates a validation error
func NewValidationError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodeValidation, message, cause)
}

// NewNetworkError creates a network error
func NewNetworkError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodeNetwork, message, cause)
}

// NewAPIError creates an API error
func NewAPIError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodeAPI, message, cause)
}

// NewFileError creates a file operation error
func NewFileError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodeFileOperation, message, cause)
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodeTimeout, message, cause)
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodeRateLimit, message, cause)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodeNotFound, message, cause)
}

// NewPermissionError creates a permission error
func NewPermissionError(message string, cause error) *CLIError {
	return NewCLIError(ErrCodePermission, message, cause)
}

// ExitWithError prints an error message and exits with code 1
func ExitWithError(err error) {
	if cliErr, ok := err.(*CLIError); ok {
		fmt.Fprintf(os.Stderr, "Error [%s]: %s\n", cliErr.Code, cliErr.Message)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	os.Exit(1)
}

// WrapError wraps an error with additional context
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}

	if cliErr, ok := err.(*CLIError); ok {
		return &CLIError{
			Code:    cliErr.Code,
			Message: message,
			Cause:   err,
		}
	}

	return fmt.Errorf("%s: %w", message, err)
}

// IsNotFoundError checks if an error is a not found error
func IsNotFoundError(err error) bool {
	if cliErr, ok := err.(*CLIError); ok {
		return cliErr.Code == ErrCodeNotFound
	}
	return false
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if cliErr, ok := err.(*CLIError); ok {
		switch cliErr.Code {
		case ErrCodeNetwork, ErrCodeTimeout, ErrCodeRateLimit:
			return true
		case ErrCodeAPI:
			// Check if it's a server error (5xx) using the new SDK error type
			if apiErr, ok := cliErr.Cause.(*api.APIError); ok {
				return apiErr.IsRetryable()
			}
		}
	}
	return false
}

// GetExitCode returns appropriate exit code for error
func GetExitCode(err error) int {
	if cliErr, ok := err.(*CLIError); ok {
		switch cliErr.Code {
		case ErrCodeAuth:
			return 2
		case ErrCodeConfig:
			return 3
		case ErrCodeValidation:
			return 4
		case ErrCodeNotFound:
			return 5
		case ErrCodePermission:
			return 6
		case ErrCodeNetwork, ErrCodeTimeout:
			return 7
		case ErrCodeRateLimit:
			return 8
		default:
			return 1
		}
	}
	return 1
}

// ValidationError represents a field validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "validation errors"
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}

	return fmt.Sprintf("validation errors: %s", strings.Join(messages, "; "))
}

// AddValidationError adds a validation error
func (e *ValidationErrors) AddValidationError(field string, value interface{}, message string) {
	*e = append(*e, ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}
