// Package mocks holds MockClient, a testify mock of client.AhaSendClient,
// with which tests run commands without calling the API, and fixtures of
// the resources commands print. APIResponse records a body behind a mocked
// result, as the real client does, so a test can check JSON output.
package mocks

import (
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
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

// withOptions appends each request option to a mocked call's arguments, so
// tests match options with mock.MatchedBy.
func withOptions(args []interface{}, opts []api.RequestOption) []interface{} {
	for _, opt := range opts {
		args = append(args, opt)
	}
	return args
}

// result returns a mocked call's (*T, error) results. A nil first result,
// typed or untyped, returns a nil *T.
func result[T any](args mock.Arguments) (*T, error) {
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*T), args.Error(1)
}

// Authentication and account info methods

func (m *MockClient) GetAccountID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockClient) GetAccount() (*responses.Account, error) {
	return result[responses.Account](m.Called())
}

func (m *MockClient) Ping() (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called())
}

// Message operations methods

func (m *MockClient) SendMessageWithIdempotencyKey(req requests.CreateMessageRequest, idempotencyKey string) (*responses.CreateMessageResponse, error) {
	return result[responses.CreateMessageResponse](m.Called(req, idempotencyKey))
}

func (m *MockClient) SendTemplateMessageWithIdempotencyKey(req requests.CreateTemplateMessageRequest, idempotencyKey string) (*responses.CreateMessageResponse, error) {
	return result[responses.CreateMessageResponse](m.Called(req, idempotencyKey))
}

func (m *MockClient) CancelMessage(messageID string) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(messageID))
}

func (m *MockClient) GetMessages(params requests.GetMessagesParams) (*responses.PaginatedMessagesResponse, error) {
	return result[responses.PaginatedMessagesResponse](m.Called(params))
}

func (m *MockClient) GetMessage(messageID string) (*responses.Message, error) {
	return result[responses.Message](m.Called(messageID))
}

// Domain operations methods

func (m *MockClient) ListDomains(params requests.GetDomainsParams) (*responses.PaginatedDomainsResponse, error) {
	return result[responses.PaginatedDomainsResponse](m.Called(params))
}

func (m *MockClient) CreateDomain(req requests.CreateDomainRequest, opts ...api.RequestOption) (*responses.Domain, error) {
	return result[responses.Domain](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) GetDomain(domain string) (*responses.Domain, error) {
	return result[responses.Domain](m.Called(domain))
}

func (m *MockClient) UpdateDomain(domain string, req requests.UpdateDomainRequest) (*responses.Domain, error) {
	return result[responses.Domain](m.Called(domain, req))
}

func (m *MockClient) CheckDomainDNS(domain string) (*responses.Domain, error) {
	return result[responses.Domain](m.Called(domain))
}

func (m *MockClient) DeleteDomain(domain string) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(domain))
}

// Webhook operations methods

func (m *MockClient) ListWebhooks(pagination common.PaginationParams, enabled *bool) (*responses.PaginatedWebhooksResponse, error) {
	return result[responses.PaginatedWebhooksResponse](m.Called(pagination, enabled))
}

func (m *MockClient) CreateWebhook(req requests.CreateWebhookRequest, opts ...api.RequestOption) (*responses.Webhook, error) {
	return result[responses.Webhook](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) GetWebhook(webhookID uuid.UUID) (*responses.Webhook, error) {
	return result[responses.Webhook](m.Called(webhookID))
}

func (m *MockClient) UpdateWebhook(webhookID uuid.UUID, req requests.UpdateWebhookRequest) (*responses.Webhook, error) {
	return result[responses.Webhook](m.Called(webhookID, req))
}

func (m *MockClient) DeleteWebhook(webhookID uuid.UUID) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(webhookID))
}

func (m *MockClient) TriggerWebhook(webhookID uuid.UUID, events []string) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(webhookID, events))
}

// Route operations methods

func (m *MockClient) ListRoutes(params requests.GetRoutesParams) (*responses.PaginatedRoutesResponse, error) {
	return result[responses.PaginatedRoutesResponse](m.Called(params))
}

