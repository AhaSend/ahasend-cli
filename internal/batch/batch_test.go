package batch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/client"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/progress"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

func TestNewBatchProcessor(t *testing.T) {
	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(10, false, false, io.Discard)

	processor := NewBatchProcessor(mockClient, 5, 3, progressReporter)

	assert.NotNil(t, processor)
	assert.Equal(t, mockClient, processor.client)
	assert.Equal(t, 5, processor.maxConcurrency)
	assert.Equal(t, 3, processor.maxRetries)
	assert.Equal(t, progressReporter, processor.progressReporter)
}

func TestBatchProcessor_ProcessJobs_EmptyJobs(t *testing.T) {
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 2, 1, nil)

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result.Results))
	assert.Equal(t, 0, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 0, countOutcomes(result, OutcomeFailed))
	assert.Empty(t, result.FailedRecipients)
}

func TestBatchProcessor_ProcessJobs_SingleJobSuccess(t *testing.T) {
	testutil.UseNewHome(t)

	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(1, false, false, io.Discard)
	processor := NewBatchProcessor(mockClient, 1, 1, progressReporter)

	// Set up mock expectations
	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: "sender@example.com"},
		Recipients: []common.Recipient{{Email: "test@example.com"}},
		Subject:    "Test Subject",
	}

	expectedResponse := mocks.NewMessageResponse("msg-123")
	mockClient.On("SendMessageWithIdempotencyKey", *request, "key-123").Return(expectedResponse, nil)

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{{Email: "test@example.com"}},
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{job})

	require.NoError(t, err)
	assert.Equal(t, 1, len(result.Results))
	assert.Equal(t, 1, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 0, countOutcomes(result, OutcomeFailed))
	assert.Empty(t, result.FailedRecipients)
	assert.Empty(t, result.FailedRecipientsFile)

	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_ProcessJobs_TemplateJobUsesTemplateMethod(t *testing.T) {
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 1, nil)

	request := &requests.CreateTemplateMessageRequest{
		TemplateID: uuid.MustParse("77777777-7777-4777-8777-777777777777"),
		Recipients: []common.Recipient{{Email: "test@example.com"}},
	}
	expectedResponse := mocks.NewMessageResponse("msg-123")
	mockClient.On("SendTemplateMessageWithIdempotencyKey", *request, "key-123").Return(expectedResponse, nil)

	job := &SendJob{
		TemplateRequest: request,
		IdempotencyKey:  "key-123",
		Recipients:      request.Recipients,
	}

	result, err := processor.ProcessJobs(context.Background(), []*SendJob{job})

	require.NoError(t, err)
	assert.Equal(t, 1, countOutcomes(result, OutcomeSent))
	mockClient.AssertExpectations(t)
	mockClient.AssertNotCalled(t, "SendMessageWithIdempotencyKey", mock.Anything, mock.Anything)
}

func TestBatchProcessor_ProcessJobs_SingleJobFailure(t *testing.T) {
	testutil.UseNewHome(t)

	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(1, false, false, io.Discard)
	processor := NewBatchProcessor(mockClient, 1, 1, progressReporter)

	// Set up mock expectations for failure
	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: "sender@example.com"},
		Recipients: []common.Recipient{{Email: "test@example.com"}},
		Subject:    "Test Subject",
	}

	testError := errors.New("API error")
	mockClient.On("SendMessageWithIdempotencyKey", *request, "key-123").Return(nil, testError)

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{{Email: "test@example.com"}},
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{job})

	require.NoError(t, err)
	assert.Equal(t, 1, len(result.Results))
	assert.Equal(t, 0, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 1, countOutcomes(result, OutcomeFailed))
	assert.Len(t, result.FailedRecipients, 1)
	assert.NotEmpty(t, result.FailedRecipientsFile)

	// Check failed recipient details
	failedRecipient := result.FailedRecipients[0]
	assert.Equal(t, "test@example.com", failedRecipient.Email)
	assert.Equal(t, "API error", failedRecipient.Error)
	assert.False(t, failedRecipient.Retryable)

	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_ProcessJobs_MultipleJobs(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)

	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(3, false, false, io.Discard)
	processor := NewBatchProcessor(mockClient, 2, 1, progressReporter)

	// Create test jobs
	jobs := make([]*SendJob, 3)
	for i := 0; i < 3; i++ {
		request := &requests.CreateMessageRequest{
			From:       common.SenderAddress{Email: "sender@example.com"},
			Recipients: []common.Recipient{{Email: fmt.Sprintf("test%d@example.com", i+1)}},
			Subject:    "Test Subject",
		}

		jobs[i] = &SendJob{
			Request:        request,
			IdempotencyKey: fmt.Sprintf("key-%d", i+1),
			BatchIndex:     i,
			Recipients:     []common.Recipient{{Email: fmt.Sprintf("test%d@example.com", i+1)}},
		}

		// First two succeed, third fails
		if i < 2 {
			expectedResponse := mocks.NewMessageResponse(fmt.Sprintf("msg-%d", i+1))
			mockClient.On("SendMessageWithIdempotencyKey", *request, fmt.Sprintf("key-%d", i+1)).Return(expectedResponse, nil)
		} else {
			testError := &api.APIError{StatusCode: 503, Message: "service unavailable"}
			mockClient.On("SendMessageWithIdempotencyKey", *request, fmt.Sprintf("key-%d", i+1)).Return(nil, testError)
		}
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, jobs)

	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Results))
	assert.Equal(t, 2, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 1, countOutcomes(result, OutcomeFailed))
	assert.Len(t, result.FailedRecipients, 1)
	assert.NotEmpty(t, result.FailedRecipientsFile)

	// Check that the failed recipient is the third one
	failedRecipient := result.FailedRecipients[0]
	assert.Equal(t, "test3@example.com", failedRecipient.Email)
	assert.True(t, failedRecipient.Retryable) // a 503 is retryable
	assert.Equal(t, 503, failedRecipient.ErrorCode)

	mockClient.AssertExpectations(t)
}

