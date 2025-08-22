// Package client provides a wrapper around the AhaSend SDK with enhanced functionality.
//
// This package implements the AhaSendClient interface and provides additional
// features beyond the base SDK:
//
//   - Rate limiting (50 requests/second with 100 burst capacity)
//   - Automatic retry logic with exponential backoff
//   - HTTP request/response logging for debugging
//   - Structured error handling and API error translation
//   - Context-aware request handling
//   - Idempotency key support for message sending
//   - Configuration validation and connection testing
//
// The Client struct wraps the official AhaSend SDK while maintaining full
// compatibility and adding CLI-specific enhancements for better user experience.
package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/AhaSend/ahasend-go/webhooks"
	"github.com/google/uuid"

	"github.com/AhaSend/ahasend-cli/internal/logger"
)

// Client wraps the AhaSend SDK client with additional functionality
type Client struct {
	*api.APIClient
	config      *api.Configuration
	auth        context.Context
	accountID   string
	rateLimiter *RateLimiter
}

// NewClient creates a new AhaSend client with rate limiting
func NewClient(apiKey, accountID string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}

	config := api.NewConfiguration()

	// Configure retry behavior using new SDK RetryConfig
	config.RetryConfig = api.RetryConfig{
		Enabled:               true,
		MaxRetries:            3,
		RetryClientErrors:     false, // Never retry 4xx errors
		RetryValidationErrors: false, // Never retry validation errors
		BackoffStrategy:       api.BackoffExponential,
		BaseDelay:             1000 * time.Millisecond,  // 1 second base delay
		MaxDelay:              30000 * time.Millisecond, // 30 second max delay
	}

	// Set custom user agent
	config.UserAgent = fmt.Sprintf("ahasend-cli/1.0.0 %s", config.UserAgent)

	// Add HTTP logging transport
	httpTransport := logger.NewHTTPTransport(http.DefaultTransport, logger.Get())
	config.HTTPClient = &http.Client{
		Transport: httpTransport,
		Timeout:   30 * time.Second,
	}

	// Create authenticated context
	auth := context.WithValue(context.Background(), api.ContextAccessToken, apiKey)

	// Create rate limiter (50 req/sec with 100 burst capacity as per SDK docs)
	rateLimiter := NewRateLimiter(50, 100)

	client := &Client{
		APIClient:   api.NewAPIClientWithConfig(config),
		config:      config,
		auth:        auth,
		accountID:   accountID,
		rateLimiter: rateLimiter,
	}

	return client, nil
}

// GetAccountID returns the configured account ID
func (c *Client) GetAccountID() string {
	return c.accountID
}

// GetAuthContext returns the authenticated context
func (c *Client) GetAuthContext() context.Context {
	return c.auth
}

// GetAccount retrieves account information
func (c *Client) GetAccount() (*responses.Account, error) {
	// Parse account ID as UUID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	// Call the account API
	account, _, err := c.AccountsAPI.GetAccount(c.auth, accountUUID)
	return account, err
}

// Ping tests the connection and validates the API key
func (c *Client) Ping() error {
	_, _, err := c.UtilityAPI.Ping(c.auth)
	return err
}

// SendMessage sends a message with retry and rate limiting
func (c *Client) SendMessage(req requests.CreateMessageRequest) (*responses.CreateMessageResponse, error) {
	return c.SendMessageWithIdempotencyKey(req, "")
}

// SendMessageWithIdempotencyKey sends a message with idempotency key support
func (c *Client) SendMessageWithIdempotencyKey(req requests.CreateMessageRequest, idempotencyKey string) (*responses.CreateMessageResponse, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.MessagesAPI.CreateMessage(c.auth, accountUUID, req, api.WithIdempotencyKey(idempotencyKey))

	return response, err
}