func (m *MockClient) CreateRoute(req requests.CreateRouteRequest, opts ...api.RequestOption) (*responses.Route, error) {
	return result[responses.Route](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) GetRoute(routeID uuid.UUID) (*responses.Route, error) {
	return result[responses.Route](m.Called(routeID))
}

func (m *MockClient) UpdateRoute(routeID uuid.UUID, req requests.UpdateRouteRequest) (*responses.Route, error) {
	return result[responses.Route](m.Called(routeID, req))
}

func (m *MockClient) DeleteRoute(routeID uuid.UUID) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(routeID))
}

// SMTP operations methods

func (m *MockClient) ListSMTPCredentials(pagination common.PaginationParams) (*responses.PaginatedSMTPCredentialsResponse, error) {
	return result[responses.PaginatedSMTPCredentialsResponse](m.Called(pagination))
}

func (m *MockClient) GetSMTPCredential(credentialID uuid.UUID) (*responses.SMTPCredential, error) {
	return result[responses.SMTPCredential](m.Called(credentialID))
}

func (m *MockClient) CreateSMTPCredential(req requests.CreateSMTPCredentialRequest, opts ...api.RequestOption) (*responses.SMTPCredential, error) {
	return result[responses.SMTPCredential](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) DeleteSMTPCredential(credentialID uuid.UUID) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(credentialID))
}

// Suppression operations methods

func (m *MockClient) ListSuppressions(params requests.GetSuppressionsParams) (*responses.PaginatedSuppressionsResponse, error) {
	return result[responses.PaginatedSuppressionsResponse](m.Called(params))
}

func (m *MockClient) CreateSuppression(req requests.CreateSuppressionRequest, opts ...api.RequestOption) (*responses.CreateSuppressionResponse, error) {
	return result[responses.CreateSuppressionResponse](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) DeleteSuppression(email string, domain *string) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(email, domain))
}

func (m *MockClient) WipeSuppressions(domain *string) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(domain))
}

// Statistics operations methods
func (m *MockClient) GetDeliverabilityStatistics(params requests.GetDeliverabilityStatisticsParams) (*responses.DeliverabilityStatisticsResponse, error) {
	return result[responses.DeliverabilityStatisticsResponse](m.Called(params))
}

func (m *MockClient) GetBounceStatistics(params requests.GetBounceStatisticsParams) (*responses.BounceStatisticsResponse, error) {
	return result[responses.BounceStatisticsResponse](m.Called(params))
}

func (m *MockClient) GetDeliveryTimeStatistics(params requests.GetDeliveryTimeStatisticsParams) (*responses.DeliveryTimeStatisticsResponse, error) {
	return result[responses.DeliveryTimeStatisticsResponse](m.Called(params))
}

// Sub-account operations methods
func (m *MockClient) ListSubAccounts(pagination common.PaginationParams) (*responses.PaginatedSubAccountsResponse, error) {
	return result[responses.PaginatedSubAccountsResponse](m.Called(pagination))
}

func (m *MockClient) CreateSubAccount(req requests.CreateSubAccountRequest, opts ...api.RequestOption) (*responses.SubAccount, error) {
	return result[responses.SubAccount](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) GetSubAccountsUsage() (*responses.SubAccountUsageResponse, error) {
	return result[responses.SubAccountUsageResponse](m.Called())
}

func (m *MockClient) GetSubAccount(subAccountID uuid.UUID) (*responses.SubAccount, error) {
	return result[responses.SubAccount](m.Called(subAccountID))
}

func (m *MockClient) UpdateSubAccount(subAccountID uuid.UUID, req requests.UpdateSubAccountRequest) (*responses.SubAccount, error) {
	return result[responses.SubAccount](m.Called(subAccountID, req))
}

func (m *MockClient) DeleteSubAccount(subAccountID uuid.UUID) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(subAccountID))
}

func (m *MockClient) SuspendSubAccount(subAccountID uuid.UUID, req requests.SuspendSubAccountRequest) (*responses.SubAccount, error) {
	return result[responses.SubAccount](m.Called(subAccountID, req))
}

func (m *MockClient) UnsuspendSubAccount(subAccountID uuid.UUID) (*responses.SubAccount, error) {
	return result[responses.SubAccount](m.Called(subAccountID))
}

