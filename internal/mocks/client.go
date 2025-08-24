// Package mocks provides comprehensive mock implementations for testing AhaSend CLI components.
//
// This package implements standardized mock objects using the testify/mock framework:
//
//   - MockClient: Complete AhaSendClient interface implementation
//   - MockConfigManager: Full ConfigManager interface implementation
//   - Interface compliance verification at compile time
//   - Helper methods for creating common test data structures
//   - Consistent error handling patterns for test scenarios
//   - Documentation and examples for proper usage patterns
//
// All mocks follow the same interface-based design patterns as production code,
// enabling comprehensive testing without external dependencies while maintaining
// type safety and interface compliance.
package mocks

import (
	"context"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/AhaSend/ahasend-go/webhooks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/AhaSend/ahasend-cli/internal/client"
)

// MockClient is a mock implementation of the AhaSendClient interface
type MockClient struct {
	mock.Mock
}

// Ensure MockClient implements AhaSendClient interface
var _ client.AhaSendClient = (*MockClient)(nil)

// Authentication and account info methods

func (m *MockClient) GetAccountID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockClient) GetAuthContext() context.Context {
	args := m.Called()
	if args.Get(0) == nil {
		return context.Background()
	}
	return args.Get(0).(context.Context)
}

func (m *MockClient) GetAccount() (*responses.Account, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.Account), args.Error(1)
}

func (m *MockClient) Ping() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockClient) ValidateConfiguration() error {
	args := m.Called()
	return args.Error(0)
}

// Message operations methods

func (m *MockClient) SendMessage(req requests.CreateMessageRequest) (*responses.CreateMessageResponse, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.CreateMessageResponse), args.Error(1)
}

func (m *MockClient) SendMessageWithIdempotencyKey(req requests.CreateMessageRequest, idempotencyKey string) (*responses.CreateMessageResponse, error) {
	args := m.Called(req, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.CreateMessageResponse), args.Error(1)
}

func (m *MockClient) CancelMessage(accountID, messageID string) (*common.SuccessResponse, error) {
	args := m.Called(accountID, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.SuccessResponse), args.Error(1)
}

func (m *MockClient) GetMessages(params requests.GetMessagesParams) (*responses.PaginatedMessagesResponse, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.PaginatedMessagesResponse), args.Error(1)
}

// Domain operations methods

func (m *MockClient) ListDomains(limit *int32, cursor *string) (*responses.PaginatedDomainsResponse, error) {
	args := m.Called(limit, cursor)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.PaginatedDomainsResponse), args.Error(1)
}

func (m *MockClient) CreateDomain(domain string) (*responses.Domain, error) {
	args := m.Called(domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.Domain), args.Error(1)
}

func (m *MockClient) GetDomain(domain string) (*responses.Domain, error) {
	args := m.Called(domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.Domain), args.Error(1)
}

func (m *MockClient) DeleteDomain(domain string) (*common.SuccessResponse, error) {
	args := m.Called(domain)
	return args.Get(0).(*common.SuccessResponse), args.Error(1)
}

// Webhook operations methods

func (m *MockClient) CreateWebhookVerifier(secret string) (*webhooks.WebhookVerifier, error) {
	args := m.Called(secret)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*webhooks.WebhookVerifier), args.Error(1)
}

func (m *MockClient) ListWebhooks(limit *int32, cursor *string) (*responses.PaginatedWebhooksResponse, error) {
	args := m.Called(limit, cursor)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.PaginatedWebhooksResponse), args.Error(1)
}

func (m *MockClient) CreateWebhook(req requests.CreateWebhookRequest) (*responses.Webhook, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.Webhook), args.Error(1)
}

func (m *MockClient) GetWebhook(webhookID string) (*responses.Webhook, error) {
	args := m.Called(webhookID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.Webhook), args.Error(1)
}

func (m *MockClient) UpdateWebhook(webhookID string, req requests.UpdateWebhookRequest) (*responses.Webhook, error) {
	args := m.Called(webhookID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.Webhook), args.Error(1)
}

