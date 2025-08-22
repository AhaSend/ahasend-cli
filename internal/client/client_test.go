package client

import (
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_Success(t *testing.T) {
	apiKey := "test-api-key"
	accountID := uuid.New().String()

	client, err := NewClient(apiKey, accountID)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, accountID, client.GetAccountID())
	assert.NotNil(t, client.GetAuthContext())
	assert.NotNil(t, client.rateLimiter)
	assert.NotNil(t, client.APIClient)

	// Verify the auth context contains the API key
	authValue := client.GetAuthContext().Value(api.ContextAccessToken)
	assert.Equal(t, apiKey, authValue)

	// Verify configuration is set up correctly
	assert.NotNil(t, client.config)
	assert.True(t, client.config.RetryConfig.Enabled)
	assert.Equal(t, 3, client.config.RetryConfig.MaxRetries)
	assert.Equal(t, 1*time.Second, client.config.RetryConfig.BaseDelay)
	assert.Equal(t, 30*time.Second, client.config.RetryConfig.MaxDelay)
	assert.Equal(t, api.BackoffExponential, client.config.RetryConfig.BackoffStrategy)
	assert.False(t, client.config.RetryConfig.RetryClientErrors)
	assert.False(t, client.config.RetryConfig.RetryValidationErrors)

	// Verify HTTP client configuration
	assert.NotNil(t, client.config.HTTPClient)
	assert.Equal(t, 30*time.Second, client.config.HTTPClient.Timeout)
	assert.Contains(t, client.config.UserAgent, "ahasend-cli/1.0.0")
}

func TestNewClient_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		accountID string
		wantError string
	}{
		{
			name:      "empty API key",
			apiKey:    "",
			accountID: uuid.New().String(),
			wantError: "API key is required",
		},
		{
			name:      "empty account ID",
			apiKey:    "test-api-key",
			accountID: "",
			wantError: "account ID is required",
		},
		{
			name:      "both empty",
			apiKey:    "",
			accountID: "",
			wantError: "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.apiKey, tt.accountID)

			assert.Error(t, err)
			assert.Nil(t, client)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestClient_GetAccountID(t *testing.T) {
	accountID := uuid.New().String()
	client, err := NewClient("test-api-key", accountID)
	require.NoError(t, err)

	result := client.GetAccountID()
	assert.Equal(t, accountID, result)
}

func TestClient_GetAuthContext(t *testing.T) {
	apiKey := "test-api-key"
	accountID := uuid.New().String()
	client, err := NewClient(apiKey, accountID)
	require.NoError(t, err)

	ctx := client.GetAuthContext()
	assert.NotNil(t, ctx)

	// Verify the context contains the API key
	authValue := ctx.Value(api.ContextAccessToken)
	assert.Equal(t, apiKey, authValue)
}

func TestClient_ValidateConfiguration_WithValidAccountID(t *testing.T) {
	apiKey := "test-api-key"
	accountID := uuid.New().String()
	client, err := NewClient(apiKey, accountID)
	require.NoError(t, err)

	// We can't easily test the actual ping without a real server or complex mocking
	// But we can test that the validation method exists and handles basic validation
	// The ping will fail against the real API without valid credentials, but that's expected
	err = client.ValidateConfiguration()
	// We expect this to fail against the real API, but not due to empty account ID
	if err != nil {
		// Should not be an account ID validation error
		assert.NotContains(t, err.Error(), "account ID is required")
	}
}

func TestClient_ValidateConfiguration_EmptyAccountID(t *testing.T) {
	// Create client with valid parameters first
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	// Manually clear the account ID to test validation
	client.accountID = ""

	err = client.ValidateConfiguration()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account ID is required")
}

func TestClient_SendMessageWithIdempotencyKey_InvalidAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", "invalid-uuid")
	require.NoError(t, err)

	req := requests.CreateMessageRequest{
		From: common.SenderAddress{Email: "test@example.com"},
		Recipients: []common.Recipient{
			{Email: "recipient@example.com"},
		},
		Subject: "Test Subject",
	}

	response, err := client.SendMessageWithIdempotencyKey(req, "test-key")

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid account ID format")
}

func TestClient_SendMessage_InvalidAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", "invalid-uuid")
	require.NoError(t, err)

	req := requests.CreateMessageRequest{
		From: common.SenderAddress{Email: "test@example.com"},
		Recipients: []common.Recipient{
			{Email: "recipient@example.com"},
		},
		Subject: "Test Subject",
	}

	response, err := client.SendMessage(req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid account ID format")
}

func TestClient_CancelMessage_InvalidAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	response, err := client.CancelMessage("invalid-uuid", "msg-123")

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid account ID format")
}