// TestBatchProcessor_ProcessJobs_ContextCancellation checks that a send
// whose context is done before it starts sends nothing and reports its job
// as not sent.
func TestBatchProcessor_ProcessJobs_ContextCancellation(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 1, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := processor.ProcessJobs(ctx, templateJobs(1))

	require.NoError(t, err)
	assert.True(t, result.Interrupted)
	require.Len(t, result.Results, 1)
	assert.Equal(t, OutcomeNotSent, result.Results[0].Outcome)
	assert.Equal(t, 0, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 0, countOutcomes(result, OutcomeFailed))
	assert.Equal(t, 0, result.SuccessfulRecipients)
	assert.Empty(t, result.FailedRecipients)
	require.Len(t, result.NotSentRecipients, 1)
	assert.Equal(t, "user0@example.com", result.NotSentRecipients[0].Email)
	mockClient.AssertNotCalled(t, "SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything)
}

func TestBatchProcessor_RetryLogic(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)

	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 1, nil) // 1 retry for 2 total attempts

	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: "sender@example.com"},
		Recipients: []common.Recipient{{Email: "test@example.com"}},
		Subject:    "Test Subject",
	}

	// First call fails with retryable error, second succeeds
	retryableError := &api.APIError{StatusCode: 429, Message: "rate limit exceeded"}
	successResponse := mocks.NewMessageResponse("msg-123")

	// Use mock.Anything for the request to avoid comparison issues
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, "key-123").Return(nil, retryableError).Once()
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, "key-123").Return(successResponse, nil).Once()

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{{Email: "test@example.com"}},
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{job})

	require.NoError(t, err)
	assert.Equal(t, 1, len(result.Results))
	assert.Equal(t, 1, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 0, countOutcomes(result, OutcomeFailed))

	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_NonRetryableError(t *testing.T) {
	testutil.UseNewHome(t)

	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 2, nil) // 2 retries

	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: "sender@example.com"},
		Recipients: []common.Recipient{{Email: "test@example.com"}},
		Subject:    "Test Subject",
	}

	// Non-retryable error should only be called once
	nonRetryableError := errors.New("invalid email address")
	mockClient.On("SendMessageWithIdempotencyKey", *request, "key-123").Return(nil, nonRetryableError).Once()

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{{Email: "test@example.com"}},
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{job})

	require.NoError(t, err)
	assert.Equal(t, 1, len(result.Results))
	assert.Equal(t, 0, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 1, countOutcomes(result, OutcomeFailed))
	assert.Len(t, result.FailedRecipients, 1)
	assert.False(t, result.FailedRecipients[0].Retryable)

	mockClient.AssertExpectations(t)
}

func TestErrorStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"nil error", nil, 0},
		{"generic error", errors.New("rate limit exceeded"), 0},
		{"API error", &api.APIError{StatusCode: 429}, 429},
		{"wrapped API error", fmt.Errorf("send: %w", &api.APIError{StatusCode: 400}), 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, errorStatusCode(tt.err))
		})
	}
}

func TestCreateFailedRecipientFromError(t *testing.T) {
	processor := NewBatchProcessor(&mocks.MockClient{}, 1, 1, nil)

	recipient := common.Recipient{
		Email: "test@example.com",
		Name:  ahasend.String("Test User"),
		Substitutions: map[string]interface{}{
			"first_name": "John",
			"age":        25,
		},
	}

	failedRecipient := processor.createFailedRecipientFromError(recipient, errors.New("test error"), true)

	assert.Equal(t, "test@example.com", failedRecipient.Email)
	assert.Equal(t, "Test User", failedRecipient.Name)
	assert.Equal(t, "test error", failedRecipient.Error)
	assert.Equal(t, 0, failedRecipient.ErrorCode)
	assert.True(t, failedRecipient.Retryable)

	// Check substitution data
	assert.Equal(t, "John", failedRecipient.SubstitutionData["first_name"])
	assert.Equal(t, 25, failedRecipient.SubstitutionData["age"])
}

func TestCreateFailedRecipientFromError_NoNameOrSubstitutions(t *testing.T) {
	processor := NewBatchProcessor(&mocks.MockClient{}, 1, 1, nil)

	recipient := common.Recipient{
		Email: "test@example.com",
		// No name or substitutions
	}

	failedRecipient := processor.createFailedRecipientFromError(recipient, nil, false)

	assert.Equal(t, "test@example.com", failedRecipient.Email)
	assert.Equal(t, "", failedRecipient.Name)
	assert.Equal(t, "Unknown error", failedRecipient.Error)
	assert.Equal(t, 0, failedRecipient.ErrorCode)
	assert.False(t, failedRecipient.Retryable)
	assert.Nil(t, failedRecipient.SubstitutionData)
}