// CancelMessage cancels a scheduled message
func (c *Client) CancelMessage(accountID, messageID string) (*common.SuccessResponse, error) {
	accountUUID, err := uuid.Parse(accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.MessagesAPI.CancelMessage(c.auth, accountUUID, messageID)
	return response, err
}

// ListDomains lists domains with pagination
// @TODO: Add DNSValid parameter
func (c *Client) ListDomains(limit *int32, cursor *string) (*responses.PaginatedDomainsResponse, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.DomainsAPI.GetDomains(c.auth, accountUUID, nil, limit, cursor)
	return response, err
}

// CreateDomain creates a new domain
func (c *Client) CreateDomain(domain string) (*responses.Domain, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	req := requests.CreateDomainRequest{
		Domain: domain,
	}

	response, _, err := c.DomainsAPI.CreateDomain(c.auth, accountUUID, req, api.WithIdempotencyKey(uuid.NewString()))
	return response, err
}

// GetDomain gets a specific domain
func (c *Client) GetDomain(domain string) (*responses.Domain, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.DomainsAPI.GetDomain(c.auth, accountUUID, domain)
	return response, err
}

// DeleteDomain deletes a domain
func (c *Client) DeleteDomain(domain string) (*common.SuccessResponse, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.DomainsAPI.DeleteDomain(c.auth, accountUUID, domain)
	return response, err
}

// GetMessages retrieves messages with filtering and pagination
func (c *Client) GetMessages(params requests.GetMessagesParams) (*responses.PaginatedMessagesResponse, error) {
	startTime := time.Now()

	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	// Build endpoint URL for logging
	endpoint := fmt.Sprintf("/v2/accounts/%s/messages", c.accountID)

	// Log API call parameters
	logger.Get().WithFields(map[string]interface{}{
		"method":     "GET",
		"endpoint":   endpoint,
		"account_id": c.accountID,
		"sender":     params.Sender,
		"recipient":  params.Recipient,
		"subject":    params.Subject,
		"message_id": params.MessageIDHeader,
		"from_time":  params.FromTime,
		"to_time":    params.ToTime,
		"limit":      params.Limit,
		"cursor":     params.Cursor,
	}).Debug("API Request")

	response, _, err := c.MessagesAPI.GetMessages(c.auth, accountUUID, params)

	duration := time.Since(startTime)

	if err != nil {
		logger.APIError("GET", endpoint, 0, err, duration)
	} else {
		logger.APICall("GET", endpoint, duration)
		logger.Get().WithFields(map[string]interface{}{
			"messages_count": len(response.Data),
			"has_more":       response.Pagination.HasMore,
		}).Debug("API Response")
	}

	return response, err
}

// CreateWebhookVerifier creates a webhook verifier for the given secret
func (c *Client) CreateWebhookVerifier(secret string) (*webhooks.WebhookVerifier, error) {
	verifier, err := webhooks.NewWebhookVerifier(secret)
	return verifier, err
}

// ListWebhooks retrieves all webhooks with pagination support
func (c *Client) ListWebhooks(limit *int32, cursor *string) (*responses.PaginatedWebhooksResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	params := api.GetWebhooksParams{
		Limit:  limit,
		Cursor: cursor,
	}
	response, _, err := c.WebhooksAPI.GetWebhooks(c.auth, accountUUID, params)

	return response, err
}

// CreateWebhook creates a new webhook
func (c *Client) CreateWebhook(req requests.CreateWebhookRequest) (*responses.Webhook, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.WebhooksAPI.CreateWebhook(c.auth, accountUUID, req)

	return response, err
}

// GetWebhook retrieves a single webhook by ID
func (c *Client) GetWebhook(webhookID string) (*responses.Webhook, error) {
	// Ensure we have valid UUIDs
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	webhookUUID, err := uuid.Parse(webhookID)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook ID format: %w", err)
	}

	response, _, err := c.WebhooksAPI.GetWebhook(c.auth, accountUUID, webhookUUID)

	return response, err
}

// UpdateWebhook updates an existing webhook
func (c *Client) UpdateWebhook(webhookID string, req requests.UpdateWebhookRequest) (*responses.Webhook, error) {
	// Ensure we have valid UUIDs
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	webhookUUID, err := uuid.Parse(webhookID)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook ID format: %w", err)
	}

	response, _, err := c.WebhooksAPI.UpdateWebhook(c.auth, accountUUID, webhookUUID, req)

	return response, err
}

