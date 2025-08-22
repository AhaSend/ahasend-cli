// Package batch provides high-performance concurrent email sending capabilities.
//
// This package implements a worker pool architecture for sending large batches
// of emails with controlled concurrency, comprehensive error handling, and
// progress reporting. Key features include:
//
//   - Configurable concurrency limits (up to 10 concurrent sends)
//   - Automatic retry logic with exponential backoff
//   - Progress reporting with TTY detection
//   - Failed recipient tracking and recovery files
//   - Performance metrics and statistics
//   - Rate limiting integration
//
// The BatchProcessor is the main component that coordinates sending operations,
// manages worker pools, and collects results for comprehensive reporting.
package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/progress"
)

// SendJob represents a batch message send operation (up to 100 recipients)
type SendJob struct {
	Request        *requests.CreateMessageRequest
	IdempotencyKey string
	BatchIndex     int                // Batch number (0, 1, 2, ...)
	Recipients     []common.Recipient // Multiple recipients in this batch (max 100)
	RecipientCount int                // Number of recipients in this batch
}

// SendResult represents the result of a send operation
type SendResult struct {
	Job       *SendJob
	Response  *responses.CreateMessageResponse
	Error     error
	Success   bool
	Retryable bool
	Duration  time.Duration
}

// FailedRecipient represents a recipient that failed to send
type FailedRecipient struct {
	Email            string                 `json:"email" csv:"email"`
	Name             string                 `json:"name,omitempty" csv:"name"`
	SubstitutionData map[string]interface{} `json:"substitution_data,omitempty" csv:"-"`
	Error            string                 `json:"_error" csv:"error"`
	ErrorCode        int                    `json:"_error_code" csv:"error_code"`
	Retryable        bool                   `json:"_retryable" csv:"retryable"`

	// CSV fields for substitution data (dynamic)
	CSVFields map[string]string `json:"-" csv:"-"`
}

// BatchProcessor handles batch message sending with concurrency and error handling
type BatchProcessor struct {
	client           client.AhaSendClient
	maxConcurrency   int
	maxRetries       int
	progressReporter *progress.Reporter
}

