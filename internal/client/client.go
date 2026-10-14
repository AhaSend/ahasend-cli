// Package client provides a wrapper around the AhaSend SDK with enhanced functionality.
//
// This package implements the AhaSendClient interface and provides additional
// features beyond the base SDK:
//
//   - Automatic retry logic with exponential backoff
//   - HTTP request/response logging for debugging
//   - Structured error handling and API error translation
//   - Context-aware request handling
//   - Idempotency key support for message sending
//   - Configuration validation and connection testing
//
// The Client struct wraps the official AhaSend SDK while maintaining full
// compatibility and adding CLI-specific enhancements for better user experience.
// Requests through the SDK share the SDK's rate limiter; the trigger and
// stream-start requests the client sends itself, which the SDK does not
// cover, have no rate limit.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-cli/internal/version"
)

// Client wraps the AhaSend SDK client with additional functionality
type Client struct {
	*api.APIClient
	config      *api.Configuration
	auth        context.Context
	accountID   string
	accountUUID uuid.UUID
}

// MaxSendConcurrency is the most send requests messages send has in flight
// at once, and so the most connections to the API a client keeps open for
// the next request.
const MaxSendConcurrency = 10

// retryBaseDelay is the delay before the first retry of a failed request;
// each later retry waits about twice as long as the one before.
var retryBaseDelay = time.Second

// The limits of a request to the API. Uploading a request takes as long as
// its body needs, such as a 10 MB attachment on a slow uplink, so each wait
// for the API has its own limit, and the whole request a long one.
var (
	// connectTimeout is how long connecting to the API may take.
	connectTimeout = 30 * time.Second
	// tlsHandshakeTimeout is how long the TLS handshake may take.
	tlsHandshakeTimeout = 10 * time.Second
	// responseHeaderTimeout is how long the API may take to answer once the
	// request, body included, is written.
	responseHeaderTimeout = 30 * time.Second
	// requestTimeout is how long a request may take, from connecting to
	// reading the whole answer: long enough to upload a request with 10 MB
	// of attachments at under 200 kbit/s.
	requestTimeout = 10 * time.Minute
)

// dialAPI connects to the API within connectTimeout. Like the limits above,
// it is a variable, which a test replaces to slow the connection down.
var dialAPI = func(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
}

// newTransport returns the HTTP transport of an API client: the default
// transport with the limits above, which keeps a connection for each of the
// concurrent sends, and whose response bodies are cut off at
// maxResponseBytes.
func newTransport() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialAPI
	transport.TLSHandshakeTimeout = tlsHandshakeTimeout
	transport.ResponseHeaderTimeout = responseHeaderTimeout
	transport.MaxIdleConnsPerHost = MaxSendConcurrency
	return limitBodies{transport}
}

// maxResponseBytes is the size of the largest response body the CLI reads,
// 32 MiB: well above any real response, the largest of which is a template
// with its content, which the API holds to a few hundred KB. A test lowers
// it.
var maxResponseBytes int64 = 32 << 20

// responseTooLargeError reports a response body larger than the CLI reads.
// The command exits 1.
type responseTooLargeError struct {
	Limit int64
}

func (e *responseTooLargeError) Error() string {
	if e.Limit%(1<<20) == 0 {
		return fmt.Sprintf("the API response is larger than %d MiB, the most the CLI reads", e.Limit>>20)
	}
	return fmt.Sprintf("the API response is larger than %d bytes, the most the CLI reads", e.Limit)
}

// limitBodies is a transport whose response bodies fail with a
// responseTooLargeError once more than maxResponseBytes are read.
type limitBodies struct {
	transport http.RoundTripper
}

func (l limitBodies) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := l.transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	resp.Body = &limitedBody{body: resp.Body, left: maxResponseBytes}
	return resp, nil
}