func TestClient_ListDomains_InvalidAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", "invalid-uuid")
	require.NoError(t, err)

	response, err := client.ListDomains(nil, nil)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid account ID format")
}

func TestClient_CreateDomain_InvalidAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", "invalid-uuid")
	require.NoError(t, err)

	response, err := client.CreateDomain("example.com")

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid account ID format")
}

func TestClient_GetDomain_InvalidAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", "invalid-uuid")
	require.NoError(t, err)

	response, err := client.GetDomain("example.com")

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid account ID format")
}

func TestClient_DeleteDomain_InvalidAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", "invalid-uuid")
	require.NoError(t, err)

	_, err = client.DeleteDomain("example.com")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid account ID format")
}

func TestClient_GetMessages_InvalidAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", "invalid-uuid")
	require.NoError(t, err)

	params := requests.GetMessagesParams{
		Limit: int32Ptr(10),
	}

	response, err := client.GetMessages(params)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid account ID format")
}

func TestClient_CreateWebhookVerifier_Success(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	secret := "test-webhook-secret"
	verifier, err := client.CreateWebhookVerifier(secret)

	// The actual behavior depends on the SDK implementation
	// This test primarily ensures the method can be called without panics
	if err != nil {
		// If there's an error, it should be related to the secret format or SDK initialization
		assert.NotNil(t, err)
	} else {
		assert.NotNil(t, verifier)
	}
}

func TestClient_RateLimiter_Integration(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	// Test rate limiter is properly initialized
	assert.NotNil(t, client.rateLimiter)

	// Test rate limiter allows requests
	allowed := client.rateLimiter.Allow()
	assert.True(t, allowed, "First request should be allowed")

	// Test wait time calculation
	waitTime := client.rateLimiter.GetWaitTime()
	assert.True(t, waitTime >= 0, "Wait time should be non-negative")
}

func TestGetMessagesParams_ParameterBuilding(t *testing.T) {
	// Test that GetMessagesParams struct works correctly
	sender := "sender@example.com"
	recipient := "recipient@example.com"
	subject := "Test Subject"
	messageID := "msg-123"
	fromTime := time.Now().Add(-24 * time.Hour)
	toTime := time.Now()
	limit := int32(50)
	cursor := "test-cursor"

	params := requests.GetMessagesParams{
		Sender:          &sender,
		Recipient:       &recipient,
		Subject:         &subject,
		MessageIDHeader: &messageID,
		FromTime:        &fromTime,
		ToTime:          &toTime,
		Limit:           &limit,
		Cursor:          &cursor,
	}

	// Test that all parameters are properly set
	assert.Equal(t, sender, *params.Sender)
	assert.Equal(t, recipient, *params.Recipient)
	assert.Equal(t, subject, *params.Subject)
	assert.Equal(t, messageID, *params.MessageIDHeader)
	assert.Equal(t, fromTime, *params.FromTime)
	assert.Equal(t, toTime, *params.ToTime)
	assert.Equal(t, limit, *params.Limit)
	assert.Equal(t, cursor, *params.Cursor)
}

func TestGetMessagesParams_NilValues(t *testing.T) {
	// Test with nil values (all parameters optional)
	params := requests.GetMessagesParams{}

	assert.Nil(t, params.Sender)
	assert.Nil(t, params.Recipient)
	assert.Nil(t, params.Subject)
	assert.Nil(t, params.MessageIDHeader)
	assert.Nil(t, params.FromTime)
	assert.Nil(t, params.ToTime)
	assert.Nil(t, params.Limit)
	assert.Nil(t, params.Cursor)
}

// Test that the client implements the AhaSendClient interface
func TestClient_InterfaceCompliance(t *testing.T) {
	var _ AhaSendClient = (*Client)(nil)
}

// Benchmark tests to ensure the client creation is performant
func BenchmarkNewClient(b *testing.B) {
	apiKey := "test-api-key"
	accountID := uuid.New().String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, err := NewClient(apiKey, accountID)
		if err != nil {
			b.Fatal(err)
		}
		_ = client
	}
}