func TestSaveFailedRecipients(t *testing.T) {
	testutil.UseNewHome(t)

	failedRecipients := []FailedRecipient{
		{
			Email:     "test1@example.com",
			Name:      "Test User 1",
			Error:     "Rate limit exceeded",
			ErrorCode: 429,
			Retryable: true,
		},
		{
			Email:     "test2@example.com",
			Name:      "Test User 2",
			Error:     "Invalid email",
			ErrorCode: 400,
			Retryable: false,
		},
	}

	filename, err := saveRecipients("failed", failedRecipients)

	require.NoError(t, err)
	assert.NotEmpty(t, filename)
	assert.Contains(t, filename, filepath.Join(".ahasend", "failed-"))
	assert.Contains(t, filename, ".json")

	// Verify file exists and contains expected data
	assert.FileExists(t, filename)

	// Read and verify file contents
	fileData, err := os.ReadFile(filename)
	require.NoError(t, err)

	content := string(fileData)
	assert.Contains(t, content, "test1@example.com")
	assert.Contains(t, content, "test2@example.com")
	assert.Contains(t, content, "Rate limit exceeded")
	assert.Contains(t, content, "Invalid email")
}

func TestBatchProcessor_ConcurrencyLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	testutil.UseNewHome(t)

	mockClient := &mocks.MockClient{}
	// Set concurrency to 2
	processor := NewBatchProcessor(mockClient, 2, 1, nil)

	// Create 5 jobs
	jobs := make([]*SendJob, 5)
	for i := 0; i < 5; i++ {
		request := &requests.CreateMessageRequest{
			From:       common.SenderAddress{Email: "sender@example.com"},
			Recipients: []common.Recipient{{Email: fmt.Sprintf("test%d@example.com", i+1)}},
			Subject:    "Test Subject",
		}

		jobs[i] = &SendJob{
			Request:        request,
			IdempotencyKey: fmt.Sprintf("key-%d", i+1),
			BatchIndex:     i,
			Recipients:     []common.Recipient{{Email: fmt.Sprintf("test%d@example.com", i+1)}},
		}

		// All succeed but with a small delay to test concurrency
		expectedResponse := mocks.NewMessageResponse(fmt.Sprintf("msg-%d", i+1))
		mockClient.On("SendMessageWithIdempotencyKey", *request, fmt.Sprintf("key-%d", i+1)).Return(expectedResponse, nil).Maybe()
	}

	ctx := context.Background()
	start := time.Now()
	result, err := processor.ProcessJobs(ctx, jobs)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 5, len(result.Results))
	assert.Equal(t, 5, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 0, countOutcomes(result, OutcomeFailed))

	// With concurrency of 2, this should complete faster than sequential
	// This is a rough test - actual timing depends on system load
	t.Logf("Batch processing took %v with concurrency limit 2", duration)

	// We just verify it completed without hanging
	assert.Less(t, duration, 30*time.Second, "Batch processing should not take too long")
}

func TestBatchProcessor_ProgressReporting(t *testing.T) {
	testutil.UseNewHome(t)

	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(2, false, false, io.Discard) // No progress bar, just stats
	processor := NewBatchProcessor(mockClient, 1, 1, progressReporter)

	// Create two jobs
	jobs := make([]*SendJob, 2)
	for i := 0; i < 2; i++ {
		request := &requests.CreateMessageRequest{
			From:       common.SenderAddress{Email: "sender@example.com"},
			Recipients: []common.Recipient{{Email: fmt.Sprintf("test%d@example.com", i+1)}},
			Subject:    "Test Subject",
		}

		jobs[i] = &SendJob{
			Request:        request,
			IdempotencyKey: fmt.Sprintf("key-%d", i+1),
			BatchIndex:     i,
			Recipients:     []common.Recipient{{Email: fmt.Sprintf("test%d@example.com", i+1)}},
		}

		// First succeeds, second fails
		if i == 0 {
			expectedResponse := mocks.NewMessageResponse("msg-1")
			mockClient.On("SendMessageWithIdempotencyKey", *request, "key-1").Return(expectedResponse, nil)
		} else {
			testError := errors.New("API error")
			mockClient.On("SendMessageWithIdempotencyKey", *request, "key-2").Return(nil, testError)
		}
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, jobs)

	require.NoError(t, err)
	assert.Equal(t, 2, len(result.Results))
	assert.Equal(t, 1, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 1, countOutcomes(result, OutcomeFailed))

	// Check that stats were collected
	assert.NotEmpty(t, result.Stats) // Stats should be populated

	mockClient.AssertExpectations(t)
}

// Benchmark tests
func BenchmarkBatchProcessor_ProcessJobs_Sequential(b *testing.B) {
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 1, nil) // Sequential processing

	// Set up mock expectations
	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: "sender@example.com"},
		Recipients: []common.Recipient{{Email: "test@example.com"}},
		Subject:    "Test Subject",
	}

	expectedResponse := mocks.NewMessageResponse("msg-123")
	mockClient.On("SendMessageWithIdempotencyKey", *request, "key-123").Return(expectedResponse, nil).Maybe()

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{{Email: "test@example.com"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, _ = processor.ProcessJobs(ctx, []*SendJob{job})
	}
}