// DeleteWebhook deletes a webhook
func (c *Client) DeleteWebhook(webhookID string) error {
	// Ensure we have valid UUIDs
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return fmt.Errorf("invalid account ID format: %w", err)
	}

	webhookUUID, err := uuid.Parse(webhookID)
	if err != nil {
		return fmt.Errorf("invalid webhook ID format: %w", err)
	}

	_, _, err = c.WebhooksAPI.DeleteWebhook(c.auth, accountUUID, webhookUUID)
	return err
}

// ListRoutes retrieves a paginated list of routes
func (c *Client) ListRoutes(limit *int32, cursor *string) (*responses.PaginatedRoutesResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.RoutesAPI.GetRoutes(c.auth, accountUUID, limit, cursor)
	return response, err
}

// CreateRoute creates a new route
func (c *Client) CreateRoute(req requests.CreateRouteRequest) (*responses.Route, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.RoutesAPI.CreateRoute(c.auth, accountUUID, req)

	return response, err
}

// GetRoute retrieves a single route by ID
func (c *Client) GetRoute(routeID string) (*responses.Route, error) {
	// Ensure we have valid UUIDs
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	routeUUID, err := uuid.Parse(routeID)
	if err != nil {
		return nil, fmt.Errorf("invalid route ID format: %w", err)
	}

	response, _, err := c.RoutesAPI.GetRoute(c.auth, accountUUID, routeUUID)
	return response, err
}

// UpdateRoute updates an existing route
func (c *Client) UpdateRoute(routeID string, req requests.UpdateRouteRequest) (*responses.Route, error) {
	// Ensure we have valid UUIDs
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	routeUUID, err := uuid.Parse(routeID)
	if err != nil {
		return nil, fmt.Errorf("invalid route ID format: %w", err)
	}

	response, _, err := c.RoutesAPI.UpdateRoute(c.auth, accountUUID, routeUUID, req)
	return response, err
}

// DeleteRoute deletes a route
func (c *Client) DeleteRoute(routeID string) error {
	// Ensure we have valid UUIDs
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return fmt.Errorf("invalid account ID format: %w", err)
	}

	routeUUID, err := uuid.Parse(routeID)
	if err != nil {
		return fmt.Errorf("invalid route ID format: %w", err)
	}

	_, _, err = c.RoutesAPI.DeleteRoute(c.auth, accountUUID, routeUUID)
	return err
}

// ListSuppressions retrieves a paginated list of suppressions
func (c *Client) ListSuppressions(params requests.GetSuppressionsParams) (*responses.PaginatedSuppressionsResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.SuppressionsAPI.GetSuppressions(c.auth, accountUUID, params)
	return response, err
}

// CreateSuppression creates a new suppression
func (c *Client) CreateSuppression(req requests.CreateSuppressionRequest) (*responses.CreateSuppressionResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.SuppressionsAPI.CreateSuppression(c.auth, accountUUID, req)
	return response, err
}

// DeleteSuppression deletes a suppression by email and optional domain
func (c *Client) DeleteSuppression(email string, domain *string) (*common.SuccessResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.SuppressionsAPI.DeleteSuppression(c.auth, accountUUID, email, domain)
	return response, err
}

// WipeSuppressions deletes all suppressions in the account
func (c *Client) WipeSuppressions(domain *string) (*common.SuccessResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.SuppressionsAPI.DeleteAllSuppressions(c.auth, accountUUID, domain)
	return response, err
}

// ValidateConfiguration validates the client configuration
func (c *Client) ValidateConfiguration() error {
	if c.accountID == "" {
		return fmt.Errorf("account ID is required")
	}

	// Test the connection
	return c.Ping()
}

// ListSMTPCredentials lists all SMTP credentials with pagination
func (c *Client) ListSMTPCredentials(limit *int32, cursor *string) (*responses.PaginatedSMTPCredentialsResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	resp, _, err := c.SMTPCredentialsAPI.GetSMTPCredentials(c.auth, accountUUID, limit, cursor)
	return resp, err
}

// GetSMTPCredential gets a specific SMTP credential by ID
func (c *Client) GetSMTPCredential(credentialID string) (*responses.SMTPCredential, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	// Parse the credential ID as UUID
	credentialUUID, err := uuid.Parse(credentialID)
	if err != nil {
		return nil, fmt.Errorf("invalid credential ID format: %w", err)
	}

	resp, _, err := c.SMTPCredentialsAPI.GetSMTPCredential(c.auth, accountUUID, credentialUUID)
	return resp, err
}

