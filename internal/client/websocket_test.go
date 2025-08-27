package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitiateRouteStream_Success(t *testing.T) {
	tests := []struct {
		name        string
		routeID     string
		recipient   string
		wantRouteID string
		wantQuery   map[string]string
	}{
		{
			name:        "with route ID",
			routeID:     "test-route-123",
			recipient:   "",
			wantRouteID: "test-route-123",
			wantQuery:   map[string]string{"route_id": "test-route-123"},
		},
		{
			name:        "with recipient pattern",
			routeID:     "",
			recipient:   "*@example.com",
			wantRouteID: "temp-route-456",
			wantQuery:   map[string]string{"recipient": "*@example.com"},
		},
		{
			name:        "with complex recipient pattern",
			routeID:     "",
			recipient:   "support-*@company.com",
			wantRouteID: "temp-route-789",
			wantQuery:   map[string]string{"recipient": "support-*@company.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/routes/stream")
				
				// Verify authorization header
				assert.NotEmpty(t, r.Header.Get("Authorization"))
				assert.Contains(t, r.Header.Get("Authorization"), "Bearer")
				
				// Verify query parameters
				for key, value := range tt.wantQuery {
					assert.Equal(t, value, r.URL.Query().Get(key))
				}
				
				// Send mock response
				response := RouteStreamResponse{
					RouteID: tt.wantRouteID,
					WsURL:   fmt.Sprintf("wss://ws.example.com/routes/%s", tt.wantRouteID),
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			
			// Create client with test server
			client := createTestClient(t, server.URL)
			
			// Call InitiateRouteStream
			resp, err := client.InitiateRouteStream(tt.routeID, tt.recipient)
			
			// Verify response
			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.wantRouteID, resp.RouteID)
			assert.Contains(t, resp.WsURL, "wss://")
		})
	}
}

func TestInitiateRouteStream_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		routeID   string
		recipient string
		wantError string
	}{
		{
			name:      "no parameters provided",
			routeID:   "",
			recipient: "",
			wantError: "either route_id or recipient must be provided",
		},
		{
			name:      "both parameters provided",
			routeID:   "test-route-123",
			recipient: "*@example.com",
			wantError: "only one of route_id or recipient can be provided, not both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server (won't be called)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("Server should not be called for validation errors")
			}))
			defer server.Close()
			
			// Create client with test server
			client := createTestClient(t, server.URL)
			
			// Call InitiateRouteStream
			resp, err := client.InitiateRouteStream(tt.routeID, tt.recipient)
			
			// Verify error
			assert.Error(t, err)
			assert.Nil(t, resp)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestInitiateRouteStream_ServerErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		routeID    string
		recipient  string
		wantError  string
	}{
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			routeID:    "test-route-123",
			recipient:  "",
			wantError:  "unexpected status code: 401",
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			routeID:    "non-existent-route",
			recipient:  "",
			wantError:  "unexpected status code: 404",
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			routeID:    "",
			recipient:  "*@test.com",
			wantError:  "unexpected status code: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server that returns error
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()
			
			// Create client with test server
			client := createTestClient(t, server.URL)
			
			// Call InitiateRouteStream
			resp, err := client.InitiateRouteStream(tt.routeID, tt.recipient)
			
			// Verify error
			assert.Error(t, err)
			assert.Nil(t, resp)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestInitiateWebhookStream_Success(t *testing.T) {
	tests := []struct {
		name          string
		webhookID     string
		wantWebhookID string
	}{
		{
			name:          "with webhook ID",
			webhookID:     "webhook-123",
			wantWebhookID: "webhook-123",
		},
		{
			name:          "without webhook ID (creates temporary)",
			webhookID:     "",
			wantWebhookID: "temp-webhook-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/webhooks/stream")
				
				// Verify webhook_id query parameter if provided
				if tt.webhookID != "" {
					assert.Equal(t, tt.webhookID, r.URL.Query().Get("webhook_id"))
				}
				
				// Send mock response
				response := WebhookStreamResponse{
					WebhookID: tt.wantWebhookID,
					WsURL:     fmt.Sprintf("wss://ws.example.com/webhooks/%s", tt.wantWebhookID),
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			
			// Create client with test server
			client := createTestClient(t, server.URL)
			
			// Call InitiateWebhookStream
			resp, err := client.InitiateWebhookStream(tt.webhookID)
			
			// Verify response
			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.wantWebhookID, resp.WebhookID)
			assert.Contains(t, resp.WsURL, "wss://")
		})
	}
}

func TestInitiateRouteStream_JSONDecodeError(t *testing.T) {
	// Create test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()
	
	// Create client with test server
	client := createTestClient(t, server.URL)
	
	// Call InitiateRouteStream
	resp, err := client.InitiateRouteStream("test-route-123", "")
	
	// Verify error
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid character")
}

// Helper function to create a test client with custom server URL
func createTestClient(t *testing.T, serverURL string) *Client {
	apiKey := "test-api-key"
	accountID := uuid.New().String()
	
	// Create SDK config with test server URL
	config := api.NewConfiguration()
	config.Host = serverURL[7:] // Remove http:// prefix
	config.Scheme = "http"
	config.UserAgent = "test-client"
	config.HTTPClient = http.DefaultClient
	config.RetryConfig = api.RetryConfig{
		Enabled: false,
	}
	
	// Create auth context
	auth := context.WithValue(context.Background(), api.ContextAccessToken, apiKey)
	
	// Create our wrapper client
	client := &Client{
		APIClient:   api.NewAPIClientWithConfig(config),
		auth:        auth,
		accountID:   accountID,
		config:      config,
		rateLimiter: NewRateLimiter(50, 100),
	}
	
	return client
}