// limitedBody reads body, failing with a responseTooLargeError, on this read
// and every later one, once more than left bytes would be read.
type limitedBody struct {
	body io.ReadCloser
	left int64
	err  error
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if int64(len(p)) > b.left+1 {
		p = p[:b.left+1]
	}
	n, err := b.body.Read(p)
	if int64(n) > b.left {
		b.err = &responseTooLargeError{Limit: maxResponseBytes}
		return int(b.left), b.err
	}
	b.left -= int64(n)
	return n, err
}

func (b *limitedBody) Close() error {
	return b.body.Close()
}

// ShortenRetryDelaysForTesting makes the clients created after it retry a
// failed request after a millisecond, so a test of an unreachable API does
// not wait through the backoff. It returns the function that restores the
// delay.
func ShortenRetryDelaysForTesting() (restore func()) {
	previous := retryBaseDelay
	retryBaseDelay = time.Millisecond
	return func() { retryBaseDelay = previous }
}

// NewClient creates a new AhaSend client
func NewClient(apiKey, accountID string, apiURL ...string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	accountUUID, err := uuid.Parse(accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID format: %w", err)
	}

	config := api.NewConfiguration()

	// Set API URL if provided
	if len(apiURL) > 0 && apiURL[0] != "" {
		if err := setConfigFromURL(config, apiURL[0]); err != nil {
			return nil, fmt.Errorf("invalid API URL: %w", err)
		}
	}

	// Configure retry behavior using new SDK RetryConfig
	config.RetryConfig = api.RetryConfig{
		Enabled:               true,
		MaxRetries:            3,
		RetryClientErrors:     false, // Never retry 4xx errors
		RetryValidationErrors: false, // Never retry validation errors
		BackoffStrategy:       api.BackoffExponential,
		BaseDelay:             retryBaseDelay,
		MaxDelay:              30 * time.Second,
	}

	// Set custom user agent
	config.UserAgent = fmt.Sprintf("ahasend-cli/%s %s", version.Version, config.UserAgent)

	// Add HTTP logging transport
	httpTransport := logger.NewHTTPTransport(newTransport(), logger.Get())
	config.HTTPClient = &http.Client{
		Transport: httpTransport,
		Timeout:   requestTimeout,
	}

	// Create authenticated context
	auth := context.WithValue(context.Background(), api.ContextAccessToken, apiKey)

	client := &Client{
		APIClient:   api.NewAPIClientWithConfig(config),
		config:      config,
		auth:        auth,
		accountID:   accountID,
		accountUUID: accountUUID,
	}

	return client, nil
}

// GetAccountID returns the configured account ID
func (c *Client) GetAccountID() string {
	return c.accountID
}

// GetAccount retrieves account information
func (c *Client) GetAccount() (*responses.Account, error) {
	// Call the account API
	return recordBody(c.AccountsAPI.GetAccount(c.auth, c.accountUUID))
}

// recordBody remembers the API response body behind result for JSON output.
// A successful response the SDK could not decode is an
// UnreadableResponseError, and so is an empty body, whatever its status, 204
// No Content included: every endpoint answers with a JSON body, none with
// 204, and the SDK leaves result empty for an empty one, which would read as
// a result with no data.
func recordBody[T any](result *T, resp *http.Response, err error) (*T, error) {
	return record(result, resp, err, false)
}

// recordDone is recordBody for a delete-style call, such as a delete, a
// cancel or a wipe, that the API answers with a SuccessResponse: its 2xx
// status alone says that the request succeeded, so an empty body is
// remembered too, and JSON output prints nothing for it.
func recordDone(result *common.SuccessResponse, resp *http.Response, err error) (*common.SuccessResponse, error) {
	return record(result, resp, err, true)
}