func (m *MockClient) DeleteWebhook(webhookID string) error {
	args := m.Called(webhookID)
	return args.Error(0)
}

// Helper methods for creating mock data

// NewMockDomain creates a mock domain for testing
func (m *MockClient) NewMockDomain(domain string, valid bool) *responses.Domain {
	return &responses.Domain{
		Domain:    domain,
		DNSValid:  valid,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// NewMockDomainsResponse creates a mock paginated domains response for testing
func (m *MockClient) NewMockDomainsResponse(domains []responses.Domain, hasMore bool) *responses.PaginatedDomainsResponse {
	return &responses.PaginatedDomainsResponse{
		Object: "list",
		Data:   domains,
		Pagination: common.PaginationInfo{
			HasMore:    hasMore,
			NextCursor: nil,
		},
	}
}

// NewMockMessageResponse creates a mock create message response for testing
func (m *MockClient) NewMockMessageResponse(messageID string) *responses.CreateMessageResponse {
	id := messageID
	return &responses.CreateMessageResponse{
		Object: "message.create",
		Data: []responses.CreateSingleMessageResponse{
			{
				Object: "message",
				ID:     &id,
				Recipient: common.Recipient{
					Email: "recipient@example.com",
				},
				Status: "sent",
			},
		},
	}
}

// NewMockMessagesResponse creates a mock paginated messages response for testing
func (m *MockClient) NewMockMessagesResponse(messages []responses.Message, hasMore bool) *responses.PaginatedMessagesResponse {
	return &responses.PaginatedMessagesResponse{
		Object: "list",
		Data:   messages,
		Pagination: common.PaginationInfo{
			HasMore:    hasMore,
			NextCursor: nil,
		},
	}
}

// NewMockWebhook creates a mock webhook for testing
func (m *MockClient) NewMockWebhook(idStr, name, url string, enabled bool) responses.Webhook {
	// Parse string ID to UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		// If invalid UUID, create a new one
		id = uuid.New()
	}

	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now().Add(-1 * time.Hour)
	scope := "account"

	// Create boolean pointers for event types
	onReception := true
	onDelivered := true
	onBounced := true

	return responses.Webhook{
		ID:          id,
		Name:        name,
		URL:         url,
		Enabled:     enabled,
		Scope:       scope,
		Domains:     []string{"example.com", "test.com"},
		OnReception: onReception,
		OnDelivered: onDelivered,
		OnBounced:   onBounced,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

// NewMockWebhooksResponse creates a mock paginated webhooks response for testing
func (m *MockClient) NewMockWebhooksResponse(webhooks []responses.Webhook, hasMore bool) *responses.PaginatedWebhooksResponse {
	return &responses.PaginatedWebhooksResponse{
		Object: "list",
		Data:   webhooks,
		Pagination: common.PaginationInfo{
			HasMore:    hasMore,
			NextCursor: nil,
		},
	}
}

// Route operations methods

func (m *MockClient) ListRoutes(limit *int32, cursor *string) (*responses.PaginatedRoutesResponse, error) {
	args := m.Called(limit, cursor)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.PaginatedRoutesResponse), args.Error(1)
}

func (m *MockClient) CreateRoute(req requests.CreateRouteRequest) (*responses.Route, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.Route), args.Error(1)
}

func (m *MockClient) GetRoute(routeID string) (*responses.Route, error) {
	args := m.Called(routeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.Route), args.Error(1)
}

func (m *MockClient) UpdateRoute(routeID string, req requests.UpdateRouteRequest) (*responses.Route, error) {
	args := m.Called(routeID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.Route), args.Error(1)
}

func (m *MockClient) DeleteRoute(routeID string) error {
	args := m.Called(routeID)
	return args.Error(0)
}

// NewMockRoute creates a mock route for testing
func (m *MockClient) NewMockRoute(idStr, name, url, recipientFilter string, enabled bool) *responses.Route {
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now().Add(-1 * time.Hour)

	// Parse string ID to UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		// If invalid UUID, create a new one
		id = uuid.New()
	}

	route := &responses.Route{
		ID:        id,
		Name:      name,
		URL:       url,
		Enabled:   enabled,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	// Set recipient filter if provided
	if recipientFilter != "" {
		route.Recipient = recipientFilter
	}

	return route
}

// NewMockRouteWithOptions creates a mock route with processing options for testing
func (m *MockClient) NewMockRouteWithOptions(idStr, name, url string, enabled bool, options map[string]bool) *responses.Route {
	route := m.NewMockRoute(idStr, name, url, "", enabled)

	// Set processing options based on provided map
	if includeAttachments, ok := options["include_attachments"]; ok {
		route.Attachments = includeAttachments
	}
	if includeHeaders, ok := options["include_headers"]; ok {
		route.Headers = includeHeaders
	}
	if groupByMessageID, ok := options["group_by_message_id"]; ok {
		route.GroupByMessageID = groupByMessageID
	}
	if stripReplies, ok := options["strip_replies"]; ok {
		route.StripReplies = stripReplies
	}

	return route
}

// NewMockRoutesResponse creates a mock paginated routes response for testing
func (m *MockClient) NewMockRoutesResponse(routes []responses.Route, hasMore bool) *responses.PaginatedRoutesResponse {
	return &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   routes,
		Pagination: common.PaginationInfo{
			HasMore:    hasMore,
			NextCursor: nil,
		},
	}
}

