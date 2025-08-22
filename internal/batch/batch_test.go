package batch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/progress"
)

func TestNewBatchProcessor(t *testing.T) {
	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(10, false, false)

	processor := NewBatchProcessor(mockClient, 5, 3, progressReporter)

	assert.NotNil(t, processor)
	assert.Equal(t, mockClient, processor.client)
	assert.Equal(t, 5, processor.maxConcurrency)
	assert.Equal(t, 3, processor.maxRetries)
	assert.Equal(t, progressReporter, processor.progressReporter)
}

func TestSendJob_Structure(t *testing.T) {
	recipient := common.Recipient{
		Email: "test@example.com",
		Name:  stringPtr("Test User"),
	}

	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: "sender@example.com"},
		Recipients: []common.Recipient{recipient},
		Subject:    "Test Subject",
	}

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "test-key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{recipient},
		RecipientCount: 1,
	}

	assert.Equal(t, request, job.Request)
	assert.Equal(t, "test-key-123", job.IdempotencyKey)
	assert.Equal(t, 0, job.BatchIndex)
	assert.Len(t, job.Recipients, 1)
	assert.Equal(t, 1, job.RecipientCount)
	assert.Equal(t, "test@example.com", job.Recipients[0].Email)
	assert.Equal(t, "Test User", *job.Recipients[0].Name)
}

func TestBatchProcessor_ProcessJobs_EmptyJobs(t *testing.T) {
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 2, 1, nil)

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.TotalJobs)
	assert.Equal(t, 0, result.SuccessfulJobs)
	assert.Equal(t, 0, result.FailedJobs)
	assert.Empty(t, result.FailedRecipients)
}

func TestBatchProcessor_ProcessJobs_SingleJobSuccess(t *testing.T) {
	// Clean up any existing test files
	t.Cleanup(func() {
		os.RemoveAll(".ahasend")
	})

	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(1, false, false)
	processor := NewBatchProcessor(mockClient, 1, 1, progressReporter)

	// Set up mock expectations
	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: "sender@example.com"},
		Recipients: []common.Recipient{{Email: "test@example.com"}},
		Subject:    "Test Subject",
	}

	expectedResponse := mockClient.NewMockMessageResponse("msg-123")
	mockClient.On("SendMessageWithIdempotencyKey", *request, "key-123").Return(expectedResponse, nil)

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{{Email: "test@example.com"}},
		RecipientCount: 1,
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{job})

	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalJobs)
	assert.Equal(t, 1, result.SuccessfulJobs)
	assert.Equal(t, 0, result.FailedJobs)
	assert.Empty(t, result.FailedRecipients)
	assert.Empty(t, result.FailedRecipientsFile)

	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_ProcessJobs_SingleJobFailure(t *testing.T) {
	// Clean up any existing test files
	t.Cleanup(func() {
		os.RemoveAll(".ahasend")
	})

	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(1, false, false)
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
		RecipientCount: 1,
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{job})

	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalJobs)
	assert.Equal(t, 0, result.SuccessfulJobs)
	assert.Equal(t, 1, result.FailedJobs)
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
	// Clean up any existing test files
	t.Cleanup(func() {
		os.RemoveAll(".ahasend")
	})

	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(3, false, false)
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
			RecipientCount: 1,
		}

		// First two succeed, third fails
		if i < 2 {
			expectedResponse := mockClient.NewMockMessageResponse(fmt.Sprintf("msg-%d", i+1))
			mockClient.On("SendMessageWithIdempotencyKey", *request, fmt.Sprintf("key-%d", i+1)).Return(expectedResponse, nil)
		} else {
			testError := errors.New("service unavailable")
			mockClient.On("SendMessageWithIdempotencyKey", *request, fmt.Sprintf("key-%d", i+1)).Return(nil, testError)
		}
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, jobs)

	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalJobs)
	assert.Equal(t, 2, result.SuccessfulJobs)
	assert.Equal(t, 1, result.FailedJobs)
	assert.Len(t, result.FailedRecipients, 1)
	assert.NotEmpty(t, result.FailedRecipientsFile)

	// Check that the failed recipient is the third one
	failedRecipient := result.FailedRecipients[0]
	assert.Equal(t, "test3@example.com", failedRecipient.Email)
	assert.True(t, failedRecipient.Retryable) // "service unavailable" is retryable

	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_ProcessJobs_ContextCancellation(t *testing.T) {
	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 1, nil)

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Set up a slow mock response
	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: "sender@example.com"},
		Recipients: []common.Recipient{{Email: "test@example.com"}},
		Subject:    "Test Subject",
	}

	mockClient.On("SendMessageWithIdempotencyKey", *request, "key-123").Return(nil, context.Canceled)

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{{Email: "test@example.com"}},
		RecipientCount: 1,
	}

	// Cancel context immediately
	cancel()

	result, err := processor.ProcessJobs(ctx, []*SendJob{job})

	require.NoError(t, err)
	assert.NotNil(t, result)
	// Results may vary based on timing, but should not hang
}