// record is recordBody, accepting an empty body on any 2xx status when
// emptyOK is set.
func record[T any](result *T, resp *http.Response, err error, emptyOK bool) (*T, error) {
	var apiErr *api.APIError
	if resp != nil && resp.StatusCode < 300 && stderrors.As(err, &apiErr) {
		return nil, unreadableResponse(apiErr.Raw, new(T))
	}
	// The SDK reports a body it could not read as a network error; one cut
	// off at maxResponseBytes is not.
	var tooLarge *responseTooLargeError
	if stderrors.As(err, &tooLarge) {
		return nil, tooLarge
	}
	if err != nil || result == nil || resp == nil {
		return result, err
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return result, nil
	}
	if len(body) == 0 && !emptyOK {
		return nil, emptyResponse()
	}
	apiresponse.Remember(result, body)
	return result, nil
}

// emptyResponse is the UnreadableResponseError of a successful response with
// no body where the CLI expects one.
func emptyResponse() error {
	return clierrors.NewUnreadableResponseError(nil, "the response is empty")
}

// unreadableResponse is the UnreadableResponseError of a successful response
// whose body does not decode into result. Its reason names the field whose
// value does not fit, never the decoder's Go types.
func unreadableResponse(body []byte, result any) error {
	reason := fmt.Sprintf("unexpected response body of %d bytes", len(body))
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	switch err := json.Unmarshal(body, result); {
	case stderrors.As(err, &typeErr) && typeErr.Field != "":
		reason = "unexpected value for field " + typeErr.Field
	case stderrors.As(err, &syntaxErr), stderrors.Is(err, io.ErrUnexpectedEOF):
		reason = "the response is not JSON"
	}
	return clierrors.NewUnreadableResponseError(body, reason)
}

// Ping tests the connection and validates the API key
func (c *Client) Ping() (*common.SuccessResponse, error) {
	return recordBody(c.UtilityAPI.Ping(c.auth))
}

// noRetries turns off the SDK's own retries for a request that must be sent
// once: a send, which messages send retries itself up to --max-retries
// times, and every DELETE request, such as a delete, a cancel or a wipe,
// whose repeat after a lost answer would fail with 404 although the first
// one succeeded.
var noRetries = api.WithRetry(api.RetryConfig{Enabled: false})

// SendMessageWithIdempotencyKey sends a message with idempotency key support,
// once: the caller decides about retries. A failure after the request was
// written is a RequestWrittenError.
func (c *Client) SendMessageWithIdempotencyKey(req requests.CreateMessageRequest, idempotencyKey string) (*responses.CreateMessageResponse, error) {
	ctx, written := traceWrite(c.auth)
	result, err := recordBody(c.MessagesAPI.CreateMessage(ctx, c.accountUUID, req, api.WithIdempotencyKey(idempotencyKey), noRetries))
	return result, markWritten(err, written)
}

// SendTemplateMessageWithIdempotencyKey sends a stored template with
// idempotency key support, once: the caller decides about retries. A failure
// after the request was written is a RequestWrittenError.
func (c *Client) SendTemplateMessageWithIdempotencyKey(req requests.CreateTemplateMessageRequest, idempotencyKey string) (*responses.CreateMessageResponse, error) {
	ctx, written := traceWrite(c.auth)
	result, err := recordBody(c.MessagesAPI.CreateTemplateMessage(ctx, c.accountUUID, req, api.WithIdempotencyKey(idempotencyKey), noRetries))
	return result, markWritten(err, written)
}

// RequestWrittenError is a failure to get the API's answer to a request that
// was written in full, such as a timeout or a connection closed while
// waiting for the answer: the API may have received the request and acted
// on it. It wraps the failure, so it prints and exits the same way.
type RequestWrittenError struct {
	Err error
}

func (e *RequestWrittenError) Error() string { return e.Err.Error() }

func (e *RequestWrittenError) Unwrap() error { return e.Err }

// traceWrite returns ctx with a trace that records, in written, that a
// request made with it was written in full.
func traceWrite(ctx context.Context) (context.Context, *atomic.Bool) {
	written := &atomic.Bool{}
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				written.Store(true)
			}
		},
	}), written
}