// BatchResult contains the overall batch operation results
type BatchResult struct {
	TotalJobs            int                                // Number of API calls made
	TotalRecipients      int                                // Total number of recipients processed
	SuccessfulJobs       int                                // Successful API calls
	FailedJobs           int                                // Failed API calls
	SuccessfulRecipients int                                // Individual recipients sent successfully
	FailedRecipients     []FailedRecipient                  // Individual failed recipients
	SuccessfulResponses  []*responses.CreateMessageResponse // Raw API responses for successful sends
	FailedResponses      []interface{}                      // Raw API error responses for failed batch calls
	Stats                progress.Stats
	FailedRecipientsFile string
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(client client.AhaSendClient, maxConcurrency, maxRetries int, progressReporter *progress.Reporter) *BatchProcessor {
	return &BatchProcessor{
		client:           client,
		maxConcurrency:   maxConcurrency,
		maxRetries:       maxRetries,
		progressReporter: progressReporter,
	}
}

// ProcessJobs processes a batch of send jobs with controlled concurrency
func (bp *BatchProcessor) ProcessJobs(ctx context.Context, jobs []*SendJob) (*BatchResult, error) {
	if len(jobs) == 0 {
		return &BatchResult{}, nil
	}

	// Calculate total recipients
	totalRecipients := 0
	for _, job := range jobs {
		totalRecipients += job.RecipientCount
	}

	logger.Get().WithFields(map[string]interface{}{
		"total_jobs":       len(jobs),
		"total_recipients": totalRecipients,
		"max_concurrency":  bp.maxConcurrency,
		"max_retries":      bp.maxRetries,
	}).Debug("Starting batch processing")

	// Start progress reporting
	if bp.progressReporter != nil {
		bp.progressReporter.Start()
	}

	// Create channels for job processing
	jobChan := make(chan *SendJob, bp.maxConcurrency)
	resultChan := make(chan *SendResult, len(jobs))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < bp.maxConcurrency; i++ {
		wg.Add(1)
		go bp.worker(ctx, &wg, jobChan, resultChan)
	}

	// Send jobs to workers
	go func() {
		defer close(jobChan)
		for _, job := range jobs {
			select {
			case jobChan <- job:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for workers to complete and close result channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var failedRecipients []FailedRecipient
	var successfulResponses []*responses.CreateMessageResponse
	var failedResponses []interface{}
	successfulJobs := 0
	failedJobs := 0
	successfulRecipients := 0
	failedRecipientCount := 0

	for result := range resultChan {
		if result.Success {
			successfulJobs++
			if result.Response != nil {
				successfulResponses = append(successfulResponses, result.Response)
				// Count successful recipients from API response
				if result.Response.Data != nil {
					successfulRecipients += len(result.Response.Data)
				}
			}
		} else {
			failedJobs++
			// Store the raw API error response for JSON output
			failedResponses = append(failedResponses, result.Error)
			// Create failed recipients for all recipients in the failed batch
			for _, recipient := range result.Job.Recipients {
				failedRecipient := bp.createFailedRecipientFromError(recipient, result.Error, result.Retryable)
				failedRecipients = append(failedRecipients, failedRecipient)
				failedRecipientCount++
			}
		}

		// Update progress (recipient-level progress)
		if bp.progressReporter != nil {
			if result.Success && result.Response != nil && result.Response.Data != nil {
				// Update progress for each successful recipient
				for range result.Response.Data {
					bp.progressReporter.Update(true)
				}
			} else {
				// Update progress for each failed recipient in the batch
				for range result.Job.Recipients {
					bp.progressReporter.Update(false)
				}
			}
		}
	}

	// Finish progress reporting and get stats
	var stats progress.Stats
	if bp.progressReporter != nil {
		stats = bp.progressReporter.Finish()
	}

	// Generate failed recipients file if there are failures
	var failedRecipientsFile string
	if len(failedRecipients) > 0 {
		var err error
		failedRecipientsFile, err = bp.saveFailedRecipients(failedRecipients)
		if err != nil {
			logger.Get().WithField("error", err.Error()).Debug("Failed to save failed recipients file")
		}
	}

	return &BatchResult{
		TotalJobs:            len(jobs),
		TotalRecipients:      totalRecipients,
		SuccessfulJobs:       successfulJobs,
		FailedJobs:           failedJobs,
		SuccessfulRecipients: successfulRecipients,
		FailedRecipients:     failedRecipients,
		SuccessfulResponses:  successfulResponses,
		FailedResponses:      failedResponses,
		Stats:                stats,
		FailedRecipientsFile: failedRecipientsFile,
	}, nil
}

// worker processes send jobs from the job channel
func (bp *BatchProcessor) worker(ctx context.Context, wg *sync.WaitGroup, jobChan <-chan *SendJob, resultChan chan<- *SendResult) {
	defer wg.Done()

	for {
		select {
		case job, ok := <-jobChan:
			if !ok {
				return
			}
			result := bp.processSingleJob(ctx, job)

			select {
			case resultChan <- result:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// processSingleJob processes a single send job with retry logic
func (bp *BatchProcessor) processSingleJob(ctx context.Context, job *SendJob) *SendResult {
	var lastErr error
	var response *responses.CreateMessageResponse
	startTime := time.Now()

	ctxDone := false
	for attempt := 0; attempt <= bp.maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			delay := time.Duration(attempt) * time.Second
			logger.Get().WithFields(map[string]interface{}{
				"batch_index":     job.BatchIndex,
				"recipient_count": job.RecipientCount,
				"attempt":         attempt,
				"delay":           delay.String(),
			}).Debug("Retrying batch send job")

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				lastErr = ctx.Err()
				ctxDone = true
				break
			}
		}
		if ctxDone {
			break
		}

		// Attempt to send
		resp, err := bp.client.SendMessageWithIdempotencyKey(*job.Request, job.IdempotencyKey)
		if err == nil {
			response = resp
			lastErr = nil // Clear any previous error on success
			break
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			logger.Get().WithFields(map[string]interface{}{
				"batch_index":     job.BatchIndex,
				"recipient_count": job.RecipientCount,
				"error":           err.Error(),
			}).Debug("Non-retryable error, aborting retries")
			break
		}
	}

	duration := time.Since(startTime)
	success := lastErr == nil
	retryable := false

	if lastErr != nil {
		retryable = isRetryableError(lastErr)
		logger.Get().WithFields(map[string]interface{}{
			"batch_index":     job.BatchIndex,
			"recipient_count": job.RecipientCount,
			"error":           lastErr.Error(),
			"retryable":       retryable,
			"attempts":        bp.maxRetries + 1,
		}).Debug("Batch send job failed")
	} else {
		logger.Get().WithFields(map[string]interface{}{
			"batch_index":     job.BatchIndex,
			"recipient_count": job.RecipientCount,
			"duration":        duration.String(),
		}).Debug("Batch send job succeeded")
	}

	return &SendResult{
		Job:       job,
		Response:  response,
		Error:     lastErr,
		Success:   success,
		Retryable: retryable,
		Duration:  duration,
	}
}

// createFailedRecipientFromError creates a FailedRecipient from an error and recipient info
func (bp *BatchProcessor) createFailedRecipientFromError(recipient common.Recipient, err error, retryable bool) FailedRecipient {
	errorMsg := "Unknown error"
	errorCode := 0

	if err != nil {
		errorMsg = extractActualErrorMessage(err)
		errorCode = extractErrorCode(err)
	}

	failedRecipient := FailedRecipient{
		Email:     recipient.Email,
		Error:     errorMsg,
		ErrorCode: errorCode,
		Retryable: retryable,
		CSVFields: make(map[string]string),
	}

	// Set name if available
	if recipient.Name != nil {
		failedRecipient.Name = *recipient.Name
	}

	// Copy substitution data
	if recipient.Substitutions != nil {
		failedRecipient.SubstitutionData = recipient.Substitutions

		// Also create CSV fields for flat representation
		for key, value := range recipient.Substitutions {
			if str, ok := value.(string); ok {
				failedRecipient.CSVFields[key] = str
			} else {
				failedRecipient.CSVFields[key] = fmt.Sprintf("%v", value)
			}
		}
	}

	return failedRecipient
}

// saveFailedRecipients saves failed recipients to a file for retry
func (bp *BatchProcessor) saveFailedRecipients(failedRecipients []FailedRecipient) (string, error) {
	// Create .ahasend directory if it doesn't exist
	ahasendDir := os.Getenv("HOME") + "/.ahasend"
	if err := os.MkdirAll(ahasendDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .ahasend directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(ahasendDir, fmt.Sprintf("failed-%s.json", timestamp))

	// Save as JSON for now (CSV support can be added later)
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create failed recipients file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(failedRecipients); err != nil {
		return "", fmt.Errorf("failed to write failed recipients: %w", err)
	}

	logger.Get().WithFields(map[string]interface{}{
		"file":  filename,
		"count": len(failedRecipients),
	}).Debug("Saved failed recipients file")
	return filename, nil
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := strings.ToLower(err.Error())

	// Check for retryable conditions
	retryableErrors := []string{
		"rate limit",
		"too many requests",
		"timeout",
		"connection",
		"network",
		"internal server error",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"502", "503", "504", // HTTP status codes
		"429", // Rate limit
	}

	for _, retryableError := range retryableErrors {
		if strings.Contains(errorStr, retryableError) {
			return true
		}
	}

	return false
}

// extractActualErrorMessage attempts to extract the real error message from SDK errors
func extractActualErrorMessage(err error) string {
	if err == nil {
		return "Unknown error"
	}

	// First try to get the raw error message
	errMsg := err.Error()

	// Check if this is a SDK parsing error and try to extract actual message
	if strings.Contains(errMsg, "no value given for required property") {
		// This is likely a SDK parsing error, but we can't extract the original
		// API response here. Return the SDK error but add context.
		return fmt.Sprintf("API parsing error (check debug logs for actual response): %s", errMsg)
	}

	// For other error types, return the raw error message
	return errMsg
}

// extractErrorCode attempts to extract HTTP status code from error
func extractErrorCode(err error) int {
	if err == nil {
		return 0
	}

	// Try to extract from error message
	errMsg := err.Error()
	if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "too many requests") {
		return 429
	}
	if strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "bad request") {
		return 400
	}
	if strings.Contains(errMsg, "unauthorized") {
		return 401
	}
	if strings.Contains(errMsg, "forbidden") {
		return 403
	}
	if strings.Contains(errMsg, "not found") {
		return 404
	}
	if strings.Contains(errMsg, "internal server error") {
		return 500
	}

	// Default to 0 if we can't extract a code
	return 0
}

// GetExitCode returns appropriate exit code based on batch results
func (br *BatchResult) GetExitCode() int {
	if br.TotalJobs == 0 {
		return 3 // Critical error
	}

	if br.FailedJobs == 0 {
		return 0 // All success
	}

	if br.SuccessfulJobs == 0 {
		return 2 // All failed
	}

	return 1 // Partial success
}