func TestBatchProcessor_RetryLogic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping retry logic test in short mode due to timing")
	}

	// Clean up any existing test files
	t.Cleanup(func() {
		os.RemoveAll(".ahasend")
	})

	mockClient := &mocks.MockClient{}
	processor := NewBatchProcessor(mockClient, 1, 1, nil) // 1 retry for 2 total attempts

	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: "sender@example.com"},
		Recipients: []common.Recipient{{Email: "test@example.com"}},
		Subject:    "Test Subject",
	}

	// First call fails with retryable error, second succeeds
	retryableError := errors.New("rate limit exceeded")
	successResponse := mockClient.NewMockMessageResponse("msg-123")

	// Use mock.Anything for the request to avoid comparison issues
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, "key-123").Return(nil, retryableError).Once()
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, "key-123").Return(successResponse, nil).Once()

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{{Email: "test@example.com"}},
		RecipientCount: 1,
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{job})

	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalJobs)
	assert.Equal(t, 1, result.SuccessfulJobs)
	assert.Equal(t, 0, result.FailedJobs)

	mockClient.AssertExpectations(t)
}

func TestBatchProcessor_NonRetryableError(t *testing.T) {
	// Clean up any existing test files
	t.Cleanup(func() {
		os.RemoveAll(".ahasend")
	})

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
		RecipientCount: 1,
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, []*SendJob{job})

	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalJobs)
	assert.Equal(t, 0, result.SuccessfulJobs)
	assert.Equal(t, 1, result.FailedJobs)
	assert.Len(t, result.FailedRecipients, 1)
	assert.False(t, result.FailedRecipients[0].Retryable)

	mockClient.AssertExpectations(t)
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "rate limit error",
			err:       errors.New("Rate limit exceeded"),
			retryable: true,
		},
		{
			name:      "too many requests",
			err:       errors.New("Too many requests"),
			retryable: true,
		},
		{
			name:      "timeout error",
			err:       errors.New("Request timeout"),
			retryable: true,
		},
		{
			name:      "connection error",
			err:       errors.New("Connection failed"),
			retryable: true,
		},
		{
			name:      "network error",
			err:       errors.New("Network unreachable"),
			retryable: true,
		},
		{
			name:      "internal server error",
			err:       errors.New("Internal server error"),
			retryable: true,
		},
		{
			name:      "bad gateway",
			err:       errors.New("Bad gateway"),
			retryable: true,
		},
		{
			name:      "service unavailable",
			err:       errors.New("Service unavailable"),
			retryable: true,
		},
		{
			name:      "gateway timeout",
			err:       errors.New("Gateway timeout"),
			retryable: true,
		},
		{
			name:      "HTTP 429",
			err:       errors.New("HTTP 429 Rate Limited"),
			retryable: true,
		},
		{
			name:      "HTTP 502",
			err:       errors.New("HTTP 502 Bad Gateway"),
			retryable: true,
		},
		{
			name:      "HTTP 503",
			err:       errors.New("HTTP 503 Service Unavailable"),
			retryable: true,
		},
		{
			name:      "HTTP 504",
			err:       errors.New("HTTP 504 Gateway Timeout"),
			retryable: true,
		},
		{
			name:      "invalid email",
			err:       errors.New("Invalid email address"),
			retryable: false,
		},
		{
			name:      "authorization error",
			err:       errors.New("Unauthorized"),
			retryable: false,
		},
		{
			name:      "not found",
			err:       errors.New("Not found"),
			retryable: false,
		},
		{
			name:      "generic error",
			err:       errors.New("Something went wrong"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.retryable, result)
		})
	}
}

func TestExtractActualErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "Unknown error",
		},
		{
			name:     "simple error",
			err:      errors.New("simple error message"),
			expected: "simple error message",
		},
		{
			name:     "SDK parsing error",
			err:      errors.New("no value given for required property message"),
			expected: "API parsing error (check debug logs for actual response): no value given for required property message",
		},
		{
			name:     "generic error",
			err:      errors.New("unexpected error"),
			expected: "unexpected error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractActualErrorMessage(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: 0,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractErrorCode(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateFailedRecipientFromError(t *testing.T) {
	processor := NewBatchProcessor(&mocks.MockClient{}, 1, 1, nil)

	recipient := common.Recipient{
		Email: "test@example.com",
		Name:  stringPtr("Test User"),
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

	// Check CSV fields
	assert.Equal(t, "John", failedRecipient.CSVFields["first_name"])
	assert.Equal(t, "25", failedRecipient.CSVFields["age"])
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
	assert.NotNil(t, failedRecipient.CSVFields)
}

func TestSaveFailedRecipients(t *testing.T) {
	// Clean up any existing test files
	t.Cleanup(func() {
		os.RemoveAll(".ahasend")
	})

	processor := NewBatchProcessor(&mocks.MockClient{}, 1, 1, nil)

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

	filename, err := processor.saveFailedRecipients(failedRecipients)

	require.NoError(t, err)
	assert.NotEmpty(t, filename)
	assert.Contains(t, filename, ".ahasend/failed-")
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

func TestBatchResult_GetExitCode(t *testing.T) {
	tests := []struct {
		name     string
		result   *BatchResult
		expected int
	}{
		{
			name: "no jobs",
			result: &BatchResult{
				TotalJobs:      0,
				SuccessfulJobs: 0,
				FailedJobs:     0,
			},
			expected: 3, // Critical error
		},
		{
			name: "all success",
			result: &BatchResult{
				TotalJobs:      5,
				SuccessfulJobs: 5,
				FailedJobs:     0,
			},
			expected: 0, // Success
		},
		{
			name: "all failed",
			result: &BatchResult{
				TotalJobs:      5,
				SuccessfulJobs: 0,
				FailedJobs:     5,
			},
			expected: 2, // All failed
		},
		{
			name: "partial success",
			result: &BatchResult{
				TotalJobs:      5,
				SuccessfulJobs: 3,
				FailedJobs:     2,
			},
			expected: 1, // Partial success
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode := tt.result.GetExitCode()
			assert.Equal(t, tt.expected, exitCode)
		})
	}
}

func TestBatchProcessor_ConcurrencyLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	// Clean up any existing test files
	t.Cleanup(func() {
		os.RemoveAll(".ahasend")
	})

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
			RecipientCount: 1,
		}

		// All succeed but with a small delay to test concurrency
		expectedResponse := mockClient.NewMockMessageResponse(fmt.Sprintf("msg-%d", i+1))
		mockClient.On("SendMessageWithIdempotencyKey", *request, fmt.Sprintf("key-%d", i+1)).Return(expectedResponse, nil).Maybe()
	}

	ctx := context.Background()
	start := time.Now()
	result, err := processor.ProcessJobs(ctx, jobs)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 5, result.TotalJobs)
	assert.Equal(t, 5, result.SuccessfulJobs)
	assert.Equal(t, 0, result.FailedJobs)

	// With concurrency of 2, this should complete faster than sequential
	// This is a rough test - actual timing depends on system load
	t.Logf("Batch processing took %v with concurrency limit 2", duration)

	// We just verify it completed without hanging
	assert.Less(t, duration, 30*time.Second, "Batch processing should not take too long")
}

