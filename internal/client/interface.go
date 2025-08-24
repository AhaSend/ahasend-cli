package client

import (
	"context"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/AhaSend/ahasend-go/webhooks"
)

// AhaSendClient defines the interface for AhaSend API operations
// This interface allows for better testability and mocking
type AhaSendClient interface {
	// Authentication and account info
	GetAccountID() string
	GetAuthContext() context.Context
	GetAccount() (*responses.Account, error)
	Ping() error
	ValidateConfiguration() error

	// Message operations
	SendMessage(req requests.CreateMessageRequest) (*responses.CreateMessageResponse, error)
	SendMessageWithIdempotencyKey(req requests.CreateMessageRequest, idempotencyKey string) (*responses.CreateMessageResponse, error)
	CancelMessage(accountID, messageID string) (*common.SuccessResponse, error)
	GetMessages(params requests.GetMessagesParams) (*responses.PaginatedMessagesResponse, error)

	// Domain operations
	ListDomains(limit *int32, cursor *string) (*responses.PaginatedDomainsResponse, error)
	CreateDomain(domain string) (*responses.Domain, error)
	GetDomain(domain string) (*responses.Domain, error)
	DeleteDomain(domain string) (*common.SuccessResponse, error)

	// Webhook operations
	CreateWebhookVerifier(secret string) (*webhooks.WebhookVerifier, error)
	ListWebhooks(limit *int32, cursor *string) (*responses.PaginatedWebhooksResponse, error)
	CreateWebhook(req requests.CreateWebhookRequest) (*responses.Webhook, error)
	GetWebhook(webhookID string) (*responses.Webhook, error)
	UpdateWebhook(webhookID string, req requests.UpdateWebhookRequest) (*responses.Webhook, error)
	DeleteWebhook(webhookID string) error
	
	// Webhook streaming operations (development only)
	InitiateWebhookStream(webhookID string) (*WebhookStreamResponse, error)
	ConnectWebSocket(wsURL, webhookID string, forceReconnect, skipVerify bool) (*WebSocketClient, error)
	TriggerWebhook(webhookID string, events []string) error

	// Route operations
	ListRoutes(limit *int32, cursor *string) (*responses.PaginatedRoutesResponse, error)
	CreateRoute(req requests.CreateRouteRequest) (*responses.Route, error)
	GetRoute(routeID string) (*responses.Route, error)
	UpdateRoute(routeID string, req requests.UpdateRouteRequest) (*responses.Route, error)
	DeleteRoute(routeID string) error

	// Suppression operations
	ListSuppressions(params requests.GetSuppressionsParams) (*responses.PaginatedSuppressionsResponse, error)
	CreateSuppression(req requests.CreateSuppressionRequest) (*responses.CreateSuppressionResponse, error)
	DeleteSuppression(email string, domain *string) (*common.SuccessResponse, error)
	WipeSuppressions(domains *string) (*common.SuccessResponse, error)

	// SMTP operations
	ListSMTPCredentials(limit *int32, cursor *string) (*responses.PaginatedSMTPCredentialsResponse, error)
	GetSMTPCredential(credentialID string) (*responses.SMTPCredential, error)
	CreateSMTPCredential(req requests.CreateSMTPCredentialRequest) (*responses.SMTPCredential, error)
	DeleteSMTPCredential(credentialID string) error

	// Statistics operations
	GetDeliverabilityStatistics(params requests.GetDeliverabilityStatisticsParams) (*responses.DeliverabilityStatisticsResponse, error)
	GetBounceStatistics(params requests.GetBounceStatisticsParams) (*responses.BounceStatisticsResponse, error)
	GetDeliveryTimeStatistics(params requests.GetDeliveryTimeStatisticsParams) (*responses.DeliveryTimeStatisticsResponse, error)

	// API Key operations
	ListAPIKeys(limit *int32, cursor *string) (*responses.PaginatedAPIKeysResponse, error)
	GetAPIKey(keyID string) (*responses.APIKey, error)
	CreateAPIKey(req requests.CreateAPIKeyRequest) (*responses.APIKey, error)
	UpdateAPIKey(keyID string, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error)
	DeleteAPIKey(keyID string) (*common.SuccessResponse, error)
}