// markWritten returns err as a RequestWrittenError when the request was
// written and the API gave no answer; an API answer keeps its own error.
func markWritten(err error, written *atomic.Bool) error {
	var apiErr *api.APIError
	var unreadable *clierrors.UnreadableResponseError
	if err == nil || !written.Load() || stderrors.As(err, &apiErr) || stderrors.As(err, &unreadable) {
		return err
	}
	return &RequestWrittenError{Err: err}
}

// CancelMessage cancels a scheduled message, given its UUID or its
// Message-ID such as <uuid@example.com>
func (c *Client) CancelMessage(messageID string) (*common.SuccessResponse, error) {
	return recordDone(c.MessagesAPI.CancelMessage(c.auth, c.accountUUID, messageID, noRetries))
}

// ListDomains lists domains with filters and pagination
func (c *Client) ListDomains(params requests.GetDomainsParams) (*responses.PaginatedDomainsResponse, error) {
	return recordBody(c.DomainsAPI.GetDomainsWithParams(c.auth, c.accountUUID, params))
}

// CreateDomain creates a new domain
func (c *Client) CreateDomain(req requests.CreateDomainRequest, opts ...api.RequestOption) (*responses.Domain, error) {
	return recordBody(c.DomainsAPI.CreateDomain(c.auth, c.accountUUID, req, opts...))
}

// GetDomain gets a specific domain
func (c *Client) GetDomain(domain string) (*responses.Domain, error) {
	return recordBody(c.DomainsAPI.GetDomain(c.auth, c.accountUUID, domain))
}

// UpdateDomain updates domain settings
func (c *Client) UpdateDomain(domain string, req requests.UpdateDomainRequest) (*responses.Domain, error) {
	return recordBody(c.DomainsAPI.UpdateDomain(c.auth, c.accountUUID, domain, req))
}

// CheckDomainDNS triggers a DNS validation check for a domain
func (c *Client) CheckDomainDNS(domain string) (*responses.Domain, error) {
	return recordBody(c.DomainsAPI.CheckDomainDNS(c.auth, c.accountUUID, domain))
}

// DeleteDomain deletes a domain
func (c *Client) DeleteDomain(domain string) (*common.SuccessResponse, error) {
	return recordDone(c.DomainsAPI.DeleteDomain(c.auth, c.accountUUID, domain, noRetries))
}

// GetMessages retrieves messages with filtering and pagination
func (c *Client) GetMessages(params requests.GetMessagesParams) (*responses.PaginatedMessagesResponse, error) {
	return recordBody(c.MessagesAPI.GetMessages(c.auth, c.accountUUID, params))
}

// GetMessage retrieves a single message by its UUID or its Message-ID, such
// as <uuid@example.com>
func (c *Client) GetMessage(messageID string) (*responses.Message, error) {
	return recordBody(c.MessagesAPI.GetMessageByAPIID(c.auth, c.accountUUID, messageID))
}

// ListWebhooks lists a page of webhooks; a non-nil enabled keeps only the
// webhooks with that status, filtered by the API.
func (c *Client) ListWebhooks(pagination common.PaginationParams, enabled *bool) (*responses.PaginatedWebhooksResponse, error) {
	params := api.GetWebhooksParams{Enabled: enabled, PaginationParams: pagination}
	return recordBody(c.WebhooksAPI.GetWebhooks(c.auth, c.accountUUID, params))
}

// CreateWebhook creates a new webhook
func (c *Client) CreateWebhook(req requests.CreateWebhookRequest, opts ...api.RequestOption) (*responses.Webhook, error) {
	return recordBody(c.WebhooksAPI.CreateWebhook(c.auth, c.accountUUID, req, opts...))
}

// GetWebhook retrieves a single webhook by ID
func (c *Client) GetWebhook(webhookID uuid.UUID) (*responses.Webhook, error) {
	return recordBody(c.WebhooksAPI.GetWebhook(c.auth, c.accountUUID, webhookID))
}

// UpdateWebhook updates an existing webhook
func (c *Client) UpdateWebhook(webhookID uuid.UUID, req requests.UpdateWebhookRequest) (*responses.Webhook, error) {
	return recordBody(c.WebhooksAPI.UpdateWebhook(c.auth, c.accountUUID, webhookID, req))
}