func (m *MockClient) UnpauseSubAccountDomain(subAccountID uuid.UUID, domain string) (*responses.Domain, error) {
	return result[responses.Domain](m.Called(subAccountID, domain))
}

// Sub-account API key operations methods
func (m *MockClient) ListSubAccountAPIKeys(subAccountID uuid.UUID, pagination common.PaginationParams) (*responses.PaginatedAPIKeysResponse, error) {
	return result[responses.PaginatedAPIKeysResponse](m.Called(subAccountID, pagination))
}

func (m *MockClient) CreateSubAccountAPIKey(subAccountID uuid.UUID, req requests.CreateAPIKeyRequest, opts ...api.RequestOption) (*responses.APIKey, error) {
	return result[responses.APIKey](m.Called(withOptions([]interface{}{subAccountID, req}, opts)...))
}

func (m *MockClient) GetSubAccountAPIKey(subAccountID, keyID uuid.UUID) (*responses.APIKey, error) {
	return result[responses.APIKey](m.Called(subAccountID, keyID))
}

func (m *MockClient) UpdateSubAccountAPIKey(subAccountID, keyID uuid.UUID, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error) {
	return result[responses.APIKey](m.Called(subAccountID, keyID, req))
}

func (m *MockClient) DeleteSubAccountAPIKey(subAccountID, keyID uuid.UUID) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(subAccountID, keyID))
}

// API Key operations methods
func (m *MockClient) ListAPIKeys(pagination common.PaginationParams) (*responses.PaginatedAPIKeysResponse, error) {
	return result[responses.PaginatedAPIKeysResponse](m.Called(pagination))
}

func (m *MockClient) GetAPIKey(keyID uuid.UUID) (*responses.APIKey, error) {
	return result[responses.APIKey](m.Called(keyID))
}

func (m *MockClient) CreateAPIKey(req requests.CreateAPIKeyRequest, opts ...api.RequestOption) (*responses.APIKey, error) {
	return result[responses.APIKey](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) UpdateAPIKey(keyID uuid.UUID, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error) {
	return result[responses.APIKey](m.Called(keyID, req))
}

func (m *MockClient) DeleteAPIKey(keyID uuid.UUID) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(keyID))
}

// WebSocket methods for webhook streaming

func (m *MockClient) InitiateWebhookStream(webhookID *uuid.UUID) (*client.WebhookStreamResponse, error) {
	return result[client.WebhookStreamResponse](m.Called(webhookID))
}

func (m *MockClient) InitiateRouteStream(routeID *uuid.UUID, recipient string) (*client.RouteStreamResponse, error) {
	return result[client.RouteStreamResponse](m.Called(routeID, recipient))
}

func (m *MockClient) TriggerRoute(routeID uuid.UUID) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(routeID))
}

func (m *MockClient) ConnectWebSocket(wsURL string, forceReconnect bool) (*client.WebSocketClient, error) {
	return result[client.WebSocketClient](m.Called(wsURL, forceReconnect))
}

// Contact operations methods

func (m *MockClient) GetContacts(params requests.GetContactsParams) (*responses.PaginatedContactsResponse, error) {
	return result[responses.PaginatedContactsResponse](m.Called(params))
}

func (m *MockClient) GetContact(idOrEmail string) (*responses.Contact, error) {
	return result[responses.Contact](m.Called(idOrEmail))
}

func (m *MockClient) CreateContact(req requests.CreateContactRequest, opts ...api.RequestOption) (*responses.Contact, error) {
	return result[responses.Contact](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) UpdateContact(idOrEmail string, req requests.UpdateContactRequest) (*responses.Contact, error) {
	return result[responses.Contact](m.Called(idOrEmail, req))
}

func (m *MockClient) DeleteContact(idOrEmail string) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(idOrEmail))
}

func (m *MockClient) BatchUpsertContacts(req requests.BatchUpsertContactsRequest, opts ...api.RequestOption) (*responses.BatchUpsertContactsResponse, error) {
	return result[responses.BatchUpsertContactsResponse](m.Called(withOptions([]interface{}{req}, opts)...))
}

// List operations methods

func (m *MockClient) GetLists(params requests.GetListsParams) (*responses.PaginatedContactListsResponse, error) {
	return result[responses.PaginatedContactListsResponse](m.Called(params))
}