// CreateSMTPCredential creates a new SMTP credential
func (c *Client) CreateSMTPCredential(req requests.CreateSMTPCredentialRequest) (*responses.SMTPCredential, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	resp, _, err := c.SMTPCredentialsAPI.CreateSMTPCredential(c.auth, accountUUID, req)
	return resp, err
}

// DeleteSMTPCredential deletes an SMTP credential by ID
func (c *Client) DeleteSMTPCredential(credentialID string) error {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return fmt.Errorf("invalid account ID format: %w", err)
	}

	// Parse the credential ID as UUID
	credentialUUID, err := uuid.Parse(credentialID)
	if err != nil {
		return fmt.Errorf("invalid credential ID format: %w", err)
	}

	_, _, err = c.SMTPCredentialsAPI.DeleteSMTPCredential(c.auth, accountUUID, credentialUUID)
	return err
}

// GetDeliverabilityStatistics retrieves deliverability statistics
func (c *Client) GetDeliverabilityStatistics(params requests.GetDeliverabilityStatisticsParams) (*responses.DeliverabilityStatisticsResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.StatisticsAPI.GetDeliverabilityStatistics(c.auth, accountUUID, params)
	return response, err
}

// GetBounceStatistics retrieves bounce statistics
func (c *Client) GetBounceStatistics(params requests.GetBounceStatisticsParams) (*responses.BounceStatisticsResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.StatisticsAPI.GetBounceStatistics(c.auth, accountUUID, params)
	return response, err
}

// GetDeliveryTimeStatistics retrieves delivery time statistics
func (c *Client) GetDeliveryTimeStatistics(params requests.GetDeliveryTimeStatisticsParams) (*responses.DeliveryTimeStatisticsResponse, error) {
	// Ensure we have a valid UUID for the account ID
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	response, _, err := c.StatisticsAPI.GetDeliveryTimeStatistics(c.auth, accountUUID, params)
	return response, err
}

// API Key operations

// ListAPIKeys retrieves a list of API keys
func (c *Client) ListAPIKeys(limit *int32, cursor *string) (*responses.PaginatedAPIKeysResponse, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID: %v", err)
	}

	response, _, err := c.APIKeysAPI.GetAPIKeys(c.auth, accountUUID, limit, cursor)
	return response, err
}

// GetAPIKey retrieves a specific API key by ID
func (c *Client) GetAPIKey(keyID string) (*responses.APIKey, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID: %v", err)
	}

	keyUUID, err := uuid.Parse(keyID)
	if err != nil {
		return nil, fmt.Errorf("invalid API key ID: %v", err)
	}

	response, _, err := c.APIKeysAPI.GetAPIKey(c.auth, accountUUID, keyUUID)
	return response, err
}

// CreateAPIKey creates a new API key
func (c *Client) CreateAPIKey(req requests.CreateAPIKeyRequest) (*responses.APIKey, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID: %v", err)
	}

	response, _, err := c.APIKeysAPI.CreateAPIKey(c.auth, accountUUID, req)
	return response, err
}

// UpdateAPIKey updates an existing API key
func (c *Client) UpdateAPIKey(keyID string, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID: %v", err)
	}

	keyUUID, err := uuid.Parse(keyID)
	if err != nil {
		return nil, fmt.Errorf("invalid API key ID: %v", err)
	}

	response, _, err := c.APIKeysAPI.UpdateAPIKey(c.auth, accountUUID, keyUUID, req)
	return response, err
}

// DeleteAPIKey deletes an API key
func (c *Client) DeleteAPIKey(keyID string) (*common.SuccessResponse, error) {
	accountUUID, err := uuid.Parse(c.accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID: %v", err)
	}

	keyUUID, err := uuid.Parse(keyID)
	if err != nil {
		return nil, fmt.Errorf("invalid API key ID: %v", err)
	}

	response, _, err := c.APIKeysAPI.DeleteAPIKey(c.auth, accountUUID, keyUUID)
	return response, err
}