// DeleteWebhook deletes a webhook
func (c *Client) DeleteWebhook(webhookID uuid.UUID) (*common.SuccessResponse, error) {
	return recordDone(c.WebhooksAPI.DeleteWebhook(c.auth, c.accountUUID, webhookID, noRetries))
}

// ListRoutes retrieves a page of routes, optionally of one recipient domain
func (c *Client) ListRoutes(params requests.GetRoutesParams) (*responses.PaginatedRoutesResponse, error) {
	return recordBody(c.RoutesAPI.GetRoutesWithParams(c.auth, c.accountUUID, params))
}

// CreateRoute creates a new route
func (c *Client) CreateRoute(req requests.CreateRouteRequest, opts ...api.RequestOption) (*responses.Route, error) {
	return recordBody(c.RoutesAPI.CreateRoute(c.auth, c.accountUUID, req, opts...))
}

// GetRoute retrieves a single route by ID
func (c *Client) GetRoute(routeID uuid.UUID) (*responses.Route, error) {
	return recordBody(c.RoutesAPI.GetRoute(c.auth, c.accountUUID, routeID))
}

// UpdateRoute updates an existing route
func (c *Client) UpdateRoute(routeID uuid.UUID, req requests.UpdateRouteRequest) (*responses.Route, error) {
	return recordBody(c.RoutesAPI.UpdateRoute(c.auth, c.accountUUID, routeID, req))
}

// DeleteRoute deletes a route
func (c *Client) DeleteRoute(routeID uuid.UUID) (*common.SuccessResponse, error) {
	return recordDone(c.RoutesAPI.DeleteRoute(c.auth, c.accountUUID, routeID, noRetries))
}

// ListSuppressions retrieves a paginated list of suppressions
func (c *Client) ListSuppressions(params requests.GetSuppressionsParams) (*responses.PaginatedSuppressionsResponse, error) {
	return recordBody(c.SuppressionsAPI.GetSuppressions(c.auth, c.accountUUID, params))
}

// CreateSuppression creates a new suppression
func (c *Client) CreateSuppression(req requests.CreateSuppressionRequest, opts ...api.RequestOption) (*responses.CreateSuppressionResponse, error) {
	return recordBody(c.SuppressionsAPI.CreateSuppression(c.auth, c.accountUUID, req, opts...))
}

// DeleteSuppression deletes a suppression by email and optional domain
func (c *Client) DeleteSuppression(email string, domain *string) (*common.SuccessResponse, error) {
	return recordDone(c.SuppressionsAPI.DeleteSuppression(c.auth, c.accountUUID, email, domain, noRetries))
}

// WipeSuppressions deletes all suppressions in the account
func (c *Client) WipeSuppressions(domain *string) (*common.SuccessResponse, error) {
	return recordDone(c.SuppressionsAPI.DeleteAllSuppressions(c.auth, c.accountUUID, domain, noRetries))
}

// ListSMTPCredentials lists a page of SMTP credentials
func (c *Client) ListSMTPCredentials(pagination common.PaginationParams) (*responses.PaginatedSMTPCredentialsResponse, error) {
	return recordBody(c.SMTPCredentialsAPI.GetSMTPCredentials(c.auth, c.accountUUID, &pagination))
}

// GetSMTPCredential gets a specific SMTP credential by ID
func (c *Client) GetSMTPCredential(credentialID uuid.UUID) (*responses.SMTPCredential, error) {
	return recordBody(c.SMTPCredentialsAPI.GetSMTPCredential(c.auth, c.accountUUID, credentialID))
}

// CreateSMTPCredential creates a new SMTP credential
func (c *Client) CreateSMTPCredential(req requests.CreateSMTPCredentialRequest, opts ...api.RequestOption) (*responses.SMTPCredential, error) {
	return recordBody(c.SMTPCredentialsAPI.CreateSMTPCredential(c.auth, c.accountUUID, req, opts...))
}

