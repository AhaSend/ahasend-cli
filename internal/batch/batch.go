// Package batch sends the requests of messages send concurrently.
//
// A BatchProcessor runs send jobs, each one request of up to 100
// recipients, on as many workers as its caller allows; messages send caps
// --max-concurrency at 10. It retries a request that failed with a
// retryable error after the Retry-After the API gave, up to 5 minutes, or
// else after one second more per attempt, up to 30 seconds, plus a random
// part of a second, so that workers whose requests failed together do not
// retry together. Once a request with attachments was too large for the API
// (HTTP 413), it sends no more: they carry the same attachments. It reports each finished
// job to a progress.Reporter, and saves the recipients that failed, and
// those whose outcome is unknown, to files they can be sent again from. It
// does no rate limiting of its own.
package batch

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/config"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/progress"
)

// SendJob represents a batch message send operation (up to 100 recipients).
// A job sets exactly one of Request, for an inline send, and TemplateRequest,
// for a stored template send.
type SendJob struct {
	Request         *requests.CreateMessageRequest
	TemplateRequest *requests.CreateTemplateMessageRequest
	IdempotencyKey  string
	BatchIndex      int                // Batch number (0, 1, 2, ...)
	Recipients      []common.Recipient // Multiple recipients in this batch (max 100)
}

// Outcome is what became of a send job.
type Outcome int

const (
	// OutcomeSent: the API accepted the request.
	OutcomeSent Outcome = iota
	// OutcomeFailed: the request failed, so none of its recipients was sent.
	OutcomeFailed
	// OutcomeUnknown: the job's last request may have been sent, so its
	// recipients may or may not have been sent: the API was still
	// processing an earlier request with the job's idempotency key, a
	// gateway answered 502 or 504, the request was written but no answer
	// came, or the API accepted it with an answer the CLI cannot read.
	OutcomeUnknown
	// OutcomeNotSent: the job's request did not go out: the send was
	// interrupted before it, or stopped after an earlier request of the send
	// was too large for the API.
	OutcomeNotSent
)

// SendResult represents the result of a send operation
type SendResult struct {
	Job       *SendJob
	Response  *responses.CreateMessageResponse
	Error     error
	Outcome   Outcome
	Retryable bool
	// Interrupted is whether the send was interrupted before the job's
	// request went out, or while the job waited to retry, so its retries
	// were cut short.
	Interrupted bool
}

// FailedRecipient represents a recipient that failed to send
type FailedRecipient struct {
	Email            string                 `json:"email"`
	Name             string                 `json:"name,omitempty"`
	SubstitutionData map[string]interface{} `json:"substitution_data,omitempty"`
	Error            string                 `json:"_error"`
	// ErrorCode is the HTTP status of a failed request. It is omitted when
	// the API did not answer, or accepted the request but not the recipient.
	ErrorCode int  `json:"_error_code,omitempty"`
	Retryable bool `json:"_retryable"`
	// IdempotencyKey is the idempotency key of the request of a recipient
	// whose outcome is unknown, which sends it again safely.
	IdempotencyKey string `json:"_idempotency_key,omitempty"`
}

// BatchProcessor handles batch message sending with concurrency and error handling
type BatchProcessor struct {
	client           client.AhaSendClient
	maxConcurrency   int
	maxRetries       int
	progressReporter *progress.Reporter

	// Notices, when set, is where a long wait before a retry is announced,
	// such as stderr.
	Notices   io.Writer
	noticesMu sync.Mutex

	// tooLarge is set once a request of the send with attachments was too
	// large for the API: every request carries the same attachments, so the
	// rest would be too.
	tooLarge atomic.Bool
}

// BatchResult contains the overall batch operation results
type BatchResult struct {
	TotalRecipients      int               // Total number of recipients processed
	SuccessfulRecipients int               // Individual recipients sent successfully
	FailedRecipients     []FailedRecipient // Recipients of failed requests and recipients the API did not accept
	NotSentRecipients    []FailedRecipient // Recipients of jobs an interruption or a request too large for the API kept from being sent
	UnknownRecipients    []FailedRecipient // Recipients of jobs whose outcome is unknown
	Interrupted          bool              // Whether an interruption kept a job from being sent or cut its retries short
	TooLarge             bool              // Whether a request was too large for the API, which kept the rest from being sent
	Results              []*SendResult     // One result per job, in BatchIndex order
	Stats                progress.Stats
	// FailedRecipientsFile is where the failed and not sent recipients were
	// saved to be sent again, if they were.
	FailedRecipientsFile string
	// FailedSaveError is why the failed and not sent recipients could not be
	// saved, if they could not.
	FailedSaveError error
	// UnknownRecipientsFile is where the recipients whose outcome is unknown
	// were saved, if they were.
	UnknownRecipientsFile string
	// UnknownSaveError is why the recipients whose outcome is unknown could
	// not be saved, if they could not.
	UnknownSaveError error
}

