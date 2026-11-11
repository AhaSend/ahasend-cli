package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
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

	"github.com/AhaSend/ahasend-cli/internal/version"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
)

func TestNewClient_Success(t *testing.T) {
	apiKey := "test-api-key"
	accountID := uuid.New().String()

	client, err := NewClient(apiKey, accountID)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, accountID, client.GetAccountID())
	assert.Equal(t, uuid.MustParse(accountID), client.accountUUID)
	assert.NotNil(t, client.APIClient)

	// Verify the auth context contains the API key
	authValue := client.auth.Value(api.ContextAccessToken)
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
	assert.Equal(t, 10*time.Minute, client.config.HTTPClient.Timeout)
	assert.True(t, strings.HasPrefix(client.config.UserAgent, "ahasend-cli/"+version.Version+" "), client.config.UserAgent)
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
		{
			name:      "account ID that is not a UUID",
			apiKey:    "test-api-key",
			accountID: "invalid-uuid",
			wantError: "invalid account ID format",
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

// TestNewClientRejectsAnAPIURLItCannotSendTo checks that an API URL with a
// path, which the SDK has no place for, or a scheme other than HTTP is
// refused instead of being sent somewhere else.
func TestNewClientRejectsAnAPIURLItCannotSendTo(t *testing.T) {
	for _, apiURL := range []string{"http://127.0.0.1:1/prefix", "ftp://127.0.0.1:1", "htp://127.0.0.1:1"} {
		t.Run(apiURL, func(t *testing.T) {
			client, err := NewClient("test-api-key", uuid.New().String(), apiURL)

			assert.Nil(t, client)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid API URL: must")
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
				"limit": "25",
				"after": "sub-cursor",
			},
			call: func(c *Client) (any, error) {
				limit := int32(25)
				after := "sub-cursor"
				return c.ListSubAccounts(common.PaginationParams{Limit: &limit, After: &after})
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
				}, api.WithIdempotencyKey("sub-create-key"))
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
				return c.GetSubAccount(subAccountID)
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
				return c.UpdateSubAccount(subAccountID, requests.UpdateSubAccountRequest{
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
				return c.DeleteSubAccount(subAccountID)
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
				return c.SuspendSubAccount(subAccountID, requests.SuspendSubAccountRequest{
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
				return c.UnsuspendSubAccount(subAccountID)
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
				"before": "key-cursor",
			},
			call: func(c *Client) (any, error) {
				limit := int32(10)
				before := "key-cursor"
				return c.ListSubAccountAPIKeys(subAccountID, common.PaginationParams{Limit: &limit, Before: &before})
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
				return c.CreateSubAccountAPIKey(subAccountID, requests.CreateAPIKeyRequest{
					Label:  "Bootstrap key",
					Scopes: []string{"messages:send:all", "domains:read"},
				}, api.WithIdempotencyKey("key-create-key"))
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
				return c.GetSubAccountAPIKey(subAccountID, keyID)
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
				return c.UpdateSubAccountAPIKey(subAccountID, keyID, requests.UpdateAPIKeyRequest{
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
				return c.DeleteSubAccountAPIKey(subAccountID, keyID)
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

func TestClient_PingAndWebhookMethodsCallTheirEndpoints(t *testing.T) {
	accountID := uuid.New()
	webhookID := uuid.New()
	limit := int32(10)
	cursor := "test-cursor"
	webhookPath := "/v2/accounts/" + accountID.String() + "/webhooks"

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		call   func(*Client) error
	}{
		{"ping", http.MethodGet, "/v2/ping", `{"message":"OK"}`,
			func(c *Client) error { _, err := c.Ping(); return err }},
		{"list webhooks", http.MethodGet, webhookPath, `{"object":"list","data":[]}`,
			func(c *Client) error { _, err := c.ListWebhooks(common.PaginationParams{}, nil, nil); return err }},
		{"list webhooks with pagination", http.MethodGet, webhookPath, `{"object":"list","data":[]}`,
			func(c *Client) error {
				_, err := c.ListWebhooks(common.PaginationParams{Limit: &limit, After: &cursor}, nil, nil)
				return err
			}},
		{"create webhook", http.MethodPost, webhookPath, `{"object":"webhook"}`,
			func(c *Client) error {
				_, err := c.CreateWebhook(requests.CreateWebhookRequest{
					Name: "Test Webhook", URL: "https://example.com/webhook", Enabled: ahasend.Bool(true),
				})
				return err
			}},
		{"get webhook", http.MethodGet, webhookPath + "/" + webhookID.String(), `{"object":"webhook"}`,
			func(c *Client) error { _, err := c.GetWebhook(webhookID); return err }},
		{"update webhook", http.MethodPut, webhookPath + "/" + webhookID.String(), `{"object":"webhook"}`,
			func(c *Client) error {
				_, err := c.UpdateWebhook(webhookID, requests.UpdateWebhookRequest{Name: ahasend.String("Updated")})
				return err
			}},
		{"delete webhook", http.MethodDelete, webhookPath + "/" + webhookID.String(), `{"message":"OK"}`,
			func(c *Client) error { _, err := c.DeleteWebhook(webhookID); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requested := false
			client, cleanup := newClientTestServer(t, accountID.String(), func(w http.ResponseWriter, r *http.Request) {
				requested = true
				assert.Equal(t, tt.method, r.Method)
				assert.Equal(t, tt.path, r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			})
			defer cleanup()

			require.NoError(t, tt.call(client))
			assert.True(t, requested)
		})
	}
}

func TestClient_ListWebhooksSendsTheEnabledFilter(t *testing.T) {
	accountID := uuid.New()
	var query string
	client, cleanup := newClientTestServer(t, accountID.String(), func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("enabled")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	})
	defer cleanup()

	_, err := client.ListWebhooks(common.PaginationParams{}, ahasend.Bool(true), nil)

	require.NoError(t, err)
	assert.Equal(t, "true", query)
}

// TestClient_ListWebhooksSendsTheSendingTypeChangedFilter checks that the
// list filter of sending_type_changed sends on_sending_type_changed alone.
func TestClient_ListWebhooksSendsTheSendingTypeChangedFilter(t *testing.T) {
	accountID := uuid.New()
	eventFilters := url.Values{}
	client, cleanup := newClientTestServer(t, accountID.String(), func(w http.ResponseWriter, r *http.Request) {
		for key, values := range r.URL.Query() {
			if strings.HasPrefix(key, "on_") {
				eventFilters[key] = values
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	})
	defer cleanup()

	_, err := client.ListWebhooks(common.PaginationParams{}, nil, []string{"sending_type_changed"})

	require.NoError(t, err)
	assert.Equal(t, url.Values{"on_sending_type_changed": {"true"}}, eventFilters)
}

// TestClient_ListWebhooksFiltersByEachEvent checks that the list filter of
// each webhook event sends its on_<name> query parameter alone, as true, and
// that no two events send the same one.
func TestClient_ListWebhooksFiltersByEachEvent(t *testing.T) {
	accountID := uuid.New()
	filters := map[string]string{}
	for _, event := range webhooks.Events {
		t.Run(event.Name, func(t *testing.T) {
			eventFilters := url.Values{}
			client, cleanup := newClientTestServer(t, accountID.String(), func(w http.ResponseWriter, r *http.Request) {
				for key, values := range r.URL.Query() {
					if strings.HasPrefix(key, "on_") {
						eventFilters[key] = values
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
			})
			defer cleanup()

			_, err := client.ListWebhooks(common.PaginationParams{}, nil, []string{event.Name})

			require.NoError(t, err)
			assert.Equal(t, url.Values{"on_" + event.Name: {"true"}}, eventFilters)
			for key := range eventFilters {
				assert.NotContains(t, filters, key, "%s and %s send the same filter", filters[key], event.Name)
				filters[key] = event.Name
			}
		})
	}
}

// TestClient_ListWebhooksSendsEveryFilterInOneGet checks the method, path,
// query and body of a list with the enabled filter, two event filters and a
// limit.
func TestClient_ListWebhooksSendsEveryFilterInOneGet(t *testing.T) {
	accountID := uuid.New()
	limit := int32(5)
	var method, path, query string
	var body []byte
	client, cleanup := newClientTestServer(t, accountID.String(), func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.RawQuery
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	})
	defer cleanup()

	_, err := client.ListWebhooks(common.PaginationParams{Limit: &limit}, ahasend.Bool(false), []string{"contact_created", "delivered"})

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, method)
	assert.Equal(t, "/v2/accounts/"+accountID.String()+"/webhooks", path)
	assert.Equal(t, "enabled=false&limit=5&on_contact_created=true&on_delivered=true", query)
	assert.Empty(t, body)
}

func TestClient_ListWebhooksRefusesAnUnknownEventWithoutARequest(t *testing.T) {
	accountID := uuid.New()
	requested := false
	client, cleanup := newClientTestServer(t, accountID.String(), func(w http.ResponseWriter, r *http.Request) {
		requested = true
	})
	defer cleanup()

	_, err := client.ListWebhooks(common.PaginationParams{}, nil, []string{"message.delivered"})

	require.EqualError(t, err, `internal error: "message.delivered" is not a webhook event to filter by`)
	assert.False(t, requested)
}

// TestClient_TemplateReadsCallTheirEndpoints checks that GetTemplate reads a
// template's working copy and GetPublishedTemplate its published copy, each
// with a GET that has no query and no body.
func TestClient_TemplateReadsCallTheirEndpoints(t *testing.T) {
	accountID, templateID := uuid.New(), uuid.New()
	templatePath := "/v2/accounts/" + accountID.String() + "/templates/" + templateID.String()
	tests := []struct {
		name string
		path string
		call func(*Client) (*responses.Template, error)
	}{
		{"working copy", templatePath,
			func(c *Client) (*responses.Template, error) { return c.GetTemplate(templateID) }},
		{"published copy", templatePath + "/published",
			func(c *Client) (*responses.Template, error) { return c.GetPublishedTemplate(templateID) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var method, path, query string
			var body []byte
			client, cleanup := newClientTestServer(t, accountID.String(), func(w http.ResponseWriter, r *http.Request) {
				method, path, query = r.Method, r.URL.Path, r.URL.RawQuery
				body, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"object":"template","name":"Welcome"}`))
			})
			defer cleanup()

			template, err := tt.call(client)

			require.NoError(t, err)
			assert.Equal(t, http.MethodGet, method)
			assert.Equal(t, tt.path, path)
			assert.Empty(t, query)
			assert.Empty(t, body)
			assert.Equal(t, "Welcome", template.Name)
		})
	}
}

// TestClient_SegmentMethodsCallTheirEndpoints checks the method, path, query,
// body and idempotency key of each segment request, and that its answer is
// read.
func TestClient_SegmentMethodsCallTheirEndpoints(t *testing.T) {
	accountID, segmentID := uuid.New(), uuid.New()
	segmentsPath := "/v2/accounts/" + accountID.String() + "/segments"
	segmentPath := segmentsPath + "/" + segmentID.String()
	limit, after, before := int32(25), "next-page", "previous-page"
	timezone, empty := "Europe/Berlin", ""
	plan := "pro"
	definition := common.SegmentDefinition{
		SchemaVersion: common.SegmentSchemaVersion,
		Match:         common.SegmentMatchAll,
		Conditions: []common.SegmentRule{{Condition: &common.SegmentCondition{
			Field: "attributes.plan", Op: common.SegmentOpEqual, Value: &common.SegmentValue{String: &plan},
		}}},
	}
	definitionJSON := `{"schema_version":1,"match":"all","conditions":[{"field":"attributes.plan","op":"equal","value":"pro"}]}`
	segmentBody := `{"object":"segment","id":"` + segmentID.String() + `","name":"Pro"}`
	segmentsBody := `{"object":"list","data":[` + segmentBody + `],"pagination":{"has_more":false}}`

	tests := []struct {
		name        string
		method      string
		path        string
		query       string
		body        string
		idempotency string
		response    string
		call        func(*Client) (any, error)
	}{
		{"GetSegments", http.MethodGet, segmentsPath, "after=next-page&limit=25", "", "", segmentsBody,
			func(c *Client) (any, error) {
				return c.GetSegments(requests.GetSegmentsParams{Limit: &limit, After: &after})
			}},
		{"GetSegment", http.MethodGet, segmentPath, "", "", "", segmentBody,
			func(c *Client) (any, error) { return c.GetSegment(segmentID) }},
		{"CreateSegment", http.MethodPost, segmentsPath, "",
			`{"name":"Pro","definition":` + definitionJSON + `,"timezone":"Europe/Berlin"}`, "create-pro", segmentBody,
			func(c *Client) (any, error) {
				return c.CreateSegment(requests.CreateSegmentRequest{Name: "Pro", Definition: definition, Timezone: &timezone},
					api.WithIdempotencyKey("create-pro"))
			}},
		{"UpdateSegment", http.MethodPut, segmentPath, "", `{"description":"","definition":` + definitionJSON + `}`, "",
			segmentBody,
			func(c *Client) (any, error) {
				return c.UpdateSegment(segmentID, requests.UpdateSegmentRequest{Description: &empty, Definition: &definition})
			}},
		{"DeleteSegment", http.MethodDelete, segmentPath, "", "", "", `{"message":"Segment deleted"}`,
			func(c *Client) (any, error) { return c.DeleteSegment(segmentID) }},
		{"PreviewSegment", http.MethodPost, segmentsPath + "/preview", "",
			`{"definition":` + definitionJSON + `,"timezone":""}`, "", `{"object":"segment_preview","count":3,"contacts":[]}`,
			func(c *Client) (any, error) {
				return c.PreviewSegment(requests.PreviewSegmentRequest{Definition: definition, Timezone: &empty})
			}},
		{"GetSegmentContacts", http.MethodGet, segmentPath + "/contacts", "before=previous-page&limit=100", "", "",
			`{"object":"list","data":[{"object":"segment_contact","email":"a@example.com"}],"pagination":{"has_more":false}}`,
			func(c *Client) (any, error) {
				return c.GetSegmentContacts(segmentID, requests.GetSegmentContactsParams{Before: &before})
			}},
		{"GetContactSegments", http.MethodGet,
			"/v2/accounts/" + accountID.String() + "/contacts/user+tag@example.com/segments", "limit=100", "", "", segmentsBody,
			func(c *Client) (any, error) {
				return c.GetContactSegments("user+tag@example.com", requests.GetContactSegmentsParams{})
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var method, path, query, idempotency string
			var body []byte
			client, cleanup := newClientTestServer(t, accountID.String(), func(w http.ResponseWriter, r *http.Request) {
				method, path, query = r.Method, r.URL.Path, r.URL.RawQuery
				idempotency = r.Header.Get("Idempotency-Key")
				body, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.response))
			})
			defer cleanup()

			result, err := tt.call(client)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.method, method)
			assert.Equal(t, tt.path, path)
			assert.Equal(t, tt.query, query)
			if tt.body == "" {
				assert.Empty(t, body)
			} else {
				assert.JSONEq(t, tt.body, string(body))
			}
			if tt.idempotency != "" {
				assert.Equal(t, tt.idempotency, idempotency)
			}
		})
	}
}

// TestSendRequestsAreSentOnce checks that the send methods leave retries to
// their caller: a failed send is one request, not one plus the SDK's own
// retries.
func TestSendRequestsAreSentOnce(t *testing.T) {
	t.Cleanup(ShortenRetryDelaysForTesting())
	accountID := uuid.New().String()
	recipients := []common.Recipient{{Email: "user@example.com"}}
	sends := map[string]func(*Client) error{
		"inline": func(c *Client) error {
			_, err := c.SendMessageWithIdempotencyKey(requests.CreateMessageRequest{
				From: common.SenderAddress{Email: "sender@example.com"}, Recipients: recipients,
				Subject: "Hi", TextContent: ahasend.String("Hi"),
			}, "key")
			return err
		},
		"template": func(c *Client) error {
			_, err := c.SendTemplateMessageWithIdempotencyKey(requests.CreateTemplateMessageRequest{
				TemplateID: uuid.New(), Recipients: recipients,
			}, "key")
			return err
		},
	}
	for name, send := range sends {
		t.Run(name, func(t *testing.T) {
			var requests atomic.Int32
			client, cleanup := newClientTestServer(t, accountID, func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				writeClientTestJSON(t, w, http.StatusServiceUnavailable, map[string]string{"message": "unavailable"})
			})
			defer cleanup()

			require.Error(t, send(client))
			assert.Equal(t, int32(1), requests.Load())
		})
	}
}

// TestCampaignTestIsSentOnce checks that a campaign test the API refuses with
// 429, as it does once the daily test sends are used, is one request: no
// retry can pass the limit.
func TestCampaignTestIsSentOnce(t *testing.T) {
	t.Cleanup(ShortenRetryDelaysForTesting())
	var sent atomic.Int32
	client, cleanup := newClientTestServer(t, uuid.New().String(), func(w http.ResponseWriter, _ *http.Request) {
		sent.Add(1)
		writeClientTestJSON(t, w, http.StatusTooManyRequests, map[string]string{"message": "daily test send limit reached"})
	})
	defer cleanup()

	_, err := client.TestCampaign(uuid.New(), requests.TestCampaignRequest{To: []string{"me@example.com"}})

	require.Error(t, err)
	assert.Equal(t, int32(1), sent.Load())
}

// TestALongRetryAfterIsNotWaitedFor checks that a request the API asks to
// retry after more than the 30-second retry cap fails at once with the API's
// error, instead of being retried before the API would accept it.
func TestALongRetryAfterIsNotWaitedFor(t *testing.T) {
	t.Cleanup(ShortenRetryDelaysForTesting())
	var sent atomic.Int32
	client, cleanup := newClientTestServer(t, uuid.New().String(), func(w http.ResponseWriter, _ *http.Request) {
		sent.Add(1)
		w.Header().Set("Retry-After", "31")
		writeClientTestJSON(t, w, http.StatusTooManyRequests, map[string]string{"message": "rate limit exceeded"})
	})
	defer cleanup()

	_, err := client.GetAccount()

	var apiErr *api.APIError
	require.True(t, errors.As(err, &apiErr), "expected an API error, got %v", err)
	assert.Equal(t, 31, apiErr.RetryAfter)
	assert.Equal(t, int32(1), sent.Load())
}

// TestDeleteRequestsAreSentOnce checks that every DELETE request is sent
// once: repeated after a lost answer, it would fail with 404 although the
// first one succeeded. An update, which is safe to repeat, keeps the SDK's
// retries.
func TestDeleteRequestsAreSentOnce(t *testing.T) {
	t.Cleanup(ShortenRetryDelaysForTesting())
	accountID := uuid.New().String()
	id := uuid.New()
	domain := "example.com"
	deletes := map[string]func(*Client) error{
		"cancel message":           func(c *Client) error { _, err := c.CancelMessage(id.String()); return err },
		"delete domain":            func(c *Client) error { _, err := c.DeleteDomain(domain); return err },
		"delete webhook":           func(c *Client) error { _, err := c.DeleteWebhook(id); return err },
		"delete route":             func(c *Client) error { _, err := c.DeleteRoute(id); return err },
		"delete suppression":       func(c *Client) error { _, err := c.DeleteSuppression("a@example.com", &domain); return err },
		"wipe suppressions":        func(c *Client) error { _, err := c.WipeSuppressions(nil); return err },
		"delete smtp credential":   func(c *Client) error { _, err := c.DeleteSMTPCredential(id); return err },
		"delete sub-account":       func(c *Client) error { _, err := c.DeleteSubAccount(id); return err },
		"delete sub-account key":   func(c *Client) error { _, err := c.DeleteSubAccountAPIKey(id, id); return err },
		"delete api key":           func(c *Client) error { _, err := c.DeleteAPIKey(id); return err },
		"delete contact":           func(c *Client) error { _, err := c.DeleteContact("a@example.com"); return err },
		"delete list":              func(c *Client) error { _, err := c.DeleteList(id); return err },
		"remove list member":       func(c *Client) error { _, err := c.DeleteListContact(id, "a@example.com"); return err },
		"delete segment":           func(c *Client) error { _, err := c.DeleteSegment(id); return err },
		"delete template":          func(c *Client) error { _, err := c.DeleteTemplate(id); return err },
		"discard template draft":   func(c *Client) error { _, err := c.DiscardTemplateDraft(id); return err },
		"delete campaign":          func(c *Client) error { _, err := c.DeleteCampaign(id); return err },
		"delete campaign template": func(c *Client) error { _, err := c.DeleteCampaignTemplate(id); return err },
	}
	for name, call := range deletes {
		t.Run(name, func(t *testing.T) {
			var sent atomic.Int32
			client, cleanup := newClientTestServer(t, accountID, func(w http.ResponseWriter, r *http.Request) {
				sent.Add(1)
				assert.Equal(t, http.MethodDelete, r.Method)
				writeClientTestJSON(t, w, http.StatusServiceUnavailable, map[string]string{"message": "unavailable"})
			})
			defer cleanup()

			require.Error(t, call(client))
			assert.Equal(t, int32(1), sent.Load())
		})
	}

	t.Run("update", func(t *testing.T) {
		var sent atomic.Int32
		client, cleanup := newClientTestServer(t, accountID, func(w http.ResponseWriter, _ *http.Request) {
			sent.Add(1)
			writeClientTestJSON(t, w, http.StatusServiceUnavailable, map[string]string{"message": "unavailable"})
		})
		defer cleanup()

		_, err := client.UpdateWebhook(id, requests.UpdateWebhookRequest{Name: ahasend.String("Updated")})

		require.Error(t, err)
		assert.Equal(t, int32(4), sent.Load(), "an update is sent once and retried 3 times")
	})
}

// TestMessageIDsReachTheAPIEscapedInThePath checks that a Message-ID such as
// <uuid@example.com> is sent as one escaped path segment.
func TestMessageIDsReachTheAPIEscapedInThePath(t *testing.T) {
	accountID := uuid.New().String()
	const messageID = "<8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f@example.com>"
	calls := map[string]func(*Client) error{
		"/messages/%3C8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f@example.com%3E": func(c *Client) error {
			_, err := c.GetMessage(messageID)
			return err
		},
		"/messages/%3C8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f@example.com%3E/cancel": func(c *Client) error {
			_, err := c.CancelMessage(messageID)
			return err
		},
	}
	for suffix, call := range calls {
		t.Run(suffix, func(t *testing.T) {
			var path string
			client, cleanup := newClientTestServer(t, accountID, func(w http.ResponseWriter, r *http.Request) {
				path = r.URL.EscapedPath()
				writeClientTestJSON(t, w, http.StatusOK, map[string]string{"message": "OK"})
			})
			defer cleanup()

			_ = call(client)

			assert.Equal(t, "/v2/accounts/"+accountID+suffix, path)
		})
	}
}

// TestSendFailuresAfterTheRequestWasWrittenAreRequestWrittenErrors checks
// that a send that got no answer after its request was written fails with a
// RequestWrittenError, and one that never reached the API or got an answer
// fails with its own error.
func TestSendFailuresAfterTheRequestWasWrittenAreRequestWrittenErrors(t *testing.T) {
	previous := responseHeaderTimeout
	responseHeaderTimeout = 200 * time.Millisecond
	t.Cleanup(func() { responseHeaderTimeout = previous })
	accountID := uuid.New().String()
	tests := map[string]struct {
		handler http.HandlerFunc
		written bool
	}{
		"connection closed before the answer": {func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.ReadAll(r.Body)
			conn, _, err := http.NewResponseController(w).Hijack()
			require.NoError(t, err)
			_ = conn.Close()
		}, true},
		"timeout waiting for the answer": {func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.ReadAll(r.Body)
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		}, true},
		"gateway timeout": {func(w http.ResponseWriter, _ *http.Request) {
			writeClientTestJSON(t, w, http.StatusGatewayTimeout, map[string]string{"message": "timeout"})
		}, false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client, cleanup := newClientTestServer(t, accountID, test.handler)
			defer cleanup()

			_, err := client.SendMessageWithIdempotencyKey(requests.CreateMessageRequest{
				From: common.SenderAddress{Email: "sender@example.com"}, Recipients: []common.Recipient{{Email: "user@example.com"}},
				Subject: "Hi", TextContent: ahasend.String("Hi"),
			}, "key")

			require.Error(t, err)
			var written *RequestWrittenError
			assert.Equal(t, test.written, errors.As(err, &written), "%v", err)
		})
	}

	t.Run("connection refused", func(t *testing.T) {
		client, err := NewClient("test-api-key", accountID, "http://127.0.0.1:1")
		require.NoError(t, err)

		_, err = client.SendTemplateMessageWithIdempotencyKey(requests.CreateTemplateMessageRequest{
			TemplateID: uuid.New(), Recipients: []common.Recipient{{Email: "user@example.com"}},
		}, "key")

		require.Error(t, err)
		var written *RequestWrittenError
		assert.False(t, errors.As(err, &written), "%v", err)
	})
}

// slowConn is a connection whose writes go at about bytesPerSecond, as on a
// slow uplink.
type slowConn struct {
	net.Conn
	bytesPerSecond int
}

func (c slowConn) Write(p []byte) (int, error) {
	time.Sleep(time.Duration(len(p)) * time.Second / time.Duration(c.bytesPerSecond))
	return c.Conn.Write(p)
}

// TestASlowUploadIsNotCutShortByTheWaitForTheAnswer checks that the limit of
// the wait for the API's answer starts once the request is written, so an
// upload that takes longer than it succeeds, and a late answer still fails
// as a RequestWrittenError.
func TestASlowUploadIsNotCutShortByTheWaitForTheAnswer(t *testing.T) {
	previous := responseHeaderTimeout
	responseHeaderTimeout = 300 * time.Millisecond
	t.Cleanup(func() { responseHeaderTimeout = previous })
	dial := dialAPI
	dialAPI = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		// 512 KB takes about a second
		return slowConn{Conn: conn, bytesPerSecond: 512 * 1024}, nil
	}
	t.Cleanup(func() { dialAPI = dial })
	accountID := uuid.New().String()
	request := requests.CreateMessageRequest{
		From: common.SenderAddress{Email: "sender@example.com"}, Recipients: []common.Recipient{{Email: "user@example.com"}},
		Subject: "Hi", TextContent: ahasend.String(strings.Repeat("a", 512*1024)),
	}

	t.Run("an answer after the upload", func(t *testing.T) {
		client, cleanup := newClientTestServer(t, accountID, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.ReadAll(r.Body)
			writeClientTestJSON(t, w, http.StatusOK, map[string]any{"object": "list", "data": []any{}})
		})
		defer cleanup()

		started := time.Now()
		_, err := client.SendMessageWithIdempotencyKey(request, "key")

		require.NoError(t, err)
		assert.Greater(t, time.Since(started), responseHeaderTimeout, "the upload took longer than the wait for the answer")
	})

	t.Run("no answer after the upload", func(t *testing.T) {
		client, cleanup := newClientTestServer(t, accountID, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.ReadAll(r.Body)
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		})
		defer cleanup()

		_, err := client.SendMessageWithIdempotencyKey(request, "key")

		require.Error(t, err)
		var written *RequestWrittenError
		assert.True(t, errors.As(err, &written), "%v", err)
	})
}