// DeleteSMTPCredential deletes an SMTP credential by ID
func (c *Client) DeleteSMTPCredential(credentialID uuid.UUID) (*common.SuccessResponse, error) {
	return recordDone(c.SMTPCredentialsAPI.DeleteSMTPCredential(c.auth, c.accountUUID, credentialID, noRetries))
}

// GetDeliverabilityStatistics retrieves deliverability statistics
func (c *Client) GetDeliverabilityStatistics(params requests.GetDeliverabilityStatisticsParams) (*responses.DeliverabilityStatisticsResponse, error) {
	return recordBody(c.StatisticsAPI.GetDeliverabilityStatistics(c.auth, c.accountUUID, params))
}

// GetBounceStatistics retrieves bounce statistics
func (c *Client) GetBounceStatistics(params requests.GetBounceStatisticsParams) (*responses.BounceStatisticsResponse, error) {
	return recordBody(c.StatisticsAPI.GetBounceStatistics(c.auth, c.accountUUID, params))
}

// GetDeliveryTimeStatistics retrieves delivery time statistics
func (c *Client) GetDeliveryTimeStatistics(params requests.GetDeliveryTimeStatisticsParams) (*responses.DeliveryTimeStatisticsResponse, error) {
	return recordBody(c.StatisticsAPI.GetDeliveryTimeStatistics(c.auth, c.accountUUID, params))
}

// Sub-account operations

// ListSubAccounts retrieves a page of sub-accounts
func (c *Client) ListSubAccounts(pagination common.PaginationParams) (*responses.PaginatedSubAccountsResponse, error) {
	return recordBody(c.SubAccountsAPI.ListSubAccounts(c.auth, c.accountUUID, &pagination))
}

// CreateSubAccount creates a new sub-account
func (c *Client) CreateSubAccount(req requests.CreateSubAccountRequest, opts ...api.RequestOption) (*responses.SubAccount, error) {
	return recordBody(c.SubAccountsAPI.CreateSubAccount(c.auth, c.accountUUID, req, opts...))
}

// GetSubAccountsUsage retrieves parent and sub-account usage allocation
func (c *Client) GetSubAccountsUsage() (*responses.SubAccountUsageResponse, error) {
	return recordBody(c.SubAccountsAPI.GetSubAccountsUsage(c.auth, c.accountUUID))
}

// GetSubAccount retrieves a specific sub-account by ID
func (c *Client) GetSubAccount(subAccountID uuid.UUID) (*responses.SubAccount, error) {
	return recordBody(c.SubAccountsAPI.GetSubAccount(c.auth, c.accountUUID, subAccountID))
}

// UpdateSubAccount updates an existing sub-account
func (c *Client) UpdateSubAccount(subAccountID uuid.UUID, req requests.UpdateSubAccountRequest) (*responses.SubAccount, error) {
	return recordBody(c.SubAccountsAPI.UpdateSubAccount(c.auth, c.accountUUID, subAccountID, req))
}

// DeleteSubAccount deletes a sub-account
func (c *Client) DeleteSubAccount(subAccountID uuid.UUID) (*common.SuccessResponse, error) {
	return recordDone(c.SubAccountsAPI.DeleteSubAccount(c.auth, c.accountUUID, subAccountID, noRetries))
}

// SuspendSubAccount suspends a sub-account
func (c *Client) SuspendSubAccount(subAccountID uuid.UUID, req requests.SuspendSubAccountRequest) (*responses.SubAccount, error) {
	return recordBody(c.SubAccountsAPI.SuspendSubAccount(c.auth, c.accountUUID, subAccountID, req))
}

// UnsuspendSubAccount unsuspends a sub-account
func (c *Client) UnsuspendSubAccount(subAccountID uuid.UUID) (*responses.SubAccount, error) {
	return recordBody(c.SubAccountsAPI.UnsuspendSubAccount(c.auth, c.accountUUID, subAccountID))
}