func BenchmarkCreateFailedRecipient(b *testing.B) {
	processor := NewBatchProcessor(&mocks.MockClient{}, 1, 1, nil)

	recipient := common.Recipient{
		Email: "test@example.com",
		Name:  ahasend.String("Test User"),
		Substitutions: map[string]interface{}{
			"first_name": "John",
			"last_name":  "Doe",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.createFailedRecipientFromError(recipient, errors.New("test error"), true)
	}
}

func TestBatchProcessor_ProcessJobs_ResultsAreInJobOrder(t *testing.T) {
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 3, 0, nil)

	// Later jobs answer sooner, so results complete in reverse job order.
	delays := []time.Duration{60 * time.Millisecond, 30 * time.Millisecond, 0}
	jobs := make([]*SendJob, len(delays))
	for i, delay := range delays {
		request := &requests.CreateTemplateMessageRequest{
			TemplateID: uuid.MustParse("77777777-7777-4777-8777-777777777777"),
			Recipients: []common.Recipient{{Email: fmt.Sprintf("user%d@example.com", i)}},
		}
		key := fmt.Sprintf("key-%d", i)
		mockClient.On("SendTemplateMessageWithIdempotencyKey", *request, key).
			After(delay).Return(mocks.NewMessageResponse(key), nil)
		jobs[i] = &SendJob{TemplateRequest: request, IdempotencyKey: key, BatchIndex: i,
			Recipients: request.Recipients}
	}

	result, err := processor.ProcessJobs(context.Background(), jobs)

	require.NoError(t, err)
	require.Len(t, result.Results, len(jobs))
	for i, sendResult := range result.Results {
		assert.Same(t, jobs[i], sendResult.Job)
	}
}

func TestBatchProcessor_ProcessJobs_RefusesOptionsThatSendNothing(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		retries     int
		message     string
	}{
		{"no workers", 0, 1, "batch concurrency must be at least 1, got 0"},
		{"negative workers", -1, 1, "batch concurrency must be at least 1, got -1"},
		{"negative retries", 1, -1, "batch retries must be 0 or more, got -1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			processor := NewBatchProcessor(mockClient, test.concurrency, test.retries, nil)
			job := &SendJob{
				TemplateRequest: &requests.CreateTemplateMessageRequest{
					Recipients: []common.Recipient{{Email: "user@example.com"}},
				},
				IdempotencyKey: "key",
			}

			result, err := processor.ProcessJobs(context.Background(), []*SendJob{job})

			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.Nil(t, result)
			mockClient.AssertNotCalled(t, "SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything)
		})
	}
}

func TestBatchProcessor_ProcessJobs_CountsRecipientsTheAPIRejectsAsFailed(t *testing.T) {
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 0, nil)
	recipients := []common.Recipient{
		{Email: "ok@example.com"},
		{Email: "blocked@example.com", Name: ahasend.String("Pat"), Substitutions: map[string]interface{}{"first_name": "Pat"}},
	}
	request := &requests.CreateMessageRequest{Recipients: recipients}
	reason := "Recipient is suppressed"
	mockClient.On("SendMessageWithIdempotencyKey", *request, "key").Return(&responses.CreateMessageResponse{
		Data: []responses.CreateSingleMessageResponse{
			{Recipient: common.Recipient{Email: "ok@example.com"}, Status: "queued"},
			{Recipient: common.Recipient{Email: "blocked@example.com"}, Status: "error", Error: &reason},
		},
	}, nil)

	result, err := processor.ProcessJobs(context.Background(), []*SendJob{
		{Request: request, IdempotencyKey: "key", Recipients: recipients},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, countOutcomes(result, OutcomeSent))
	assert.Equal(t, 1, result.SuccessfulRecipients)
	require.Len(t, result.FailedRecipients, 1)
	failed := result.FailedRecipients[0]
	assert.Equal(t, "blocked@example.com", failed.Email)
	assert.Equal(t, "Pat", failed.Name)
	assert.Equal(t, map[string]interface{}{"first_name": "Pat"}, failed.SubstitutionData)
	assert.Equal(t, reason, failed.Error)
	assert.Equal(t, 0, failed.ErrorCode)
	assert.False(t, failed.Retryable)
	assert.NotEmpty(t, result.FailedRecipientsFile)
}

func TestSaveFailedRecipientsNeverOverwritesAnEarlierFile(t *testing.T) {

	first, err := saveRecipients("failed", []FailedRecipient{{Email: "a@example.com"}})
	require.NoError(t, err)
	second, err := saveRecipients("failed", []FailedRecipient{{Email: "b@example.com"}})
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
	assert.Equal(t, filepath.Join(os.Getenv("HOME"), ".ahasend"), filepath.Dir(first))
	assert.Regexp(t, `^failed-\d{8}-\d{6}-\d+\.json$`, filepath.Base(first))
	data, err := os.ReadFile(first)
	require.NoError(t, err)
	assert.Contains(t, string(data), "a@example.com")
}

func TestSaveFailedRecipientsKeepsTheFileReadableByTheOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows files have no POSIX modes")
	}
	home := testutil.UseNewHome(t)
	dir := filepath.Join(home, ".ahasend")
	require.NoError(t, os.Mkdir(dir, 0o755))
	require.NoError(t, os.Chmod(dir, 0o755))

	filename, err := saveRecipients("failed", []FailedRecipient{{Email: "a@example.com"}})
	require.NoError(t, err)

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
	fileInfo, err := os.Stat(filename)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
}

// shortenRetryDelays makes a second of retry delay a millisecond for the
// rest of the test.
func shortenRetryDelays(t *testing.T) {
	t.Helper()
	previous := retryDelayUnit
	retryDelayUnit = time.Millisecond
	t.Cleanup(func() { retryDelayUnit = previous })
}