// notSentError is the error of the recipients of a job an interruption kept
// from being sent.
const notSentError = "not sent: the send was interrupted"

// notSentTooLargeError is the error of the recipients of a job not sent
// because an earlier request of the send was too large for the API.
const notSentTooLargeError = "not sent: an earlier request of the send was too large for the API (HTTP 413)"

// retryDelayUnit is one second of retry delay: a retry waits one unit more
// per attempt, or the Retry-After seconds the API asked for. Tests shorten
// it.
var retryDelayUnit = time.Second

// maxRetryAfterSeconds is the longest Retry-After, 5 minutes, a retry waits:
// a request the API asks to retry later than that fails instead.
const maxRetryAfterSeconds = 300

// maxFallbackDelaySeconds is the longest wait before a retry the API gave no
// Retry-After for: one second more per attempt, up to 30 seconds.
const maxFallbackDelaySeconds = 30

// retryJitter returns the random part, less than limit, of a wait before a
// retry the API gave no Retry-After for.
var retryJitter = func(limit time.Duration) time.Duration {
	return rand.N(limit)
}

// noticeAfterSeconds is the longest wait before a retry that Notices does
// not announce.
const noticeAfterSeconds = 5

// RetryAfterTooLongError is the failure of a request the API asked to retry
// after more than maxRetryAfterSeconds. It wraps the API's error, so it exits
// the same way.
type RetryAfterTooLongError struct {
	Seconds int
	Err     error
}

func (e *RetryAfterTooLongError) Error() string {
	return fmt.Sprintf("the API asked to retry after %d seconds, more than the %d a send waits: %v",
		e.Seconds, maxRetryAfterSeconds, e.Err)
}

func (e *RetryAfterTooLongError) Unwrap() error { return e.Err }

