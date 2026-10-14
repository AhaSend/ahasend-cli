package client

import (
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
)

// AhaSendClient defines the interface for AhaSend API operations
// This interface allows for better testability and mocking
type AhaSendClient interface {
	// Authentication and account info
	GetAccountID() string
	GetAccount() (*responses.Account, error)
	Ping() (*common.SuccessResponse, error)

	// Message operations
	SendMessageWithIdempotencyKey(req requests.CreateMessageRequest, idempotencyKey string) (*responses.CreateMessageResponse, error)
	SendTemplateMessageWithIdempotencyKey(req requests.CreateTemplateMessageRequest, idempotencyKey string) (*responses.CreateMessageResponse, error)
	CancelMessage(messageID string) (*common.SuccessResponse, error)
	GetMessages(params requests.GetMessagesParams) (*responses.PaginatedMessagesResponse, error)
	GetMessage(messageID string) (*responses.Message, error)

	// Domain operations
	ListDomains(params requests.GetDomainsParams) (*responses.PaginatedDomainsResponse, error)
	CreateDomain(req requests.CreateDomainRequest, opts ...api.RequestOption) (*responses.Domain, error)
	GetDomain(domain string) (*responses.Domain, error)
	UpdateDomain(domain string, req requests.UpdateDomainRequest) (*responses.Domain, error)
	CheckDomainDNS(domain string) (*responses.Domain, error)
	DeleteDomain(domain string) (*common.SuccessResponse, error)

	// Webhook operations
	ListWebhooks(pagination common.PaginationParams, enabled *bool) (*responses.PaginatedWebhooksResponse, error)
	CreateWebhook(req requests.CreateWebhookRequest, opts ...api.RequestOption) (*responses.Webhook, error)
	GetWebhook(webhookID uuid.UUID) (*responses.Webhook, error)
	UpdateWebhook(webhookID uuid.UUID, req requests.UpdateWebhookRequest) (*responses.Webhook, error)
	DeleteWebhook(webhookID uuid.UUID) (*common.SuccessResponse, error)

	// Webhook streaming operations (development only)
	InitiateWebhookStream(webhookID *uuid.UUID) (*WebhookStreamResponse, error)
	ConnectWebSocket(wsURL string, forceReconnect bool) (*WebSocketClient, error)
	TriggerWebhook(webhookID uuid.UUID, events []string) (*common.SuccessResponse, error)

	// Route operations
	ListRoutes(params requests.GetRoutesParams) (*responses.PaginatedRoutesResponse, error)
	CreateRoute(req requests.CreateRouteRequest, opts ...api.RequestOption) (*responses.Route, error)
	GetRoute(routeID uuid.UUID) (*responses.Route, error)
	UpdateRoute(routeID uuid.UUID, req requests.UpdateRouteRequest) (*responses.Route, error)
	DeleteRoute(routeID uuid.UUID) (*common.SuccessResponse, error)

	// Route streaming operations (development only)
	InitiateRouteStream(routeID *uuid.UUID, recipient string) (*RouteStreamResponse, error)
	TriggerRoute(routeID uuid.UUID) (*common.SuccessResponse, error)

	// Suppression operations
	ListSuppressions(params requests.GetSuppressionsParams) (*responses.PaginatedSuppressionsResponse, error)
	CreateSuppression(req requests.CreateSuppressionRequest, opts ...api.RequestOption) (*responses.CreateSuppressionResponse, error)
	DeleteSuppression(email string, domain *string) (*common.SuccessResponse, error)
	WipeSuppressions(domains *string) (*common.SuccessResponse, error)

	// SMTP operations
	ListSMTPCredentials(pagination common.PaginationParams) (*responses.PaginatedSMTPCredentialsResponse, error)
	GetSMTPCredential(credentialID uuid.UUID) (*responses.SMTPCredential, error)
	CreateSMTPCredential(req requests.CreateSMTPCredentialRequest, opts ...api.RequestOption) (*responses.SMTPCredential, error)
	DeleteSMTPCredential(credentialID uuid.UUID) (*common.SuccessResponse, error)

	// Statistics operations
	GetDeliverabilityStatistics(params requests.GetDeliverabilityStatisticsParams) (*responses.DeliverabilityStatisticsResponse, error)
	GetBounceStatistics(params requests.GetBounceStatisticsParams) (*responses.BounceStatisticsResponse, error)
	GetDeliveryTimeStatistics(params requests.GetDeliveryTimeStatisticsParams) (*responses.DeliveryTimeStatisticsResponse, error)

	// Sub-account operations
	ListSubAccounts(pagination common.PaginationParams) (*responses.PaginatedSubAccountsResponse, error)
	CreateSubAccount(req requests.CreateSubAccountRequest, opts ...api.RequestOption) (*responses.SubAccount, error)
	GetSubAccountsUsage() (*responses.SubAccountUsageResponse, error)
	GetSubAccount(subAccountID uuid.UUID) (*responses.SubAccount, error)
	UpdateSubAccount(subAccountID uuid.UUID, req requests.UpdateSubAccountRequest) (*responses.SubAccount, error)
	DeleteSubAccount(subAccountID uuid.UUID) (*common.SuccessResponse, error)
	SuspendSubAccount(subAccountID uuid.UUID, req requests.SuspendSubAccountRequest) (*responses.SubAccount, error)
	UnsuspendSubAccount(subAccountID uuid.UUID) (*responses.SubAccount, error)
	UnpauseSubAccountDomain(subAccountID uuid.UUID, domain string) (*responses.Domain, error)

	// Sub-account API key operations
	ListSubAccountAPIKeys(subAccountID uuid.UUID, pagination common.PaginationParams) (*responses.PaginatedAPIKeysResponse, error)
	CreateSubAccountAPIKey(subAccountID uuid.UUID, req requests.CreateAPIKeyRequest, opts ...api.RequestOption) (*responses.APIKey, error)
	GetSubAccountAPIKey(subAccountID, keyID uuid.UUID) (*responses.APIKey, error)
	UpdateSubAccountAPIKey(subAccountID, keyID uuid.UUID, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error)
	DeleteSubAccountAPIKey(subAccountID, keyID uuid.UUID) (*common.SuccessResponse, error)

	// API Key operations
	ListAPIKeys(pagination common.PaginationParams) (*responses.PaginatedAPIKeysResponse, error)
	GetAPIKey(keyID uuid.UUID) (*responses.APIKey, error)
	CreateAPIKey(req requests.CreateAPIKeyRequest, opts ...api.RequestOption) (*responses.APIKey, error)
	UpdateAPIKey(keyID uuid.UUID, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error)
	DeleteAPIKey(keyID uuid.UUID) (*common.SuccessResponse, error)
}