// inProgressError is the API's answer that an earlier request with the same
// idempotency key is still being processed.
func inProgressError(retryAfter int) error {
	return &api.APIError{StatusCode: 409, Type: api.ErrorTypeIdempotencyConflict, RetryAfter: retryAfter,
		Message: "a request with this idempotency key is in progress"}
}

// templateJobs returns count template send jobs of one recipient each.
func templateJobs(count int) []*SendJob {
	jobs := make([]*SendJob, count)
	for i := range jobs {
		request := &requests.CreateTemplateMessageRequest{
			TemplateID: uuid.MustParse("77777777-7777-4777-8777-777777777777"),
			Recipients: []common.Recipient{{Email: fmt.Sprintf("user%d@example.com", i)}},
		}
		jobs[i] = &SendJob{TemplateRequest: request, IdempotencyKey: fmt.Sprintf("key-%d", i), BatchIndex: i,
			Recipients: request.Recipients}
	}
	return jobs
}

func TestBatchProcessor_RetriesARequestInProgressAfterItsRetryAfter(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 1, nil)
	jobs := templateJobs(1)
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, inProgressError(50)).Once()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(mocks.NewMessageResponse("msg"), nil).Once()

	start := time.Now()
	result, err := processor.ProcessJobs(context.Background(), jobs)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond, "the retry waits the Retry-After")
	assert.Equal(t, OutcomeSent, result.Results[0].Outcome)
	assert.Empty(t, result.FailedRecipients)
	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_ARequestStillInProgressHasAnUnknownOutcome(t *testing.T) {
	home := testutil.UseNewHome(t)
	shortenRetryDelays(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 2, nil)
	jobs := templateJobs(1)
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, inProgressError(1)).Times(3)

	result, err := processor.ProcessJobs(context.Background(), jobs)

	require.NoError(t, err)
	assert.Equal(t, OutcomeUnknown, result.Results[0].Outcome)
	assert.Empty(t, result.FailedRecipients, "the recipients may have been sent, so they are not sent again")
	assert.Empty(t, result.FailedRecipientsFile)
	require.Len(t, result.UnknownRecipients, 1)
	assert.Equal(t, "user0@example.com", result.UnknownRecipients[0].Email)
	assert.Equal(t, filepath.Join(home, ".ahasend"), filepath.Dir(result.UnknownRecipientsFile))
	assert.Regexp(t, `^unknown-\d{8}-\d{6}-\d+\.json$`, filepath.Base(result.UnknownRecipientsFile))
	mockClient.AssertExpectations(t)
}

// TestBatchProcessor_AnInterruptionLetsTheRequestInFlightFinish checks that a
// send interrupted while a request is in flight keeps that request's result,
// sends no other request, and saves the recipients of the jobs it never sent
// to be sent again.
func TestBatchProcessor_AnInterruptionLetsTheRequestInFlightFinish(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 3, nil)
	jobs := templateJobs(3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").
		Run(func(mock.Arguments) {
			cancel()
			time.Sleep(20 * time.Millisecond) // the response is still on its way
		}).
		Return(mocks.NewMessageResponse("msg"), nil).Once()

	result, err := processor.ProcessJobs(ctx, jobs)

	require.NoError(t, err)
	assert.True(t, result.Interrupted)
	require.Len(t, result.Results, 3, "every job has a result")
	assert.Equal(t, []Outcome{OutcomeSent, OutcomeNotSent, OutcomeNotSent},
		[]Outcome{result.Results[0].Outcome, result.Results[1].Outcome, result.Results[2].Outcome})
	assert.Equal(t, 1, result.SuccessfulRecipients)
	require.Len(t, result.NotSentRecipients, 2)
	assert.Equal(t, "not sent: the send was interrupted", result.NotSentRecipients[0].Error)
	assert.Empty(t, result.FailedRecipients)
	data, err := os.ReadFile(result.FailedRecipientsFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "user1@example.com")
	assert.Contains(t, string(data), "user2@example.com")
	assert.NotContains(t, string(data), "user0@example.com")
	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_AnInterruptionDuringTheWaitForARequestInProgressLeavesItUnknown(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 3, nil)
	jobs := templateJobs(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").
		Run(func(mock.Arguments) { cancel() }).
		Return(nil, inProgressError(maxRetryAfterSeconds)).Once()

	start := time.Now()
	result, err := processor.ProcessJobs(ctx, jobs)

	require.NoError(t, err)
	assert.Less(t, time.Since(start), maxRetryAfterSeconds*retryDelayUnit, "the interruption ends the wait")
	assert.Equal(t, OutcomeUnknown, result.Results[0].Outcome)
	assert.Len(t, result.UnknownRecipients, 1)
	assert.Empty(t, result.NotSentRecipients)
	assert.True(t, result.Interrupted, "the interruption cut the retries short")
	mockClient.AssertExpectations(t)
}

// TestBatchProcessor_AnInterruptionAfterTheLastRequestInterruptsNothing
// checks that a signal during the last request, which then succeeds, leaves
// nothing unsent, so the send is not interrupted.
func TestBatchProcessor_AnInterruptionAfterTheLastRequestInterruptsNothing(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 3, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").
		Run(func(mock.Arguments) { cancel() }).
		Return(mocks.NewMessageResponse("msg"), nil).Once()

	result, err := processor.ProcessJobs(ctx, templateJobs(1))

	require.NoError(t, err)
	assert.False(t, result.Interrupted)
	assert.Equal(t, 1, result.SuccessfulRecipients)
	mockClient.AssertExpectations(t)
}

// TestBatchProcessor_OutcomeOfTheLastFailure checks which failures leave
// the outcome of a request unknown, because it may have reached the API,
// and which fail it.
func TestBatchProcessor_OutcomeOfTheLastFailure(t *testing.T) {
	dialErr := &url.Error{Op: "Post", URL: "https://api.ahasend.com/v2", Err: &net.OpError{Op: "dial", Net: "tcp",
		Err: syscall.ECONNREFUSED}}
	tests := map[string]struct {
		err  error
		want Outcome
	}{
		"request in progress":   {inProgressError(1), OutcomeUnknown},
		"bad gateway":           {&api.APIError{StatusCode: 502, Message: "bad gateway"}, OutcomeUnknown},
		"gateway timeout":       {&api.APIError{StatusCode: 504, Message: "gateway timeout"}, OutcomeUnknown},
		"no answer once sent":   {&client.RequestWrittenError{Err: &url.Error{Op: "Post", URL: "https://api.ahasend.com/v2", Err: io.EOF}}, OutcomeUnknown},
		"unreadable acceptance": {clierrors.NewUnreadableResponseError([]byte("<html>"), "the response is not JSON"), OutcomeUnknown},
		"connection refused":    {dialErr, OutcomeFailed},
		"server error":          {&api.APIError{StatusCode: 500, Message: "internal error"}, OutcomeFailed},
		"unavailable":           {&api.APIError{StatusCode: 503, Message: "unavailable"}, OutcomeFailed},
		"bad request":           {&api.APIError{StatusCode: 400, Message: "bad request"}, OutcomeFailed},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			testutil.UseNewHome(t)
			mockClient := &mocks.MockClient{}
			processor := NewBatchProcessor(mockClient, 1, 0, nil)
			mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, test.err).Once()

			result, err := processor.ProcessJobs(context.Background(), templateJobs(1))

			require.NoError(t, err)
			assert.Equal(t, test.want, result.Results[0].Outcome)
			if test.want == OutcomeUnknown {
				assert.Len(t, result.UnknownRecipients, 1)
				assert.Empty(t, result.FailedRecipientsFile, "recipients that may have been sent are not sent again")
			} else {
				assert.Len(t, result.FailedRecipients, 1)
				assert.Empty(t, result.UnknownRecipientsFile)
			}
		})
	}
}