// NamesTheWait marks the error as a printer.WaitNamer: its text names the
// wait the API asked for.
func (e *RetryAfterTooLongError) NamesTheWait() {}

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
	// With no workers or no attempts nothing would be sent, yet every job
	// would look sent.
	if bp.maxConcurrency < 1 {
		return nil, fmt.Errorf("batch concurrency must be at least 1, got %d", bp.maxConcurrency)
	}
	if bp.maxRetries < 0 {
		return nil, fmt.Errorf("batch retries must be 0 or more, got %d", bp.maxRetries)
	}
	if len(jobs) == 0 {
		return &BatchResult{}, nil
	}
	bp.tooLarge.Store(false)

	// Calculate total recipients
	totalRecipients := 0
	for _, job := range jobs {
		totalRecipients += len(job.Recipients)
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

	// Create channels for job processing. Every job gets a result, so the
	// result channel never blocks a worker.
	jobChan := make(chan *SendJob, bp.maxConcurrency)
	resultChan := make(chan *SendResult, len(jobs))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < bp.maxConcurrency; i++ {
		wg.Add(1)
		go bp.worker(ctx, &wg, jobChan, resultChan)
	}

	// Send jobs to workers; once ctx is done, workers turn the rest into
	// not sent results.
	go func() {
		defer close(jobChan)
		for _, job := range jobs {
			jobChan <- job
		}
	}()

	// Wait for workers to complete and close result channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var failedRecipients, notSentRecipients, unknownRecipients []FailedRecipient
	var results []*SendResult
	successfulRecipients := 0
	interrupted := false

	for result := range resultChan {
		results = append(results, result)
		interrupted = interrupted || result.Interrupted
		var jobFailures, jobUnknown []FailedRecipient
		sent := 0
		switch result.Outcome {
		case OutcomeSent:
			// The API answers a sent request with one item per recipient,
			// and an item with status "error" is a recipient it did not
			// accept.
			jobFailures = RejectedRecipients(result)
			if result.Response != nil {
				sent = len(result.Response.Data) - len(jobFailures)
			}
			failedRecipients = append(failedRecipients, jobFailures...)
		case OutcomeFailed:
			// Every recipient of a failed request failed
			jobFailures = bp.recipientsOf(result)
			failedRecipients = append(failedRecipients, jobFailures...)
		case OutcomeUnknown:
			jobUnknown = bp.recipientsOf(result)
			for i := range jobUnknown {
				jobUnknown[i].IdempotencyKey = result.Job.IdempotencyKey
			}
			unknownRecipients = append(unknownRecipients, jobUnknown...)
		case OutcomeNotSent:
			reason, retryable := notSentError, true
			if !result.Interrupted {
				reason, retryable = notSentTooLargeError, false
			}
			for _, recipient := range result.Job.Recipients {
				notSentRecipients = append(notSentRecipients, newFailedRecipient(recipient, reason, 0, retryable))
			}
			continue
		}
		successfulRecipients += sent

		// Update progress (recipient-level progress)
		if bp.progressReporter != nil {
			bp.progressReporter.Record(sent, len(jobFailures), len(jobUnknown))
		}
	}

	// Results arrive in completion order; report them in job order
	sort.Slice(results, func(i, j int) bool {
		return results[i].Job.BatchIndex < results[j].Job.BatchIndex
	})

	// Finish progress reporting and get stats
	var stats progress.Stats
	if bp.progressReporter != nil {
		stats = bp.progressReporter.Finish()
	}

	// Save the recipients to send again, and apart from them those whose
	// outcome is unknown; each file is saved whether or not the other was.
	var failedRecipientsFile, unknownRecipientsFile string
	var failedSaveErr, unknownSaveErr error
	if resend := append(slices.Clip(failedRecipients), notSentRecipients...); len(resend) > 0 {
		failedRecipientsFile, failedSaveErr = saveRecipients("failed", resend)
	}
	if len(unknownRecipients) > 0 {
		unknownRecipientsFile, unknownSaveErr = saveRecipients("unknown", unknownRecipients)
	}

	return &BatchResult{
		TotalRecipients:       totalRecipients,
		SuccessfulRecipients:  successfulRecipients,
		FailedRecipients:      failedRecipients,
		NotSentRecipients:     notSentRecipients,
		UnknownRecipients:     unknownRecipients,
		Interrupted:           interrupted,
		TooLarge:              bp.tooLarge.Load(),
		Results:               results,
		Stats:                 stats,
		FailedRecipientsFile:  failedRecipientsFile,
		FailedSaveError:       failedSaveErr,
		UnknownRecipientsFile: unknownRecipientsFile,
		UnknownSaveError:      unknownSaveErr,
	}, nil
}

// recipientsOf returns the recipients of a job whose request failed or whose
// outcome is unknown, each with the request's error.
func (bp *BatchProcessor) recipientsOf(result *SendResult) []FailedRecipient {
	recipients := make([]FailedRecipient, 0, len(result.Job.Recipients))
	for _, recipient := range result.Job.Recipients {
		recipients = append(recipients, bp.createFailedRecipientFromError(recipient, result.Error, result.Retryable))
	}
	return recipients
}

// worker processes send jobs from the job channel. Once ctx is done, or a
// request with attachments was too large for the API, it sends no new
// request: each job left becomes a not sent result.
func (bp *BatchProcessor) worker(ctx context.Context, wg *sync.WaitGroup, jobChan <-chan *SendJob, resultChan chan<- *SendResult) {
	defer wg.Done()

	for job := range jobChan {
		if ctx.Err() != nil {
			resultChan <- &SendResult{Job: job, Outcome: OutcomeNotSent, Interrupted: true}
			continue
		}
		if bp.tooLarge.Load() {
			resultChan <- &SendResult{Job: job, Outcome: OutcomeNotSent}
			continue
		}
		result := bp.processSingleJob(ctx, job)
		if errorStatusCode(result.Error) == http.StatusRequestEntityTooLarge && hasAttachments(job) {
			bp.tooLarge.Store(true)
		}
		resultChan <- result
	}
}

// hasAttachments reports whether job's request carries attachments, which
// every request of a send carries alike.
func hasAttachments(job *SendJob) bool {
	if job.TemplateRequest != nil {
		return len(job.TemplateRequest.Attachments) > 0
	}
	return job.Request != nil && len(job.Request.Attachments) > 0
}

