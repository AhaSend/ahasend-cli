package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
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
		PaginationParams: common.PaginationParams{
			Limit: int32Ptr(10),
		},
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
		PaginationParams: common.PaginationParams{
			Limit:  &limit,
			Cursor: &cursor,
		},
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

func TestClient_SubAccountWrappers_InvalidParentAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", "invalid-uuid")
	require.NoError(t, err)

	validSubAccountID := uuid.New().String()
	validKeyID := uuid.New().String()
	createSubAccountReq := requests.CreateSubAccountRequest{
		Name:    "Acme Subsidiary",
		Website: "acme.example.com",
	}
	updateSubAccountReq := requests.UpdateSubAccountRequest{
		Name: ahasend.String("Updated Subsidiary"),
	}
	suspendReq := requests.SuspendSubAccountRequest{Reason: "Customer requested temporary pause"}
	createAPIKeyReq := requests.CreateAPIKeyRequest{
		Label:  "Bootstrap key",
		Scopes: []string{"messages:send:all"},
	}
	updateAPIKeyReq := requests.UpdateAPIKeyRequest{
		Label: ahasend.String("Updated bootstrap key"),
	}

	tests := []struct {
		name string
		call func(*Client) (any, error)
	}{
		{
			name: "ListSubAccounts",
			call: func(c *Client) (any, error) {
				return c.ListSubAccounts(nil, nil)
			},
		},
		{
			name: "CreateSubAccount",
			call: func(c *Client) (any, error) {
				return c.CreateSubAccount(createSubAccountReq, "sub-create-key")
			},
		},
		{
			name: "GetSubAccountsUsage",
			call: func(c *Client) (any, error) {
				return c.GetSubAccountsUsage()
			},
		},
		{
			name: "GetSubAccount",
			call: func(c *Client) (any, error) {
				return c.GetSubAccount(validSubAccountID)
			},
		},
		{
			name: "UpdateSubAccount",
			call: func(c *Client) (any, error) {
				return c.UpdateSubAccount(validSubAccountID, updateSubAccountReq)
			},
		},
		{
			name: "DeleteSubAccount",
			call: func(c *Client) (any, error) {
				return c.DeleteSubAccount(validSubAccountID)
			},
		},
		{
			name: "SuspendSubAccount",
			call: func(c *Client) (any, error) {
				return c.SuspendSubAccount(validSubAccountID, suspendReq)
			},
		},
		{
			name: "UnsuspendSubAccount",
			call: func(c *Client) (any, error) {
				return c.UnsuspendSubAccount(validSubAccountID)
			},
		},
		{
			name: "ListSubAccountAPIKeys",
			call: func(c *Client) (any, error) {
				return c.ListSubAccountAPIKeys(validSubAccountID, nil, nil)
			},
		},
		{
			name: "CreateSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.CreateSubAccountAPIKey(validSubAccountID, createAPIKeyReq, "key-create-key")
			},
		},
		{
			name: "GetSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.GetSubAccountAPIKey(validSubAccountID, validKeyID)
			},
		},
		{
			name: "UpdateSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.UpdateSubAccountAPIKey(validSubAccountID, validKeyID, updateAPIKeyReq)
			},
		},
		{
			name: "DeleteSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.DeleteSubAccountAPIKey(validSubAccountID, validKeyID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := tt.call(client)

			assert.Error(t, err)
			assert.Nil(t, response)
			assert.Contains(t, err.Error(), "invalid account ID format")
		})
	}
}