func TestBatchProcessor_DebugLogCountsTheAttemptsMade(t *testing.T) {
	testutil.UseNewHome(t)
	var log strings.Builder
	cmd := &cobra.Command{}
	cmd.Flags().Bool("debug", true, "")
	cmd.SetErr(&log)
	logger.Initialize(cmd)
	t.Cleanup(func() { logger.Initialize(&cobra.Command{}) })
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 3, nil)
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").
		Return(nil, &api.APIError{StatusCode: 400, Message: "bad request"}).Once()

	_, err := processor.ProcessJobs(context.Background(), templateJobs(1))

	require.NoError(t, err)
	assert.Contains(t, log.String(), "attempts=1 ")
	mockClient.AssertExpectations(t)
}

// TestBatchProcessor_SavesEachRecipientsFileOnItsOwn checks that the failed
// and the unknown recipients files are saved whether or not the other one
// could be, that each save error is reported on its own, and that a file
// that could not be written in full is removed.
func TestBatchProcessor_SavesEachRecipientsFileOnItsOwn(t *testing.T) {
	gatewayTimeout := &api.APIError{StatusCode: 504, Message: "gateway timeout"}
	badRequest := &api.APIError{StatusCode: 400, Message: "bad request"}
	tests := map[string]struct {
		unsavable  error  // the error of the job whose recipient cannot be encoded
		saved      error  // the error of the other job
		wantPrefix string // the prefix of the file that is saved
	}{
		"failed file cannot be saved":  {unsavable: badRequest, saved: gatewayTimeout, wantPrefix: "unknown-"},
		"unknown file cannot be saved": {unsavable: gatewayTimeout, saved: badRequest, wantPrefix: "failed-"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			home := testutil.UseNewHome(t)
			mockClient := &mocks.MockClient{}
			processor := NewBatchProcessor(mockClient, 1, 0, nil)
			jobs := templateJobs(2)
			jobs[0].Recipients[0].Substitutions = map[string]interface{}{"score": math.Inf(1)}
			mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, test.unsavable).Once()
			mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-1").Return(nil, test.saved).Once()

			result, err := processor.ProcessJobs(context.Background(), jobs)

			require.NoError(t, err)
			files, globErr := filepath.Glob(filepath.Join(home, ".ahasend", "*.json"))
			require.NoError(t, globErr)
			require.Len(t, files, 1, "the file that could not be written is removed")
			assert.True(t, strings.HasPrefix(filepath.Base(files[0]), test.wantPrefix), files[0])
			if test.wantPrefix == "unknown-" {
				assert.Equal(t, files[0], result.UnknownRecipientsFile)
				assert.NoError(t, result.UnknownSaveError)
				assert.Empty(t, result.FailedRecipientsFile)
				assert.ErrorContains(t, result.FailedSaveError, "failed to write the failed recipients: ")
			} else {
				assert.Equal(t, files[0], result.FailedRecipientsFile)
				assert.NoError(t, result.FailedSaveError)
				assert.Empty(t, result.UnknownRecipientsFile)
				assert.ErrorContains(t, result.UnknownSaveError, "failed to write the unknown recipients: ")
			}
		})
	}
}

