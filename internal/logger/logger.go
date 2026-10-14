// Package logger provides comprehensive logging capabilities for the AhaSend CLI.
//
// This package implements multi-level logging with security-aware features:
//
//   - Structured logging using logrus, as text lines of key=value fields
//   - HTTP transport logging for API debugging (request/response details)
//   - Security-aware log sanitization (API keys and sensitive headers redacted)
//   - Two-level logging: --verbose (API summaries) and --debug (full details)
//   - Configuration operation logging for troubleshooting
//   - Request timing
//   - Color-coded output when terminal.UseColor says so for stderr
//
// The logger automatically detects and redacts sensitive information while
// providing comprehensive debugging information for development and support.
package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/AhaSend/ahasend-cli/internal/terminal"
)

// Logger wraps logrus with CLI-specific functionality
type Logger struct {
	*logrus.Logger
	debugMode   bool
	verboseMode bool
}

// Global logger instance
var defaultLogger *Logger

// Initialize sets up the global logger based on CLI flags. Logs go to the
// command's stderr in every output format, so they never mix with stdout.
func Initialize(cmd *cobra.Command) {
	debug, _ := cmd.Flags().GetBool("debug")
	verbose, _ := cmd.Flags().GetBool("verbose")

	defaultLogger = newLogger(debug, verbose, terminal.UseColor(cmd, cmd.ErrOrStderr()))
	defaultLogger.SetOutput(cmd.ErrOrStderr())
}

// newLogger creates a new logger instance that writes to os.Stderr, in color
// when useColor is set.
func newLogger(debugMode, verboseMode, useColor bool) *Logger {
	log := logrus.New()

	// Set log level based on flags
	if debugMode {
		log.SetLevel(logrus.DebugLevel)
	} else if verboseMode {
		log.SetLevel(logrus.InfoLevel)
	} else {
		log.SetLevel(logrus.WarnLevel)
	}

	// The caller decides colors, so logrus does not check the terminal
	// itself.
	log.SetFormatter(&logrus.TextFormatter{
		ForceColors:            useColor,
		DisableColors:          !useColor,
		DisableTimestamp:       false,
		FullTimestamp:          true,
		TimestampFormat:        time.RFC3339,
		DisableLevelTruncation: true,
	})

	// Set output to stderr to avoid interfering with command output
	log.SetOutput(os.Stderr)

	return &Logger{
		Logger:      log,
		debugMode:   debugMode,
		verboseMode: verboseMode,
	}
}

// Redirect sends the logs to w until the returned function sends them back
// where they went before, such as while a progress bar owns stderr.
func Redirect(w io.Writer) (restore func()) {
	log := Get()
	previous := log.Out
	log.SetOutput(w)
	return func() { log.SetOutput(previous) }
}

// Get returns the global logger instance
func Get() *Logger {
	if defaultLogger == nil {
		// Fallback logger if not initialized
		defaultLogger = newLogger(false, false, false)
	}
	return defaultLogger
}

// IsDebugEnabled returns true if debug mode is enabled
func (l *Logger) IsDebugEnabled() bool {
	return l.debugMode
}

// HTTPRequest logs HTTP request details
func (l *Logger) HTTPRequest(method, url string, headers http.Header, body interface{}) {
	if !l.debugMode && !l.verboseMode {
		return
	}

	// Log the full URL prominently in both verbose and debug modes
	l.WithFields(logrus.Fields{
		"method":   method,
		"full_url": url,
		"type":     "http_request",
	}).Info("Making API request")

	// More detailed logging only in debug mode
	if !l.debugMode {
		return
	}

	l.WithFields(logrus.Fields{
		"method": method,
		"url":    url,
		"type":   "http_request",
	}).Debug("HTTP Request Details")

	if len(headers) > 0 {
		sanitizedHeaders := sanitizeHeaders(headers)
		if len(sanitizedHeaders) > 0 {
			l.WithField("headers", sanitizedHeaders).Debug("Request Headers")
		}
	}

	if body != nil {
		l.WithField("body", loggedBody(body)).Debug("Request Body")
	}
}

// HTTPResponse logs HTTP response details
func (l *Logger) HTTPResponse(statusCode int, status string, headers http.Header, body interface{}, duration time.Duration) {
	if !l.debugMode {
		return
	}

	l.WithFields(logrus.Fields{
		"status_code": statusCode,
		"status":      status,
		"duration":    duration.String(),
		"type":        "http_response",
	}).Debug("HTTP Response")

	if len(headers) > 0 {
		sanitizedHeaders := sanitizeHeaders(headers)
		if len(sanitizedHeaders) > 0 {
			l.WithField("headers", sanitizedHeaders).Debug("Response Headers")
		}
	}

	if body != nil {
		l.WithField("body", loggedBody(body)).Debug("Response Body")
	}
}

// APICall logs API call details at info level (for verbose mode)
func (l *Logger) APICall(method, endpoint string, duration time.Duration) {
	if !l.verboseMode && !l.debugMode {
		return
	}

	l.WithFields(logrus.Fields{
		"method":   method,
		"endpoint": endpoint,
		"duration": duration.String(),
		"type":     "api_call",
	}).Info("API Call")
}