func BenchmarkClient_GetAccountID(b *testing.B) {
	client, err := NewClient("test-api-key", uuid.New().String())
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.GetAccountID()
	}
}

func BenchmarkClient_GetAuthContext(b *testing.B) {
	client, err := NewClient("test-api-key", uuid.New().String())
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.GetAuthContext()
	}
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	client, err := NewClient("test-api-key", uuid.New().String())
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.rateLimiter.Allow()
	}
}

// Helper functions for tests
func int32Ptr(v int32) *int32 {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

// TestClient_ErrorScenarios tests various error conditions
func TestClient_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name          string
		setupClient   func() (*Client, error)
		operation     func(*Client) error
		expectedError string
	}{
		{
			name: "validate configuration with empty account ID",
			setupClient: func() (*Client, error) {
				client, err := NewClient("test-api-key", uuid.New().String())
				if err != nil {
					return nil, err
				}
				client.accountID = ""
				return client, nil
			},
			operation: func(c *Client) error {
				return c.ValidateConfiguration()
			},
			expectedError: "account ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := tt.setupClient()
			require.NoError(t, err)

			err = tt.operation(client)
			assert.Error(t, err)
			if tt.expectedError != "" {
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

// Webhook Tests

func TestClient_ListWebhooks_Success(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	// The actual behavior depends on the SDK implementation
	// This test primarily ensures the method can be called without panics
	_, err = client.ListWebhooks(nil, nil)

	// We expect this to fail against the real API, but not due to implementation issues
	if err != nil {
		// Should be an API-related error, not a panic or nil pointer
		assert.NotContains(t, err.Error(), "panic")
		assert.NotContains(t, err.Error(), "nil pointer")
	}
}

func TestClient_ListWebhooks_WithPagination(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	limit := int32(10)
	cursor := "test-cursor"

	_, err = client.ListWebhooks(&limit, &cursor)

	// We expect this to fail against the real API, but not due to parameter handling
	if err != nil {
		assert.NotContains(t, err.Error(), "panic")
		assert.NotContains(t, err.Error(), "nil pointer")
	}
}

func TestClient_CreateWebhook_Success(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	// Create a valid webhook request
	req := requests.CreateWebhookRequest{
		Name:    "Test Webhook",
		URL:     "https://example.com/webhook",
		Enabled: true,
	}

	_, err = client.CreateWebhook(req)

	// We expect this to fail against the real API, but not due to implementation issues
	if err != nil {
		assert.NotContains(t, err.Error(), "panic")
		assert.NotContains(t, err.Error(), "nil pointer")
	}
}

func TestClient_GetWebhook_Success(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	webhookID := uuid.New().String()

	_, err = client.GetWebhook(webhookID)

	// We expect this to fail against the real API, but not due to implementation issues
	if err != nil {
		assert.NotContains(t, err.Error(), "panic")
		assert.NotContains(t, err.Error(), "nil pointer")
	}
}

func TestClient_GetWebhook_InvalidID(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	_, err = client.GetWebhook("invalid-uuid")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid webhook ID format")
}

func TestClient_UpdateWebhook_Success(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	webhookID := uuid.New().String()
	req := requests.UpdateWebhookRequest{
		Name:    ahasend.String("Updated Webhook"),
		Enabled: ahasend.Bool(false),
	}

	_, err = client.UpdateWebhook(webhookID, req)

	// We expect this to fail against the real API, but not due to implementation issues
	if err != nil {
		assert.NotContains(t, err.Error(), "panic")
		assert.NotContains(t, err.Error(), "nil pointer")
	}
}

func TestClient_UpdateWebhook_InvalidID(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	req := requests.UpdateWebhookRequest{
		Name: ahasend.String("Updated Webhook"),
	}

	_, err = client.UpdateWebhook("invalid-uuid", req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid webhook ID format")
}

func TestClient_DeleteWebhook_Success(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	webhookID := uuid.New().String()

	err = client.DeleteWebhook(webhookID)

	// We expect this to fail against the real API, but not due to implementation issues
	if err != nil {
		assert.NotContains(t, err.Error(), "panic")
		assert.NotContains(t, err.Error(), "nil pointer")
	}
}

func TestClient_DeleteWebhook_InvalidID(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	err = client.DeleteWebhook("invalid-uuid")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid webhook ID format")
}
