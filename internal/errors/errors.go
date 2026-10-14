package errors

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	stderrors "errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"

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
	ErrCodeAuth       = "AUTH_ERROR"
	ErrCodeConfig     = "CONFIG_ERROR"
	ErrCodeValidation = "VALIDATION_ERROR"
	ErrCodeNetwork    = "NETWORK_ERROR"
	ErrCodeAPI        = "API_ERROR"
)

// newCLIError creates a new CLI error
func newCLIError(code, message string, cause error) *CLIError {
	return &CLIError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewAuthError creates an authentication error
func NewAuthError(message string, cause error) *CLIError {
	return newCLIError(ErrCodeAuth, message, cause)
}

// NewConfigError creates a configuration error
func NewConfigError(message string, cause error) *CLIError {
	return newCLIError(ErrCodeConfig, message, cause)
}

// NewValidationError creates a validation error
func NewValidationError(message string, cause error) *CLIError {
	return newCLIError(ErrCodeValidation, message, cause)
}

// NewNetworkError creates the error of a connection that failed, which exits
// 7
func NewNetworkError(message string, cause error) *CLIError {
	return newCLIError(ErrCodeNetwork, message, cause)
}

// NewAPIError creates an API error
func NewAPIError(message string, cause error) *CLIError {
	return newCLIError(ErrCodeAPI, message, cause)
}

// ItemsFailedError reports that a command printed its whole result but some
// of the items in it failed. The command exits 1; the result already shows
// which items failed and why.
type ItemsFailedError struct {
	Failed int
	Total  int
	// Noun names the items, such as "recipients".
	Noun string
}

func (e *ItemsFailedError) Error() string {
	return fmt.Sprintf("%d of %d %s failed", e.Failed, e.Total, e.Noun)
}

// NewItemsFailedError creates the error for a result in which failed of total
// items failed.
func NewItemsFailedError(failed, total int, noun string) *ItemsFailedError {
	return &ItemsFailedError{Failed: failed, Total: total, Noun: noun}
}

// UnreadableResponseError reports a successful API response whose body the
// CLI could not read as the result it expects. The command exits 1; JSON
// output prints Body as it is.
type UnreadableResponseError struct {
	Body   []byte // the response body
	Reason string // why the body could not be read
}

func (e *UnreadableResponseError) Error() string {
	return "could not read the API response: " + e.Reason
}

// NewUnreadableResponseError creates the error for a successful response
// whose body could not be read, and why.
func NewUnreadableResponseError(body []byte, reason string) *UnreadableResponseError {
	return &UnreadableResponseError{Body: body, Reason: reason}
}

// ResultPrintedError reports a failure that the command's printed result
// already shows, such as a credentials check that did not pass. The command
// exits with Code, and outside JSON output Message is printed to stderr.
type ResultPrintedError struct {
	Message string
	Code    int
}

func (e *ResultPrintedError) Error() string {
	return e.Message
}

// NewResultPrintedError creates the error for a printed result that shows a
// failure; the command exits with code.
func NewResultPrintedError(message string, code int) *ResultPrintedError {
	return &ResultPrintedError{Message: message, Code: code}
}

// GetExitCode returns the exit code for err. A printed result's error exits
// with its own code, and a CLI error with its own code maps by that code.
// Otherwise an API error, bare or wrapped, maps by its HTTP status, and a
// network failure or timeout exits 7. Anything else exits 1.
func GetExitCode(err error) int {
	var printed *ResultPrintedError
	if stderrors.As(err, &printed) {
		return printed.Code
	}
	var cliErr *CLIError
	if stderrors.As(err, &cliErr) {
		if code := exitCodeForErrorCode(cliErr.Code); code != 1 {
			return code
		}
	}
	var apiErr *api.APIError
	if stderrors.As(err, &apiErr) {
		return exitCodeForHTTPStatus(apiErr.StatusCode)
	}
	if isNetworkError(err) {
		return exitCodeForErrorCode(ErrCodeNetwork)
	}
	return 1
}

// isNetworkError reports whether err is a failure to reach a server: the
// SDK's network error, a failed HTTP request, a failed connection or DNS
// lookup, or a timeout. A failure to read stdin or write stdout is not one,
// although its system error also satisfies net.Error.
func isNetworkError(err error) bool {
	var sdkErr *api.NetworkError
	var urlErr *url.Error
	var opErr *net.OpError
	var dnsErr *net.DNSError
	return stderrors.As(err, &sdkErr) || stderrors.As(err, &urlErr) || stderrors.As(err, &opErr) ||
		stderrors.As(err, &dnsErr) || stderrors.Is(err, os.ErrDeadlineExceeded) ||
		stderrors.Is(err, context.DeadlineExceeded)
}

// IsAPIUnreachable reports whether err is a failure to reach the API: the
// SDK's network error or a failed HTTP request, such as a refused connection,
// a failed DNS lookup or a timeout. A failure to reach another server, such
// as the SMTP server of smtp send, is not one.
func IsAPIUnreachable(err error) bool {
	var sdkErr *api.NetworkError
	var urlErr *url.Error
	return stderrors.As(err, &sdkErr) || stderrors.As(err, &urlErr)
}

// IsRetryable reports whether a request that failed with err may succeed
// when sent again: the API answered 429 or a 5xx status, or said an earlier
// request with the same idempotency key is still in progress, or could not
// be reached for a reason other than its TLS certificate.
func IsRetryable(err error) bool {
	var apiErr *api.APIError
	if stderrors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= http.StatusInternalServerError ||
			apiErr.Type == api.ErrorTypeIdempotencyConflict
	}
	return isNetworkError(err) && !isCertificateError(err)
}

// isCertificateError reports whether err is a failed check of the server's
// TLS certificate, which sending the request again meets again.
func isCertificateError(err error) bool {
	var verifyErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var invalid x509.CertificateInvalidError
	var hostname x509.HostnameError
	return stderrors.As(err, &verifyErr) || stderrors.As(err, &unknownAuthority) ||
		stderrors.As(err, &invalid) || stderrors.As(err, &hostname)
}

func exitCodeForErrorCode(code string) int {
	switch code {
	case ErrCodeAuth:
		return 2
	case ErrCodeConfig:
		return 3
	case ErrCodeValidation:
		return 4
	case ErrCodeNetwork:
		return 7
	default:
		return 1
	}
}

func exitCodeForHTTPStatus(status int) int {
	switch status {
	case http.StatusUnauthorized:
		return 2
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return 4
	case http.StatusNotFound:
		return 5
	case http.StatusForbidden:
		return 6
	case http.StatusTooManyRequests:
		return 8
	default:
		return 1
	}
}