// SMTP operations methods

func (m *MockClient) ListSMTPCredentials(limit *int32, cursor *string) (*responses.PaginatedSMTPCredentialsResponse, error) {
	args := m.Called(limit, cursor)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.PaginatedSMTPCredentialsResponse), args.Error(1)
}

func (m *MockClient) GetSMTPCredential(credentialID string) (*responses.SMTPCredential, error) {
	args := m.Called(credentialID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.SMTPCredential), args.Error(1)
}

func (m *MockClient) CreateSMTPCredential(req requests.CreateSMTPCredentialRequest) (*responses.SMTPCredential, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.SMTPCredential), args.Error(1)
}

func (m *MockClient) DeleteSMTPCredential(credentialID string) error {
	args := m.Called(credentialID)
	return args.Error(0)
}

// Suppression operations methods

func (m *MockClient) ListSuppressions(params requests.GetSuppressionsParams) (*responses.PaginatedSuppressionsResponse, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.PaginatedSuppressionsResponse), args.Error(1)
}

func (m *MockClient) CheckSuppression(email string, domain *string) (bool, *responses.Suppression, error) {
	args := m.Called(email, domain)
	return args.Bool(0), args.Get(1).(*responses.Suppression), args.Error(2)
}

func (m *MockClient) CreateSuppression(req requests.CreateSuppressionRequest) (*responses.CreateSuppressionResponse, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*responses.CreateSuppressionResponse), args.Error(1)
}

func (m *MockClient) DeleteSuppression(email string, domain *string) (*common.SuccessResponse, error) {
	args := m.Called(email, domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.SuccessResponse), args.Error(1)
}

func (m *MockClient) WipeSuppressions(domain *string) (*common.SuccessResponse, error) {
	args := m.Called(domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.SuccessResponse), args.Error(1)
}

// NewMockSuppression creates a mock suppression for testing
func (m *MockClient) NewMockSuppression(email, reason, domain string) *responses.Suppression {
	createdAt := time.Now().Add(-24 * time.Hour)
	expiresAt := time.Time{} // Zero time means never expires

	suppression := &responses.Suppression{
		Email:     email,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}

	// Set reason if provided
	if reason != "" {
		suppression.Reason = reason
	}

	// Set domain if provided
	if domain != "" {
		suppression.Domain = domain
	}

	return suppression
}

// NewMockSuppressionWithExpiry creates a mock suppression with expiry time for testing
func (m *MockClient) NewMockSuppressionWithExpiry(email, reason, domain string, expiresIn time.Duration) *responses.Suppression {
	suppression := m.NewMockSuppression(email, reason, domain)
	suppression.ExpiresAt = time.Now().Add(expiresIn)
	return suppression
}