// TestBatchProcessor_StatsCountUnknownOutcomesApart checks that the
// recipients of a request of unknown outcome count in the stats as unknown,
// not as failed.
func TestBatchProcessor_StatsCountUnknownOutcomesApart(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 0, progress.NewReporter(3, false, false, io.Discard))
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(mocks.NewMessageResponse("msg"), nil).Once()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-1").
		Return(nil, &api.APIError{StatusCode: 504, Message: "gateway timeout"}).Once()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-2").
		Return(nil, &api.APIError{StatusCode: 400, Message: "bad request"}).Once()

	result, err := processor.ProcessJobs(context.Background(), templateJobs(3))

	require.NoError(t, err)
	assert.Equal(t, 1, result.Stats.Sent)
	assert.Equal(t, 1, result.Stats.Failed)
	assert.Equal(t, 1, result.Stats.Unknown)
}

// rateLimitedError is the API's 429 answer asking to retry after retryAfter
// seconds.
func rateLimitedError(retryAfter int) error {
	return &api.APIError{StatusCode: 429, Type: api.ErrorTypeRateLimit, RetryAfter: retryAfter, Message: "rate limit exceeded"}
}

func TestBatchProcessor_FailsARequestTheAPIAsksToRetryTooLate(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 3, nil)
	var notices bytes.Buffer
	processor.Notices = &notices
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, rateLimitedError(3600)).Once()

	result, err := processor.ProcessJobs(context.Background(), templateJobs(1))

	require.NoError(t, err)
	assert.Equal(t, OutcomeFailed, result.Results[0].Outcome)
	assert.EqualError(t, result.Results[0].Error,
		"the API asked to retry after 3600 seconds, more than the 300 a send waits: rate_limit error (HTTP 429): rate limit exceeded")
	assert.Equal(t, 8, clierrors.GetExitCode(result.Results[0].Error), "the API's 429 sets the exit code")
	require.Len(t, result.FailedRecipients, 1, "the recipient can be sent again later")
	assert.Empty(t, notices.String())
	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_WaitsTheLongestRetryAfterItAllows(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 3, nil)
	var notices bytes.Buffer
	processor.Notices = &notices
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, rateLimitedError(300)).Once()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(mocks.NewMessageResponse("msg"), nil).Once()

	result, err := processor.ProcessJobs(context.Background(), templateJobs(1))

	require.NoError(t, err)
	assert.Equal(t, OutcomeSent, result.Results[0].Outcome)
	assert.Equal(t, "Waiting 300 seconds, as the API asked, before sending the request of 1 recipient again\n", notices.String())
	mockClient.AssertExpectations(t)
}

// unavailableError is the API's 503 answer, which asks to retry after
// retryAfter seconds.
func unavailableError(retryAfter int) error {
	return &api.APIError{StatusCode: 503, Type: api.ErrorTypeServer, RetryAfter: retryAfter, Message: "maintenance"}
}

// TestBatchProcessor_WaitsTheRetryAfterOfA503 checks that a send waits the
// Retry-After of a 503, as of a 429, up to the same cap.
func TestBatchProcessor_WaitsTheRetryAfterOfA503(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 3, nil)
	var notices bytes.Buffer
	processor.Notices = &notices
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, unavailableError(120)).Once()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(mocks.NewMessageResponse("msg"), nil).Once()

	result, err := processor.ProcessJobs(context.Background(), templateJobs(1))

	require.NoError(t, err)
	assert.Equal(t, OutcomeSent, result.Results[0].Outcome)
	assert.Equal(t, "Waiting 120 seconds, as the API asked, before sending the request of 1 recipient again\n", notices.String())
	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_FailsA503TheAPIAsksToRetryTooLate(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 3, nil)
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, unavailableError(301)).Once()

	result, err := processor.ProcessJobs(context.Background(), templateJobs(1))

	require.NoError(t, err)
	assert.EqualError(t, result.Results[0].Error,
		"the API asked to retry after 301 seconds, more than the 300 a send waits: server error (HTTP 503): maintenance")
	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_DoesNotAnnounceAShortWait(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 3, nil)
	var notices bytes.Buffer
	processor.Notices = &notices
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, rateLimitedError(5)).Once()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(mocks.NewMessageResponse("msg"), nil).Once()

	_, err := processor.ProcessJobs(context.Background(), templateJobs(1))

	require.NoError(t, err)
	assert.Empty(t, notices.String())
}

func TestRetryDelay_WaitsTheAPIsRetryAfterOrAFallbackUpToItsCap(t *testing.T) {
	delay, askedByAPI := retryDelay(rateLimitedError(40), 1)
	assert.Equal(t, 40*retryDelayUnit, delay)
	assert.True(t, askedByAPI)

	unavailable := &api.APIError{StatusCode: 503, Message: "unavailable"}
	delay, askedByAPI = retryDelay(unavailable, 3)
	assertFallbackDelay(t, 3, delay)
	assert.False(t, askedByAPI)

	delay, _ = retryDelay(unavailable, 400)
	assertFallbackDelay(t, 30, delay)

	delay, _ = retryDelay(unavailable, 30)
	assertFallbackDelay(t, 30, delay)
	delay, _ = retryDelay(unavailable, 29)
	assertFallbackDelay(t, 29, delay)
}