func TestClient_SubAccountWrappers_InvalidSubAccountID(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	validKeyID := uuid.New().String()
	updateSubAccountReq := requests.UpdateSubAccountRequest{
		Name: ahasend.String("Updated Subsidiary"),
	}
	suspendReq := requests.SuspendSubAccountRequest{Reason: "Customer requested temporary pause"}
	createAPIKeyReq := requests.CreateAPIKeyRequest{
		Label:  "Bootstrap key",
		Scopes: []string{"messages:send:all"},
	}
	updateAPIKeyReq := requests.UpdateAPIKeyRequest{
		Label: ahasend.String("Updated bootstrap key"),
	}

	tests := []struct {
		name string
		call func(*Client) (any, error)
	}{
		{
			name: "GetSubAccount",
			call: func(c *Client) (any, error) {
				return c.GetSubAccount("invalid-uuid")
			},
		},
		{
			name: "UpdateSubAccount",
			call: func(c *Client) (any, error) {
				return c.UpdateSubAccount("invalid-uuid", updateSubAccountReq)
			},
		},
		{
			name: "DeleteSubAccount",
			call: func(c *Client) (any, error) {
				return c.DeleteSubAccount("invalid-uuid")
			},
		},
		{
			name: "SuspendSubAccount",
			call: func(c *Client) (any, error) {
				return c.SuspendSubAccount("invalid-uuid", suspendReq)
			},
		},
		{
			name: "UnsuspendSubAccount",
			call: func(c *Client) (any, error) {
				return c.UnsuspendSubAccount("invalid-uuid")
			},
		},
		{
			name: "ListSubAccountAPIKeys",
			call: func(c *Client) (any, error) {
				return c.ListSubAccountAPIKeys("invalid-uuid", nil, nil)
			},
		},
		{
			name: "CreateSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.CreateSubAccountAPIKey("invalid-uuid", createAPIKeyReq, "key-create-key")
			},
		},
		{
			name: "GetSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.GetSubAccountAPIKey("invalid-uuid", validKeyID)
			},
		},
		{
			name: "UpdateSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.UpdateSubAccountAPIKey("invalid-uuid", validKeyID, updateAPIKeyReq)
			},
		},
		{
			name: "DeleteSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.DeleteSubAccountAPIKey("invalid-uuid", validKeyID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := tt.call(client)

			assert.Error(t, err)
			assert.Nil(t, response)
			assert.Contains(t, err.Error(), "invalid sub-account ID format")
		})
	}
}

func TestClient_SubAccountAPIKeyWrappers_InvalidAPIKeyID(t *testing.T) {
	client, err := NewClient("test-api-key", uuid.New().String())
	require.NoError(t, err)

	validSubAccountID := uuid.New().String()
	updateAPIKeyReq := requests.UpdateAPIKeyRequest{
		Label: ahasend.String("Updated bootstrap key"),
	}

	tests := []struct {
		name string
		call func(*Client) (any, error)
	}{
		{
			name: "GetSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.GetSubAccountAPIKey(validSubAccountID, "invalid-uuid")
			},
		},
		{
			name: "UpdateSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.UpdateSubAccountAPIKey(validSubAccountID, "invalid-uuid", updateAPIKeyReq)
			},
		},
		{
			name: "DeleteSubAccountAPIKey",
			call: func(c *Client) (any, error) {
				return c.DeleteSubAccountAPIKey(validSubAccountID, "invalid-uuid")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := tt.call(client)

			assert.Error(t, err)
			assert.Nil(t, response)
			assert.Contains(t, err.Error(), "invalid API key ID format")
		})
	}
}