// processSingleJob sends a job's request, and sends it again with the same
// idempotency key after a retryable failure, up to maxRetries times. A
// request in flight always runs to its end; once ctx is done, no retry
// follows, and the job keeps the error of its last request.
func (bp *BatchProcessor) processSingleJob(ctx context.Context, job *SendJob) *SendResult {
	startTime := time.Now()
	var response *responses.CreateMessageResponse
	var lastErr error
	attempts := 0
	interrupted := false
	for {
		attempts++
		response, lastErr = bp.send(job)
		if lastErr == nil || !clierrors.IsRetryable(lastErr) || attempts > bp.maxRetries {
			break
		}

		delay, askedByAPI := retryDelay(lastErr, attempts)
		if seconds := int(delay / retryDelayUnit); askedByAPI && seconds > maxRetryAfterSeconds {
			lastErr = &RetryAfterTooLongError{Seconds: seconds, Err: lastErr}
			break
		}
		if delay > noticeAfterSeconds*retryDelayUnit {
			bp.notice("Waiting %s before sending the request of %s again\n",
				describeWait(delay, askedByAPI), recipientCount(len(job.Recipients)))
		}
		logger.Get().WithFields(map[string]interface{}{
			"batch_index":     job.BatchIndex,
			"recipient_count": len(job.Recipients),
			"attempt":         attempts,
			"delay":           delay.String(),
			"error":           lastErr.Error(),
		}).Debug("Retrying batch send job")
		if !wait(ctx, delay) {
			interrupted = true
			break
		}
	}

	duration := time.Since(startTime)
	result := &SendResult{
		Job:         job,
		Response:    response,
		Error:       lastErr,
		Interrupted: interrupted,
	}
	switch {
	case lastErr == nil:
		result.Outcome = OutcomeSent
	case outcomeUnknown(lastErr):
		result.Outcome = OutcomeUnknown
	default:
		result.Outcome = OutcomeFailed
	}

	if lastErr != nil {
		result.Retryable = clierrors.IsRetryable(lastErr)
		logger.Get().WithFields(map[string]interface{}{
			"batch_index":     job.BatchIndex,
			"recipient_count": len(job.Recipients),
			"error":           lastErr.Error(),
			"retryable":       result.Retryable,
			"attempts":        attempts,
		}).Debug("Batch send job failed")
	} else {
		logger.Get().WithFields(map[string]interface{}{
			"batch_index":     job.BatchIndex,
			"recipient_count": len(job.Recipients),
			"attempts":        attempts,
			"duration":        duration.String(),
		}).Debug("Batch send job succeeded")
	}
	return result
}

// describeWait names a wait before a retry: the whole seconds of a
// Retry-After the API asked for, such as "30 seconds, as the API asked,", or
// the seconds of a wait the CLI chose with one decimal, such as
// "5.4 seconds", since its random part can take it over a whole second.
func describeWait(delay time.Duration, askedByAPI bool) string {
	seconds := float64(delay) / float64(retryDelayUnit)
	if askedByAPI {
		return fmt.Sprintf("%.0f seconds, as the API asked,", seconds)
	}
	return fmt.Sprintf("%.1f seconds", seconds)
}

// recipientCount names count recipients, such as "1 recipient".
func recipientCount(count int) string {
	if count == 1 {
		return "1 recipient"
	}
	return fmt.Sprintf("%d recipients", count)
}

// notice writes a notice to Notices, when it is set.
func (bp *BatchProcessor) notice(format string, args ...any) {
	if bp.Notices == nil {
		return
	}
	bp.noticesMu.Lock()
	defer bp.noticesMu.Unlock()
	fmt.Fprintf(bp.Notices, format, args...)
}

// retryDelay returns how long to wait before retry number attempt after
// err, and whether the API asked for that wait: the Retry-After the API gave,
// or else one more second per attempt, up to maxFallbackDelaySeconds, plus a
// random part of a second.
func retryDelay(err error, attempt int) (time.Duration, bool) {
	var apiErr *api.APIError
	if stderrors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		return time.Duration(apiErr.RetryAfter) * retryDelayUnit, true
	}
	return time.Duration(min(attempt, maxFallbackDelaySeconds))*retryDelayUnit + retryJitter(retryDelayUnit), false
}

// wait waits for delay and reports whether it did; it stops early, with
// false, when ctx is done.
func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// outcomeUnknown reports whether a send request that failed with err may
// have been sent: the API said an earlier request with the same idempotency
// key is still being processed; a gateway answered 502 or 504, as it does
// when the API took too long or its connection broke, possibly after the API
// acted; the request was written but no answer came; or the API accepted the
// request with an answer the CLI cannot read. A request that never reached
// the API, such as on a refused connection or a failed DNS lookup or TLS
// handshake, and any other answer failed.
func outcomeUnknown(err error) bool {
	var apiErr *api.APIError
	if stderrors.As(err, &apiErr) {
		return apiErr.Type == api.ErrorTypeIdempotencyConflict ||
			apiErr.StatusCode == http.StatusBadGateway || apiErr.StatusCode == http.StatusGatewayTimeout
	}
	var written *client.RequestWrittenError
	var unreadable *clierrors.UnreadableResponseError
	return stderrors.As(err, &written) || stderrors.As(err, &unreadable)
}