// assertFallbackDelay checks that delay is a fallback wait of seconds and
// its random part, under a second.
func assertFallbackDelay(t *testing.T, seconds int, delay time.Duration) {
	t.Helper()
	assert.GreaterOrEqual(t, delay, time.Duration(seconds)*retryDelayUnit)
	assert.Less(t, delay, time.Duration(seconds+1)*retryDelayUnit)
}

// TestRetryDelay_SpreadsTheFallbackWaitsOfRequestsThatFailedTogether checks
// that the workers whose requests failed at once without a Retry-After do
// not all retry at the same moment.
func TestRetryDelay_SpreadsTheFallbackWaitsOfRequestsThatFailedTogether(t *testing.T) {
	rateLimited := &api.APIError{StatusCode: 429, Message: "Too many requests, rate limited."}
	delays := map[time.Duration]bool{}
	for range 10 {
		delay, _ := retryDelay(rateLimited, 1)
		assertFallbackDelay(t, 1, delay)
		delays[delay] = true
	}

	assert.Greater(t, len(delays), 1, "ten workers retry at %d moments", len(delays))
}

// TestBatchProcessor_AnnouncesALongFallbackWaitWithoutTheAPI checks that a
// wait the CLI chose, without a Retry-After, is announced once its random
// part takes it over 5 seconds, with that part, without saying the API asked
// for it, and never fails as a Retry-After that is too long.
func TestBatchProcessor_AnnouncesALongFallbackWaitWithoutTheAPI(t *testing.T) {
	testutil.UseNewHome(t)
	shortenRetryDelays(t)
	previousJitter := retryJitter
	retryJitter = func(limit time.Duration) time.Duration { return limit * 4 / 10 }
	t.Cleanup(func() { retryJitter = previousJitter })
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 10, nil)
	var notices bytes.Buffer
	processor.Notices = &notices
	unavailable := &api.APIError{StatusCode: 503, Message: "unavailable"}
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, unavailable).Times(5)
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(mocks.NewMessageResponse("msg"), nil).Once()

	result, err := processor.ProcessJobs(context.Background(), templateJobs(1))

	require.NoError(t, err)
	assert.Equal(t, OutcomeSent, result.Results[0].Outcome)
	assert.Equal(t, "Waiting 5.4 seconds before sending the request of 1 recipient again\n", notices.String())
	mockClient.AssertExpectations(t)
}

// TestBatchProcessor_StopsTheSendAfterARequestTooLargeForTheAPI checks that
// a request the API refuses with 413 is not retried, and that the requests
// after it, which carry the same attachments, are not sent: their
// recipients are not sent, not retryable, and the send is not interrupted.
func TestBatchProcessor_StopsTheSendAfterARequestTooLargeForTheAPI(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	tooLarge := &api.APIError{StatusCode: 413, Message: "Request Entity Too Large"}
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, tooLarge).Once()
	jobs := templateJobs(3)
	for _, job := range jobs {
		job.TemplateRequest.Attachments = []common.Attachment{{FileName: "a.pdf", Data: "JVBERg==", Base64: true}}
	}

	result, err := NewBatchProcessor(mockClient, 1, 3, nil).ProcessJobs(context.Background(), jobs)

	require.NoError(t, err)
	assert.Equal(t, []Outcome{OutcomeFailed, OutcomeNotSent, OutcomeNotSent},
		[]Outcome{result.Results[0].Outcome, result.Results[1].Outcome, result.Results[2].Outcome})
	assert.True(t, result.TooLarge)
	assert.False(t, result.Interrupted)
	require.Len(t, result.NotSentRecipients, 2)
	assert.Equal(t, "not sent: an earlier request of the send was too large for the API (HTTP 413)", result.NotSentRecipients[0].Error)
	assert.False(t, result.NotSentRecipients[0].Retryable)
	assert.NotEmpty(t, result.FailedRecipientsFile)
	mockClient.AssertExpectations(t)
}

// TestBatchProcessor_SendsTheRestAfterARequestWithoutAttachmentsTooLarge
// checks that a request without attachments the API refuses with 413 fails
// alone: the requests after it hold other recipients, so they are sent.
func TestBatchProcessor_SendsTheRestAfterARequestWithoutAttachmentsTooLarge(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	tooLarge := &api.APIError{StatusCode: 413, Message: "Request Entity Too Large"}
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, "key-0").Return(nil, tooLarge).Once()
	for _, key := range []string{"key-1", "key-2"} {
		mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, key).Return(mocks.NewMessageResponse("msg"), nil).Once()
	}

	result, err := NewBatchProcessor(mockClient, 1, 3, nil).ProcessJobs(context.Background(), templateJobs(3))

	require.NoError(t, err)
	assert.Equal(t, []Outcome{OutcomeFailed, OutcomeSent, OutcomeSent},
		[]Outcome{result.Results[0].Outcome, result.Results[1].Outcome, result.Results[2].Outcome})
	assert.False(t, result.TooLarge)
	assert.Empty(t, result.NotSentRecipients)
	mockClient.AssertExpectations(t)
}

// countOutcomes returns how many jobs of result had outcome.
func countOutcomes(result *BatchResult, outcome Outcome) int {
	count := 0
	for _, jobResult := range result.Results {
		if jobResult.Outcome == outcome {
			count++
		}
	}
	return count
}