// UnpauseSubAccountDomain lifts the pause on a domain of a sub-account
func (c *Client) UnpauseSubAccountDomain(subAccountID uuid.UUID, domain string) (*responses.Domain, error) {
	return recordBody(c.SubAccountsAPI.UnpauseSubAccountDomain(c.auth, c.accountUUID, subAccountID, domain))
}

// Sub-account API key operations

// ListSubAccountAPIKeys retrieves a page of the API keys of a sub-account
func (c *Client) ListSubAccountAPIKeys(subAccountID uuid.UUID, pagination common.PaginationParams) (*responses.PaginatedAPIKeysResponse, error) {
	return recordBody(c.SubAccountsAPI.ListSubAccountAPIKeys(c.auth, c.accountUUID, subAccountID, &pagination))
}

// CreateSubAccountAPIKey creates a new API key for a sub-account
func (c *Client) CreateSubAccountAPIKey(subAccountID uuid.UUID, req requests.CreateAPIKeyRequest, opts ...api.RequestOption) (*responses.APIKey, error) {
	return recordBody(c.SubAccountsAPI.CreateSubAccountAPIKey(c.auth, c.accountUUID, subAccountID, req, opts...))
}

// GetSubAccountAPIKey retrieves a specific API key owned by a sub-account
func (c *Client) GetSubAccountAPIKey(subAccountID, keyID uuid.UUID) (*responses.APIKey, error) {
	return recordBody(c.SubAccountsAPI.GetSubAccountAPIKey(c.auth, c.accountUUID, subAccountID, keyID))
}

// UpdateSubAccountAPIKey updates an API key owned by a sub-account
func (c *Client) UpdateSubAccountAPIKey(subAccountID, keyID uuid.UUID, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error) {
	return recordBody(c.SubAccountsAPI.UpdateSubAccountAPIKey(c.auth, c.accountUUID, subAccountID, keyID, req))
}

// DeleteSubAccountAPIKey deletes an API key owned by a sub-account
func (c *Client) DeleteSubAccountAPIKey(subAccountID, keyID uuid.UUID) (*common.SuccessResponse, error) {
	return recordDone(c.SubAccountsAPI.DeleteSubAccountAPIKey(c.auth, c.accountUUID, subAccountID, keyID, noRetries))
}

// API Key operations

// ListAPIKeys retrieves a page of API keys
func (c *Client) ListAPIKeys(pagination common.PaginationParams) (*responses.PaginatedAPIKeysResponse, error) {
	return recordBody(c.APIKeysAPI.GetAPIKeys(c.auth, c.accountUUID, &pagination))
}

// GetAPIKey retrieves a specific API key by ID
func (c *Client) GetAPIKey(keyID uuid.UUID) (*responses.APIKey, error) {
	return recordBody(c.APIKeysAPI.GetAPIKey(c.auth, c.accountUUID, keyID))
}

// CreateAPIKey creates a new API key
func (c *Client) CreateAPIKey(req requests.CreateAPIKeyRequest, opts ...api.RequestOption) (*responses.APIKey, error) {
	return recordBody(c.APIKeysAPI.CreateAPIKey(c.auth, c.accountUUID, req, opts...))
}

// UpdateAPIKey updates an existing API key
func (c *Client) UpdateAPIKey(keyID uuid.UUID, req requests.UpdateAPIKeyRequest) (*responses.APIKey, error) {
	return recordBody(c.APIKeysAPI.UpdateAPIKey(c.auth, c.accountUUID, keyID, req))
}

// DeleteAPIKey deletes an API key
func (c *Client) DeleteAPIKey(keyID uuid.UUID) (*common.SuccessResponse, error) {
	return recordDone(c.APIKeysAPI.DeleteAPIKey(c.auth, c.accountUUID, keyID, noRetries))
}