func (m *MockClient) GetList(listID uuid.UUID) (*responses.ContactList, error) {
	return result[responses.ContactList](m.Called(listID))
}

func (m *MockClient) CreateList(req requests.CreateListRequest, opts ...api.RequestOption) (*responses.ContactList, error) {
	return result[responses.ContactList](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) UpdateList(listID uuid.UUID, req requests.UpdateListRequest) (*responses.ContactList, error) {
	return result[responses.ContactList](m.Called(listID, req))
}

func (m *MockClient) DeleteList(listID uuid.UUID) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(listID))
}

func (m *MockClient) GetListContacts(listID uuid.UUID, params requests.GetListContactsParams) (*responses.PaginatedListContactsResponse, error) {
	return result[responses.PaginatedListContactsResponse](m.Called(listID, params))
}

func (m *MockClient) BatchAddListContacts(listID uuid.UUID, req requests.BatchAddListContactsRequest, opts ...api.RequestOption) (*responses.BatchAddListContactsResponse, error) {
	return result[responses.BatchAddListContactsResponse](m.Called(withOptions([]interface{}{listID, req}, opts)...))
}

func (m *MockClient) UpsertListContact(listID uuid.UUID, idOrEmail string, req requests.UpsertListContactRequest) (*responses.ListContact, error) {
	return result[responses.ListContact](m.Called(listID, idOrEmail, req))
}

func (m *MockClient) DeleteListContact(listID uuid.UUID, idOrEmail string) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(listID, idOrEmail))
}

func (m *MockClient) GetContactLists(idOrEmail string, params requests.GetContactListsParams) (*responses.PaginatedListContactsResponse, error) {
	return result[responses.PaginatedListContactsResponse](m.Called(idOrEmail, params))
}

// Template operations methods

func (m *MockClient) GetTemplates(params requests.GetTemplatesParams) (*responses.PaginatedTemplatesResponse, error) {
	return result[responses.PaginatedTemplatesResponse](m.Called(params))
}

func (m *MockClient) GetTemplate(templateID uuid.UUID) (*responses.Template, error) {
	return result[responses.Template](m.Called(templateID))
}

func (m *MockClient) CreateTemplate(req requests.CreateTemplateRequest, opts ...api.RequestOption) (*responses.Template, error) {
	return result[responses.Template](m.Called(withOptions([]interface{}{req}, opts)...))
}

func (m *MockClient) UpdateTemplate(templateID uuid.UUID, req requests.UpdateTemplateRequest, opts ...api.RequestOption) (*responses.Template, error) {
	return result[responses.Template](m.Called(withOptions([]interface{}{templateID, req}, opts)...))
}

func (m *MockClient) DeleteTemplate(templateID uuid.UUID) (*common.SuccessResponse, error) {
	return result[common.SuccessResponse](m.Called(templateID))
}

func (m *MockClient) GetTemplateDraft(templateID uuid.UUID) (*responses.TemplateDraft, error) {
	return result[responses.TemplateDraft](m.Called(templateID))
}

func (m *MockClient) DiscardTemplateDraft(templateID uuid.UUID) (*responses.Template, error) {
	return result[responses.Template](m.Called(templateID))
}

func (m *MockClient) PublishTemplate(templateID uuid.UUID, opts ...api.RequestOption) (*responses.Template, error) {
	return result[responses.Template](m.Called(withOptions([]interface{}{templateID}, opts)...))
}

func (m *MockClient) GetTemplateVersions(templateID uuid.UUID) (*responses.TemplateVersionsResponse, error) {
	return result[responses.TemplateVersionsResponse](m.Called(templateID))
}

func (m *MockClient) GetTemplateVersion(templateID, versionID uuid.UUID) (*responses.TemplateVersionDetail, error) {
	return result[responses.TemplateVersionDetail](m.Called(templateID, versionID))
}

func (m *MockClient) RestoreTemplateVersion(templateID, versionID uuid.UUID, req requests.RestoreTemplateVersionRequest, opts ...api.RequestOption) (*responses.Template, error) {
	return result[responses.Template](m.Called(withOptions([]interface{}{templateID, versionID, req}, opts)...))
}