// APIError logs a failed API call in verbose and debug modes. The command
// reports the error itself, so other modes log nothing.
func (l *Logger) APIError(method, endpoint string, statusCode int, err error, duration time.Duration) {
	if !l.verboseMode && !l.debugMode {
		return
	}

	l.WithFields(logrus.Fields{
		"method":      method,
		"endpoint":    endpoint,
		"status_code": statusCode,
		"duration":    duration.String(),
		"type":        "api_error",
	}).WithError(err).Error("API Error")
}

// ConfigOperation logs configuration operations
func (l *Logger) ConfigOperation(operation, profile string, details map[string]interface{}) {
	if !l.debugMode {
		return
	}

	fields := logrus.Fields{
		"operation": operation,
		"type":      "config",
	}

	if profile != "" {
		fields["profile"] = profile
	}

	for key, value := range details {
		fields[key] = value
	}

	l.WithFields(fields).Debug("Config Operation")
}

// maxLoggedBodyBytes is how much of a request or response body debug mode
// logs: the first 64 KiB of it, with its secrets redacted.
const maxLoggedBodyBytes = 64 << 10

// loggedBody returns body, a decoded JSON value or a string, as debug mode
// logs it: indented, with its secrets redacted, and cut after
// maxLoggedBodyBytes with a mark that says how much was left out.
func loggedBody(body interface{}) string {
	text := fmt.Sprintf("%+v", body)
	if bodyBytes, err := json.MarshalIndent(redactSecrets(body), "", "  "); err == nil {
		text = string(bodyBytes)
	}
	if len(text) <= maxLoggedBodyBytes {
		return text
	}
	cut := maxLoggedBodyBytes
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return fmt.Sprintf("%s\n[cut: %d more bytes not logged]", text[:cut], len(text)-cut)
}

// sanitizeHeaders removes sensitive information from headers
func sanitizeHeaders(headers http.Header) map[string]string {
	sanitized := make(map[string]string)

	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"cookie":        true,
		"set-cookie":    true,
	}

	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if sensitiveHeaders[lowerKey] {
			sanitized[key] = "[REDACTED]"
		} else {
			sanitized[key] = strings.Join(values, ", ")
		}
	}

	return sanitized
}

// redactSecrets returns body, a decoded JSON value, with the value of every
// key that names a secret replaced by [REDACTED] at any depth, so debug logs
// never show passwords, secret keys, signing secrets, private keys, API keys
// or tokens.
func redactSecrets(body interface{}) interface{} {
	switch value := body.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(value))
		for key, field := range value {
			if isSecretKey(key) {
				redacted[key] = "[REDACTED]"
			} else {
				redacted[key] = redactSecrets(field)
			}
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, len(value))
		for i, item := range value {
			redacted[i] = redactSecrets(item)
		}
		return redacted
	default:
		return body
	}
}

// isSecretKey reports whether a JSON key names a secret: it contains
// "secret", "password", "token" or "private", or it is "api_key" or
// "apikey", in any case.
func isSecretKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "secret") || strings.Contains(key, "password") ||
		strings.Contains(key, "token") || strings.Contains(key, "private") ||
		key == "api_key" || key == "apikey"
}

// Global convenience functions
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

func ConfigOperation(operation, profile string, details map[string]interface{}) {
	Get().ConfigOperation(operation, profile, details)
}

// HTTPTransport is a custom RoundTripper that logs HTTP requests and responses
type HTTPTransport struct {
	transport http.RoundTripper
	logger    *Logger
}

// NewHTTPTransport creates a new HTTP transport with logging
func NewHTTPTransport(transport http.RoundTripper, logger *Logger) *HTTPTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &HTTPTransport{
		transport: transport,
		logger:    logger,
	}
}

// RoundTrip implements the RoundTripper interface. In debug mode it reads and
// closes the request body and sends a clone carrying a copy of it, so the
// caller's request is never modified.
func (t *HTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	startTime := time.Now()

	// Log request
	var reqBody interface{}
	if req.Body != nil && req.Body != http.NoBody && t.logger.IsDebugEnabled() {
		bodyBytes, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
		req = req.Clone(req.Context())
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}

		// Try to parse as JSON for better logging
		var jsonBody interface{}
		if json.Unmarshal(bodyBytes, &jsonBody) == nil {
			reqBody = jsonBody
		} else {
			reqBody = string(bodyBytes)
		}
	}

	t.logger.HTTPRequest(req.Method, req.URL.String(), req.Header, reqBody)

	// Execute request
	resp, err := t.transport.RoundTrip(req)
	duration := time.Since(startTime)

	if err != nil {
		t.logger.APIError(req.Method, req.URL.Path, 0, err, duration)
		return resp, err
	}

	// Log response
	var respBody interface{}
	if resp.Body != nil && t.logger.IsDebugEnabled() {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr == nil {
			// Restore the body for the caller
			resp.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

			// Try to parse as JSON for better logging
			var jsonBody interface{}
			if json.Unmarshal(bodyBytes, &jsonBody) == nil {
				respBody = jsonBody
			} else {
				respBody = string(bodyBytes)
			}
		}
	}

	t.logger.HTTPResponse(resp.StatusCode, resp.Status, resp.Header, respBody, duration)

	// Log API call summary
	t.logger.APICall(req.Method, req.URL.Path, duration)

	return resp, nil
}