// send calls the client method that matches the job's request
func (bp *BatchProcessor) send(job *SendJob) (*responses.CreateMessageResponse, error) {
	if job.TemplateRequest != nil {
		return bp.client.SendTemplateMessageWithIdempotencyKey(*job.TemplateRequest, job.IdempotencyKey)
	}
	return bp.client.SendMessageWithIdempotencyKey(*job.Request, job.IdempotencyKey)
}

// recipientStatusError is the status of a response item whose recipient the
// API did not accept.
const recipientStatusError = "error"

// RejectedRecipients returns the recipients that a sent request's response
// reports with status "error", with the API's error text. Each keeps the name
// and substitutions it was sent with, so the failed recipients file can be
// sent again. The API answers with one item per recipient in the order of
// the request, so an item is matched to the recipient at its position, which
// tells apart two entries of one address; a response of another length is
// matched to the recipient each item echoes.
func RejectedRecipients(result *SendResult) []FailedRecipient {
	if result.Response == nil {
		return nil
	}
	inOrder := len(result.Response.Data) == len(result.Job.Recipients)

	var failed []FailedRecipient
	for i, item := range result.Response.Data {
		if item.Status != recipientStatusError {
			continue
		}
		recipient := item.Recipient
		if inOrder {
			recipient = result.Job.Recipients[i]
		}
		errorMsg := "Unknown error"
		if item.Error != nil && *item.Error != "" {
			errorMsg = *item.Error
		}
		failed = append(failed, newFailedRecipient(recipient, errorMsg, 0, false))
	}
	return failed
}

// createFailedRecipientFromError creates a FailedRecipient from an error and recipient info
func (bp *BatchProcessor) createFailedRecipientFromError(recipient common.Recipient, err error, retryable bool) FailedRecipient {
	errorMsg := "Unknown error"
	errorCode := 0

	if err != nil {
		errorMsg = err.Error()
		errorCode = errorStatusCode(err)
	}

	return newFailedRecipient(recipient, errorMsg, errorCode, retryable)
}

// newFailedRecipient creates the FailedRecipient entry for recipient.
func newFailedRecipient(recipient common.Recipient, errorMsg string, errorCode int, retryable bool) FailedRecipient {
	failedRecipient := FailedRecipient{
		Email:     recipient.Email,
		Error:     errorMsg,
		ErrorCode: errorCode,
		Retryable: retryable,
	}

	// Set name if available
	if recipient.Name != nil {
		failedRecipient.Name = *recipient.Name
	}

	// Copy substitution data
	if recipient.Substitutions != nil {
		failedRecipient.SubstitutionData = recipient.Substitutions
	}

	return failedRecipient
}

// saveRecipients saves recipients to a new file in the CLI's directory,
// named after prefix and the time, in the format --recipients reads, and
// returns its path. A file it could not write in full is removed.
func saveRecipients(prefix string, recipients []FailedRecipient) (string, error) {
	ahasendDir, err := config.EnsureDir()
	if err != nil {
		return "", err
	}

	// The name holds the time and a random part, so sends in the same second
	// never share a file; CreateTemp never opens an existing one, and creates
	// the file readable by its owner only.
	timestamp := time.Now().Format("20060102-150405")
	file, err := os.CreateTemp(ahasendDir, fmt.Sprintf("%s-%s-*.json", prefix, timestamp))
	if err != nil {
		return "", fmt.Errorf("failed to create the %s recipients file: %w", prefix, err)
	}
	filename := file.Name()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(recipients)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(filename)
		return "", fmt.Errorf("failed to write the %s recipients: %w", prefix, err)
	}

	logger.Get().WithFields(map[string]interface{}{
		"file":  filename,
		"count": len(recipients),
	}).Debug("Saved recipients file")
	return filename, nil
}

// errorStatusCode returns the HTTP status of the API error in err, or 0 when
// the API did not answer.
func errorStatusCode(err error) int {
	var apiErr *api.APIError
	if stderrors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}