// NewMockSuppressionsResponse creates a mock paginated suppressions response for testing
func (m *MockClient) NewMockSuppressionsResponse(suppressions []responses.Suppression, hasMore bool) *responses.PaginatedSuppressionsResponse {
	return &responses.PaginatedSuppressionsResponse{
		Object: "list",
		Data:   suppressions,
		Pagination: common.PaginationInfo{
			HasMore:    hasMore,
			NextCursor: nil,
		},
	}
}

// NewMockSMTPCredential creates a mock SMTP credential for testing
func (m *MockClient) NewMockSMTPCredential(id uint64, name, username, scope string, sandbox bool, domains []string) *responses.SMTPCredential {
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now().Add(-1 * time.Hour)

	credential := &responses.SMTPCredential{
		Object:    "smtp_credential",
		ID:        id,
		Name:      name,
		Username:  username,
		Scope:     scope,
		Sandbox:   sandbox,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if len(domains) > 0 {
		credential.Domains = domains
	}

	return credential
}

// NewMockSMTPCredentialsResponse creates a mock paginated SMTP credentials response for testing
func (m *MockClient) NewMockSMTPCredentialsResponse(credentials []responses.SMTPCredential, hasMore bool) *responses.PaginatedSMTPCredentialsResponse {
	return &responses.PaginatedSMTPCredentialsResponse{
		Object: "list",
		Data:   credentials,
		Pagination: common.PaginationInfo{
			HasMore:    hasMore,
			NextCursor: nil,
		},
	}
}

// Statistics operations methods
func (m *MockClient) GetDeliverabilityStatistics(params requests.GetDeliverabilityStatisticsParams) (*responses.DeliverabilityStatisticsResponse, error) {
	args := m.Called(params)
	return args.Get(0).(*responses.DeliverabilityStatisticsResponse), args.Error(1)
}

func (m *MockClient) GetBounceStatistics(params requests.GetBounceStatisticsParams) (*responses.BounceStatisticsResponse, error) {
	args := m.Called(params)
	return args.Get(0).(*responses.BounceStatisticsResponse), args.Error(1)
}

func (m *MockClient) GetDeliveryTimeStatistics(params requests.GetDeliveryTimeStatisticsParams) (*responses.DeliveryTimeStatisticsResponse, error) {
	args := m.Called(params)
	return args.Get(0).(*responses.DeliveryTimeStatisticsResponse), args.Error(1)
}

// API Key operations methods
func (m *MockClient) ListAPIKeys(limit *int32, cursor *string) (*responses.PaginatedAPIKeysResponse, error) {
	args := m.Called(limit, cursor)
	return args.Get(0).(*responses.PaginatedAPIKeysResponse), args.Error(1)
}

func (m *MockClient) GetAPIKey(keyID string) (*responses.APIKey, error) {
	args := m.Called(keyID)
	return args.Get(0).(*responses.APIKey), args.Error(1)
}

func (m *MockClient) CreateAPIKey(req requests.CreateAPIKeyRequest) (*responses.APIKey, error) {
	args := m.Called(req)
	return args.Get(0).(*responses.APIKey), args.Error(1)
}

func (m *MockClient) UpdateAPIKey(keyID string, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error) {
	args := m.Called(keyID, req)
	return args.Get(0).(*responses.APIKey), args.Error(1)
}

func (m *MockClient) DeleteAPIKey(keyID string) (*common.SuccessResponse, error) {
	args := m.Called(keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.SuccessResponse), args.Error(1)
}

// WebSocket methods for webhook streaming

func (m *MockClient) InitiateWebhookStream(webhookID string) (*client.WebhookStreamResponse, error) {
	args := m.Called(webhookID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.WebhookStreamResponse), args.Error(1)
}

func (m *MockClient) ConnectWebSocket(wsURL, webhookID string, forceReconnect, skipVerify bool) (*client.WebSocketClient, error) {
	args := m.Called(wsURL, webhookID, forceReconnect, skipVerify)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.WebSocketClient), args.Error(1)
}