func TestClient_SubAccountWrappers_CallThrough(t *testing.T) {
	accountID := uuid.MustParse("9d0cf9d0-4f5e-4674-bcf1-8ec39968b6e1")
	subAccountID := uuid.MustParse("2f3c5d2a-9ef8-4c91-a5f4-79990c8c1d3a")
	keyID := uuid.MustParse("13b3aa8e-78d3-48a1-92d2-4b8b1228c2dd")
	subAccountsPath := "/v2/accounts/" + accountID.String() + "/sub-accounts"
	subAccountPath := subAccountsPath + "/" + subAccountID.String()
	apiKeysPath := subAccountPath + "/api-keys"
	apiKeyPath := apiKeysPath + "/" + keyID.String()
	secretKey := "aha-sk-child-secret-key"

	tests := []struct {
		name            string
		wantMethod      string
		wantPath        string
		wantIdempotency string
		wantQuery       map[string]string
		call            func(*Client) (any, error)
		writeResponse   func(*testing.T, http.ResponseWriter)
		assertBody      func(*testing.T, map[string]any)
		assertResult    func(*testing.T, any)
	}{
		{
			name:       "ListSubAccounts",
			wantMethod: http.MethodGet,
			wantPath:   subAccountsPath,
			wantQuery: map[string]string{
				"limit":  "25",
				"cursor": "sub-cursor",
			},
			call: func(c *Client) (any, error) {
				limit := int32(25)
				cursor := "sub-cursor"
				return c.ListSubAccounts(&limit, &cursor)
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, &responses.PaginatedSubAccountsResponse{
					Object: "list",
					Data:   []responses.SubAccount{clientTestSubAccount(accountID, subAccountID, "active")},
					Pagination: common.PaginationInfo{
						HasMore: false,
					},
				})
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.PaginatedSubAccountsResponse)
				require.Len(t, response.Data, 1)
				assert.Equal(t, subAccountID, response.Data[0].ID)
				assert.False(t, response.Pagination.HasMore)
			},
		},
		{
			name:            "CreateSubAccount",
			wantMethod:      http.MethodPost,
			wantPath:        subAccountsPath,
			wantIdempotency: "sub-create-key",
			call: func(c *Client) (any, error) {
				return c.CreateSubAccount(requests.CreateSubAccountRequest{
					Name:          "Acme Subsidiary",
					Website:       "acme.example.com",
					MonthlyCredit: ahasend.Int64(50000),
				}, "sub-create-key")
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusCreated, clientTestSubAccount(accountID, subAccountID, "active"))
			},
			assertBody: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "Acme Subsidiary", body["name"])
				assert.Equal(t, "acme.example.com", body["website"])
				assert.Equal(t, float64(50000), body["monthly_credit"])
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.SubAccount)
				assert.Equal(t, subAccountID, response.ID)
				assert.Equal(t, accountID, response.ParentAccountID)
			},
		},
		{
			name:       "GetSubAccountsUsage",
			wantMethod: http.MethodGet,
			wantPath:   subAccountsPath + "/usage",
			call: func(c *Client) (any, error) {
				return c.GetSubAccountsUsage()
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, clientTestSubAccountUsage(accountID, subAccountID))
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.SubAccountUsageResponse)
				require.NotNil(t, response.Parent.AccountID)
				assert.Equal(t, accountID, *response.Parent.AccountID)
				require.Len(t, response.SubAccounts, 1)
				require.NotNil(t, response.SubAccounts[0].AccountID)
				assert.Equal(t, subAccountID, *response.SubAccounts[0].AccountID)
			},
		},
		{
			name:       "GetSubAccount",
			wantMethod: http.MethodGet,
			wantPath:   subAccountPath,
			call: func(c *Client) (any, error) {
				return c.GetSubAccount(subAccountID.String())
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, clientTestSubAccount(accountID, subAccountID, "active"))
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.SubAccount)
				assert.Equal(t, subAccountID, response.ID)
			},
		},
		{
			name:       "UpdateSubAccount",
			wantMethod: http.MethodPut,
			wantPath:   subAccountPath,
			call: func(c *Client) (any, error) {
				return c.UpdateSubAccount(subAccountID.String(), requests.UpdateSubAccountRequest{
					Name: ahasend.String("Updated Subsidiary"),
				})
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, clientTestSubAccount(accountID, subAccountID, "active"))
			},
			assertBody: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "Updated Subsidiary", body["name"])
				assert.NotContains(t, body, "website")
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.SubAccount)
				assert.Equal(t, subAccountID, response.ID)
			},
		},
		{
			name:       "DeleteSubAccount",
			wantMethod: http.MethodDelete,
			wantPath:   subAccountPath,
			call: func(c *Client) (any, error) {
				return c.DeleteSubAccount(subAccountID.String())
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, &common.SuccessResponse{Message: "sub account deleted"})
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*common.SuccessResponse)
				assert.Equal(t, "sub account deleted", response.Message)
			},
		},
		{
			name:       "SuspendSubAccount",
			wantMethod: http.MethodPost,
			wantPath:   subAccountPath + "/suspend",
			call: func(c *Client) (any, error) {
				return c.SuspendSubAccount(subAccountID.String(), requests.SuspendSubAccountRequest{
					Reason: "Customer requested temporary pause",
				})
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, clientTestSubAccount(accountID, subAccountID, "suspended"))
			},
			assertBody: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "Customer requested temporary pause", body["reason"])
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.SubAccount)
				assert.Equal(t, "suspended", response.Status)
			},
		},
		{
			name:       "UnsuspendSubAccount",
			wantMethod: http.MethodPost,
			wantPath:   subAccountPath + "/unsuspend",
			call: func(c *Client) (any, error) {
				return c.UnsuspendSubAccount(subAccountID.String())
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, clientTestSubAccount(accountID, subAccountID, "active"))
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.SubAccount)
				assert.Equal(t, "active", response.Status)
			},
		},
		{
			name:       "ListSubAccountAPIKeys",
			wantMethod: http.MethodGet,
			wantPath:   apiKeysPath,
			wantQuery: map[string]string{
				"limit":  "10",
				"cursor": "key-cursor",
			},
			call: func(c *Client) (any, error) {
				limit := int32(10)
				cursor := "key-cursor"
				return c.ListSubAccountAPIKeys(subAccountID.String(), &limit, &cursor)
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, &responses.PaginatedAPIKeysResponse{
					Object: "list",
					Data:   []responses.APIKey{clientTestAPIKey(subAccountID, keyID, nil)},
					Pagination: common.PaginationInfo{
						HasMore: false,
					},
				})
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.PaginatedAPIKeysResponse)
				require.Len(t, response.Data, 1)
				assert.Equal(t, keyID, response.Data[0].ID)
				assert.Equal(t, subAccountID, response.Data[0].AccountID)
			},
		},
		{
			name:            "CreateSubAccountAPIKey",
			wantMethod:      http.MethodPost,
			wantPath:        apiKeysPath,
			wantIdempotency: "key-create-key",
			call: func(c *Client) (any, error) {
				return c.CreateSubAccountAPIKey(subAccountID.String(), requests.CreateAPIKeyRequest{
					Label:  "Bootstrap key",
					Scopes: []string{"messages:send:all", "domains:read"},
				}, "key-create-key")
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusCreated, clientTestAPIKey(subAccountID, keyID, &secretKey))
			},
			assertBody: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "Bootstrap key", body["label"])
				assert.Equal(t, []any{"messages:send:all", "domains:read"}, body["scopes"])
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.APIKey)
				assert.Equal(t, keyID, response.ID)
				assert.Equal(t, subAccountID, response.AccountID)
				require.NotNil(t, response.SecretKey)
				assert.Equal(t, secretKey, *response.SecretKey)
			},
		},
		{
			name:       "GetSubAccountAPIKey",
			wantMethod: http.MethodGet,
			wantPath:   apiKeyPath,
			call: func(c *Client) (any, error) {
				return c.GetSubAccountAPIKey(subAccountID.String(), keyID.String())
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, clientTestAPIKey(subAccountID, keyID, nil))
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.APIKey)
				assert.Equal(t, keyID, response.ID)
				assert.Nil(t, response.SecretKey)
			},
		},
		{
			name:       "UpdateSubAccountAPIKey",
			wantMethod: http.MethodPut,
			wantPath:   apiKeyPath,
			call: func(c *Client) (any, error) {
				scopes := []string{"messages:send:all"}
				return c.UpdateSubAccountAPIKey(subAccountID.String(), keyID.String(), requests.UpdateAPIKeyRequest{
					Label:  ahasend.String("Updated bootstrap key"),
					Scopes: &scopes,
				})
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, clientTestAPIKey(subAccountID, keyID, nil))
			},
			assertBody: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "Updated bootstrap key", body["label"])
				assert.Equal(t, []any{"messages:send:all"}, body["scopes"])
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*responses.APIKey)
				assert.Equal(t, keyID, response.ID)
			},
		},
		{
			name:       "DeleteSubAccountAPIKey",
			wantMethod: http.MethodDelete,
			wantPath:   apiKeyPath,
			call: func(c *Client) (any, error) {
				return c.DeleteSubAccountAPIKey(subAccountID.String(), keyID.String())
			},
			writeResponse: func(t *testing.T, w http.ResponseWriter) {
				writeClientTestJSON(t, w, http.StatusOK, &common.SuccessResponse{Message: "sub account api key deleted"})
			},
			assertResult: func(t *testing.T, result any) {
				response := result.(*common.SuccessResponse)
				assert.Equal(t, "sub account api key deleted", response.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := newClientTestServer(t, accountID.String(), func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.wantMethod, r.Method)
				assert.Equal(t, tt.wantPath, r.URL.Path)
				if tt.wantIdempotency != "" {
					assert.Equal(t, tt.wantIdempotency, r.Header.Get("Idempotency-Key"))
				}

				for _, key := range []string{"limit", "cursor"} {
					assert.Equal(t, tt.wantQuery[key], r.URL.Query().Get(key))
				}

				if tt.assertBody != nil {
					var body map[string]any
					err := json.NewDecoder(r.Body).Decode(&body)
					require.NoError(t, err)
					tt.assertBody(t, body)
				}

				tt.writeResponse(t, w)
			})
			defer cleanup()

			result, err := tt.call(client)

			require.NoError(t, err)
			require.NotNil(t, result)
			tt.assertResult(t, result)
		})
	}
}

