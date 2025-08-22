// Package logger provides comprehensive logging capabilities for the AhaSend CLI.
//
// This package implements multi-level logging with security-aware features:
//
//   - Structured logging using logrus with JSON formatting
//   - HTTP transport logging for API debugging (request/response details)
//   - Security-aware log sanitization (API keys and sensitive headers redacted)
//   - Two-level logging: --verbose (API summaries) and --debug (full details)
//   - Configuration operation logging for troubleshooting
//   - Performance monitoring with request timing and metrics
//   - Color-coded output with --no-color flag support
//
// The logger automatically detects and redacts sensitive information while
// providing comprehensive debugging information for development and support.
package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Logger wraps logrus with CLI-specific functionality
type Logger struct {
	*logrus.Logger
	debugMode   bool
	verboseMode bool
}

// Global logger instance
var defaultLogger *Logger

// Initialize sets up the global logger based on CLI flags
func Initialize(cmd *cobra.Command) {
	debug, _ := cmd.Flags().GetBool("debug")
	verbose, _ := cmd.Flags().GetBool("verbose")
	noColor, _ := cmd.Flags().GetBool("no-color")

	// Check if we're in JSON output mode
	outputFormat, _ := cmd.Flags().GetString("output")
	isJSONMode := outputFormat == "json"

	defaultLogger = NewLoggerWithFormat(debug, verbose, noColor, isJSONMode)
}

// NewLoggerWithFormat creates a new logger instance with JSON mode awareness
func NewLoggerWithFormat(debugMode, verboseMode, noColor, isJSONMode bool) *Logger {
	// In JSON mode, suppress all log output to avoid polluting JSON responses
	if isJSONMode {
		log := logrus.New()
		log.SetOutput(io.Discard)
		return &Logger{
			Logger:      log,
			debugMode:   false,
			verboseMode: false,
		}
	}

	// For non-JSON mode, use the regular logger
	return NewLogger(debugMode, verboseMode, noColor)
}

// NewLogger creates a new logger instance
func NewLogger(debugMode, verboseMode, noColor bool) *Logger {
	log := logrus.New()

	// Set log level based on flags
	if debugMode {
		log.SetLevel(logrus.DebugLevel)
	} else if verboseMode {
		log.SetLevel(logrus.InfoLevel)
	} else {
		log.SetLevel(logrus.WarnLevel)
	}

	// Configure formatter
	if noColor {
		log.SetFormatter(&logrus.TextFormatter{
			DisableColors:          true,
			DisableTimestamp:       false,
			FullTimestamp:          true,
			TimestampFormat:        time.RFC3339,
			DisableLevelTruncation: true,
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			DisableColors:          false,
			DisableTimestamp:       false,
			FullTimestamp:          true,
			TimestampFormat:        time.RFC3339,
			ForceColors:            true,
			DisableLevelTruncation: true,
		})
	}

	// Set output to stderr to avoid interfering with command output
	log.SetOutput(os.Stderr)

	return &Logger{
		Logger:      log,
		debugMode:   debugMode,
		verboseMode: verboseMode,
	}
}

// Get returns the global logger instance
func Get() *Logger {
	if defaultLogger == nil {
		// Fallback logger if not initialized
		defaultLogger = NewLogger(false, false, false)
	}
	return defaultLogger
}

// IsDebugEnabled returns true if debug mode is enabled
func (l *Logger) IsDebugEnabled() bool {
	return l.debugMode
}

// IsVerboseEnabled returns true if verbose mode is enabled
func (l *Logger) IsVerboseEnabled() bool {
	return l.verboseMode
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
		if bodyBytes, err := json.MarshalIndent(body, "", "  "); err == nil {
			l.WithField("body", string(bodyBytes)).Debug("Request Body")
		} else {
			l.WithField("body", fmt.Sprintf("%+v", body)).Debug("Request Body")
		}
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
		if bodyBytes, err := json.MarshalIndent(body, "", "  "); err == nil {
			l.WithField("body", string(bodyBytes)).Debug("Response Body")
		} else {
			l.WithField("body", fmt.Sprintf("%+v", body)).Debug("Response Body")
		}
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

// APIError logs API errors
func (l *Logger) APIError(method, endpoint string, statusCode int, err error, duration time.Duration) {
	l.WithFields(logrus.Fields{
		"method":      method,
		"endpoint":    endpoint,
		"status_code": statusCode,
		"duration":    duration.String(),
		"type":        "api_error",
	}).WithError(err).Error("API Error")
}

// RetryAttempt logs retry attempts
func (l *Logger) RetryAttempt(attempt int, maxAttempts int, err error, delay time.Duration) {
	if !l.debugMode {
		return
	}

	l.WithFields(logrus.Fields{
		"attempt":      attempt,
		"max_attempts": maxAttempts,
		"delay":        delay.String(),
		"type":         "retry_attempt",
	}).WithError(err).Debug("Retry Attempt")
}

// RateLimitHit logs when rate limiting is applied
func (l *Logger) RateLimitHit(waitTime time.Duration) {
	if !l.verboseMode && !l.debugMode {
		return
	}

	l.WithFields(logrus.Fields{
		"wait_time": waitTime.String(),
		"type":      "rate_limit",
	}).Info("Rate Limit Applied")
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

// Global convenience functions
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

func Info(args ...interface{}) {
	Get().Info(args...)
}

func Warn(args ...interface{}) {
	Get().Warn(args...)
}

func Error(args ...interface{}) {
	Get().Error(args...)
}

func HTTPRequest(method, url string, headers http.Header, body interface{}) {
	Get().HTTPRequest(method, url, headers, body)
}

func HTTPResponse(statusCode int, status string, headers http.Header, body interface{}, duration time.Duration) {
	Get().HTTPResponse(statusCode, status, headers, body, duration)
}

func APICall(method, endpoint string, duration time.Duration) {
	Get().APICall(method, endpoint, duration)
}

func APIError(method, endpoint string, statusCode int, err error, duration time.Duration) {
	Get().APIError(method, endpoint, statusCode, err, duration)
}

func RetryAttempt(attempt int, maxAttempts int, err error, delay time.Duration) {
	Get().RetryAttempt(attempt, maxAttempts, err, delay)
}

func RateLimitHit(waitTime time.Duration) {
	Get().RateLimitHit(waitTime)
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

// RoundTrip implements the RoundTripper interface
func (t *HTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	startTime := time.Now()

	// Log request
	var reqBody interface{}
	if req.Body != nil && t.logger.IsDebugEnabled() {
		bodyBytes, err := io.ReadAll(req.Body)
		if err == nil {
			// Restore the body for the actual request
			req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

			// Try to parse as JSON for better logging
			var jsonBody interface{}
			if json.Unmarshal(bodyBytes, &jsonBody) == nil {
				reqBody = jsonBody
			} else {
				reqBody = string(bodyBytes)
			}
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