// TriggerWebhook triggers webhook events for development testing
func (c *Client) TriggerWebhook(webhookID uuid.UUID, events []string) (*common.SuccessResponse, error) {
	endpoint := fmt.Sprintf("/v2/accounts/%s/webhooks/%s/trigger", c.accountUUID, webhookID)
	logger.Get().WithFields(map[string]interface{}{
		"method":     "POST",
		"endpoint":   endpoint,
		"webhook_id": webhookID,
		"events":     events,
	}).Debug("Sending webhook trigger request")

	result, err := c.postTrigger(endpoint, map[string]interface{}{"events": events})
	if err == nil {
		logger.Get().WithFields(map[string]interface{}{
			"webhook_id": webhookID,
			"events":     events,
		}).Debug("Webhook trigger successful")
	}
	return result, err
}

// TriggerRoute triggers route events for development testing
func (c *Client) TriggerRoute(routeID uuid.UUID) (*common.SuccessResponse, error) {
	endpoint := fmt.Sprintf("/v2/accounts/%s/routes/%s/trigger", c.accountUUID, routeID)
	logger.Get().WithFields(map[string]interface{}{
		"method":   "POST",
		"endpoint": endpoint,
		"route_id": routeID,
	}).Debug("Sending route trigger request")

	// Routes take no events, so the payload is an empty object
	result, err := c.postTrigger(endpoint, map[string]interface{}{})
	if err == nil {
		logger.Get().WithField("route_id", routeID).Debug("Route trigger successful")
	}
	return result, err
}

// postTrigger sends payload to a trigger endpoint, which the SDK does not
// cover.
func (c *Client) postTrigger(endpoint string, payload map[string]interface{}) (*common.SuccessResponse, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}
	return post[common.SuccessResponse](c, endpoint, nil, payloadBytes)
}

// post sends a POST request to an endpoint the SDK does not cover, with the
// client's authentication and user agent, and a JSON body when body is not
// nil. It bypasses the SDK, so its rate limiter too.
func post[T any](c *Client, endpoint string, query url.Values, body []byte) (*T, error) {
	fullURL := fmt.Sprintf("%s://%s%s", c.config.Scheme, c.config.Host, endpoint)
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest("POST", fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	apiKey := c.auth.Value(api.ContextAccessToken).(string)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("User-Agent", c.config.UserAgent)

	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return readResult[T](resp)
}

// readResult reads the response of a call the SDK does not cover: the API's
// response on a 2xx status, remembered for JSON output, and an *api.APIError
// that carries the body otherwise. The error's message is the one the SDK
// gives its own calls: the API's message, or the status text for a body that
// is not JSON, such as a proxy's HTML page.
func readResult[T any](resp *http.Response) (*T, error) {
	// A connection lost while reading the body is a network error, as the
	// SDK makes it for its own calls.
	body, err := io.ReadAll(resp.Body)
	var tooLarge *responseTooLargeError
	if stderrors.As(err, &tooLarge) {
		return nil, tooLarge
	}
	if err != nil {
		return nil, &api.NetworkError{Op: "reading response", Err: err}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := api.ParseAPIError(resp, body)
		var errorResp common.ErrorResponse
		if json.Unmarshal(body, &errorResp) == nil {
			apiErr.Message = errorResp.Message
		} else {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}
		return nil, apiErr
	}

	result := new(T)
	if len(body) == 0 {
		return nil, emptyResponse()
	}
	if err := json.Unmarshal(body, result); err != nil {
		return nil, unreadableResponse(body, new(T))
	}
	apiresponse.Remember(result, body)
	return result, nil
}

// setConfigFromURL sets the SDK configuration's scheme and host from
// apiURL, which must pass validation.CheckAPIURL: the SDK sends every request
// to the root of the host, so it has no place for a path.
func setConfigFromURL(config *api.Configuration, apiURL string) error {
	if err := validation.CheckAPIURL(apiURL); err != nil {
		return err
	}
	u, err := url.Parse(apiURL)
	if err != nil {
		return err
	}
	config.Scheme = u.Scheme
	config.Host = u.Host
	return nil
}