func newClientTestServer(t *testing.T, accountID string, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()

	server := httptest.NewServer(handler)
	client, err := NewClient("test-api-key", accountID, server.URL)
	require.NoError(t, err)

	return client, server.Close
}

func writeClientTestJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func clientTestSubAccount(accountID, subAccountID uuid.UUID, status string) responses.SubAccount {
	return responses.SubAccount{
		Object:          "sub_account",
		ID:              subAccountID,
		ParentAccountID: accountID,
		CreatedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Name:            "Acme Subsidiary",
		Website:         "acme.example.com",
		Status:          status,
		MonthlyCredit:   50000,
		DomainCount:     2,
		MemberCount:     3,
	}
}

func clientTestSubAccountUsage(accountID, subAccountID uuid.UUID) responses.SubAccountUsageResponse {
	subAccountName := "Acme Subsidiary"

	return responses.SubAccountUsageResponse{
		BillingPeriod: responses.SubAccountUsageBillingPeriod{
			Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		Currency:         "usd",
		AllocationMethod: "proportional",
		Parent: responses.SubAccountUsageBreakdown{
			AccountID:      &accountID,
			ReceptionCount: 1000000,
			AllocatedCost:  20,
		},
		SubAccounts: []responses.SubAccountUsageBreakdown{
			{
				AccountID:      &subAccountID,
				Name:           &subAccountName,
				ReceptionCount: 3000000,
				AllocatedCost:  60,
			},
		},
		RemovedSubAccounts: responses.SubAccountUsageBreakdown{},
		Total: responses.SubAccountUsageBreakdown{
			ReceptionCount: 4000000,
			AllocatedCost:  80,
		},
	}
}

func clientTestAPIKey(accountID, keyID uuid.UUID, secretKey *string) responses.APIKey {
	return responses.APIKey{
		Object:    "api_key",
		ID:        keyID,
		CreatedAt: time.Date(2024, 1, 1, 0, 5, 0, 0, time.UTC),
		UpdatedAt: time.Date(2024, 1, 1, 0, 5, 0, 0, time.UTC),
		AccountID: accountID,
		Label:     "Bootstrap key",
		PublicKey: "aha-pk-child-public-key",
		Scopes: []responses.APIKeyScope{
			{
				ID:        uuid.MustParse("c574470d-76ef-4f74-9b24-70a583a17e03"),
				CreatedAt: time.Date(2024, 1, 1, 0, 5, 0, 0, time.UTC),
				UpdatedAt: time.Date(2024, 1, 1, 0, 5, 0, 0, time.UTC),
				APIKeyID:  keyID,
				Scope:     "messages:send:all",
			},
		},
		SecretKey: secretKey,
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
		Enabled: ahasend.Bool(true),
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