func TestFailedRecipient_Structure(t *testing.T) {
	failedRecipient := FailedRecipient{
		Email:     "test@example.com",
		Name:      "Test User",
		Error:     "Rate limit exceeded",
		ErrorCode: 429,
		Retryable: true,
		SubstitutionData: map[string]interface{}{
			"first_name": "John",
			"last_name":  "Doe",
		},
		CSVFields: map[string]string{
			"first_name": "John",
			"last_name":  "Doe",
		},
	}

	assert.Equal(t, "test@example.com", failedRecipient.Email)
	assert.Equal(t, "Test User", failedRecipient.Name)
	assert.Equal(t, "Rate limit exceeded", failedRecipient.Error)
	assert.Equal(t, 429, failedRecipient.ErrorCode)
	assert.True(t, failedRecipient.Retryable)
	assert.NotNil(t, failedRecipient.SubstitutionData)
	assert.NotNil(t, failedRecipient.CSVFields)
}

func TestBatchProcessor_ProgressReporting(t *testing.T) {
	// Clean up any existing test files
	t.Cleanup(func() {
		os.RemoveAll(".ahasend")
	})

	mockClient := &mocks.MockClient{}
	progressReporter := progress.NewReporter(2, false, false) // No progress bar, just stats
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
			RecipientCount: 1,
		}

		// First succeeds, second fails
		if i == 0 {
			expectedResponse := mockClient.NewMockMessageResponse("msg-1")
			mockClient.On("SendMessageWithIdempotencyKey", *request, "key-1").Return(expectedResponse, nil)
		} else {
			testError := errors.New("API error")
			mockClient.On("SendMessageWithIdempotencyKey", *request, "key-2").Return(nil, testError)
		}
	}

	ctx := context.Background()
	result, err := processor.ProcessJobs(ctx, jobs)

	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalJobs)
	assert.Equal(t, 1, result.SuccessfulJobs)
	assert.Equal(t, 1, result.FailedJobs)

	// Check that stats were collected
	assert.NotEmpty(t, result.Stats) // Stats should be populated

	mockClient.AssertExpectations(t)
}

// Helper function
func stringPtr(s string) *string {
	return &s
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

	expectedResponse := mockClient.NewMockMessageResponse("msg-123")
	mockClient.On("SendMessageWithIdempotencyKey", *request, "key-123").Return(expectedResponse, nil).Maybe()

	job := &SendJob{
		Request:        request,
		IdempotencyKey: "key-123",
		BatchIndex:     0,
		Recipients:     []common.Recipient{{Email: "test@example.com"}},
		RecipientCount: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, _ = processor.ProcessJobs(ctx, []*SendJob{job})
	}
}

func BenchmarkIsRetryableError(b *testing.B) {
	testError := errors.New("rate limit exceeded")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isRetryableError(testError)
	}
}

func BenchmarkExtractActualErrorMessage(b *testing.B) {
	testError := errors.New("no value given for required property message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractActualErrorMessage(testError)
	}
}

func BenchmarkCreateFailedRecipient(b *testing.B) {
	processor := NewBatchProcessor(&mocks.MockClient{}, 1, 1, nil)

	recipient := common.Recipient{
		Email: "test@example.com",
		Name:  stringPtr("Test User"),
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
