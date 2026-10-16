// Package printer writes the result of each command in the format --output
// names. A ResponseHandler has one method per kind of result, taking the
// SDK response type, and one implementation per format:
//
//   - json: the API's response body, unchanged (see package apiresponse),
//     or the JSON of a result the CLI built itself
//   - table: bordered tables, for reading in a terminal
//   - plain: one "Label: value" line per field, leaving out missing values
//   - csv: a header row and one row per item, with RFC 3339 times
//
// Table and plain output escape control characters, since results carry
// text from email senders and other users. The root command creates the
// handler and stores it in the command's context, where a command gets it:
//
//	response, err := client.ListDomains(requests.GetDomainsParams{})
//	if err != nil {
//		return err
//	}
//	return printer.GetResponseHandlerFromCommand(cmd).HandleDomainList(response, printer.ListConfig{
//		SuccessMessage: "Domains retrieved successfully",
//		EmptyMessage:   "No domains found",
//	})
package printer

import (
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
)

// ResponseHandler defines the interface for type-safe output formatting with concrete types.
// Every method uses specific SDK response types instead of interface{} for complete type safety.
type ResponseHandler interface {
	// Error handling
	HandleError(err error) error

	// Domain responses
	HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error
	HandleSingleDomain(domain *responses.Domain, config SingleConfig) error

	// Message responses
	HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error
	HandleSingleMessage(message *responses.Message, config SingleConfig) error
	HandleCreateMessage(response *responses.CreateMessageResponse, config ResultConfig) error

	// Webhook responses
	HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error
	HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error
	HandleCreateWebhook(webhook *responses.Webhook, config ResultConfig) error
	HandleUpdateWebhook(webhook *responses.Webhook, config ResultConfig) error

	// Route responses
	HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error
	HandleSingleRoute(route *responses.Route, config SingleConfig) error
	HandleCreateRoute(route *responses.Route, config ResultConfig) error
	HandleUpdateRoute(route *responses.Route, config ResultConfig) error

	// Suppression responses
	HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error
	HandleCreateSuppression(response *responses.CreateSuppressionResponse, config ResultConfig) error
	HandleCheckSuppression(response *responses.PaginatedSuppressionsResponse, config CheckConfig) error

	// SMTP responses
	HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error
	HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error
	HandleCreateSMTP(credential *responses.SMTPCredential, config ResultConfig) error
	HandleSMTPSend(result *SMTPSendResult, config ResultConfig) error

	// API Key responses
	HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error
	HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error
	HandleCreateAPIKey(key *responses.APIKey, config ResultConfig) error
	HandleUpdateAPIKey(key *responses.APIKey, config ResultConfig) error

	// Sub-account responses
	HandleSubAccountList(response *responses.PaginatedSubAccountsResponse, config ListConfig) error
	HandleSingleSubAccount(subAccount *responses.SubAccount, config SingleConfig) error
	HandleSubAccountUsage(response *responses.SubAccountUsageResponse, config SingleConfig) error

	// Contact responses
	HandleContactList(response *responses.PaginatedContactsResponse, config ListConfig) error
	HandleSingleContact(contact *responses.Contact, config SingleConfig) error
	HandleBatchContacts(response *responses.BatchUpsertContactsResponse, config SingleConfig) error

	// List responses
	HandleListsList(response *responses.PaginatedContactListsResponse, config ListConfig) error
	HandleSingleList(list *responses.ContactList, config SingleConfig) error

	// List member responses
	HandleListMemberList(response *responses.PaginatedListContactsResponse, config ListConfig) error
	HandleContactMembershipList(response *responses.PaginatedListContactsResponse, config ListConfig) error
	HandleSingleListMember(member *responses.ListContact, config SingleConfig) error
	HandleBatchListMembers(response *responses.BatchAddListContactsResponse, config SingleConfig) error

	// Template responses
	HandleTemplateList(response *responses.PaginatedTemplatesResponse, config ListConfig) error
	HandleSingleTemplate(template *responses.Template, config SingleConfig) error
	HandleTemplateDraft(draft *responses.TemplateDraft, config SingleConfig) error
	HandleTemplateVersionList(response *responses.TemplateVersionsResponse, config ListConfig) error
	HandleSingleTemplateVersion(version *responses.TemplateVersionDetail, config SingleConfig) error

	// Campaign responses
	HandleCampaignList(response *responses.PaginatedCampaignsResponse, config ListConfig) error
	HandleSingleCampaign(campaign *responses.Campaign, config SingleConfig) error
	HandleCampaignSend(send *responses.CampaignSend, config SingleConfig) error
	HandleCampaignTest(test *responses.CampaignTest, config SingleConfig) error
	HandleCampaignStats(stats *responses.CampaignStats, config SingleConfig) error
	HandleCampaignRecipientList(response *responses.PaginatedCampaignRecipientsResponse, config ListConfig) error

	// Campaign template responses
	HandleCampaignTemplateList(response *responses.PaginatedCampaignTemplatesResponse, config ListConfig) error
	HandleSingleCampaignTemplate(template *responses.CampaignTemplate, config SingleConfig) error

	// Statistics responses
	HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error
	HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error
	HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error

	// Auth responses
	HandleAuthResult(result AuthResult) error
	HandleAuthStatus(status *AuthStatus) error
	HandleAuthProfiles(profiles []string) error

	// API success responses, such as deletes
	HandleSuccessResponse(response *common.SuccessResponse, config ResultConfig) error

	// Commands that make the same API call for several items
	HandleAPIResults(results []APIResult, config ResultConfig) error

	// Empty state
	HandleEmpty(message string) error

	// GetFormat returns the output format name
	GetFormat() string
}

// Configuration types for different response scenarios

// ListConfig configures how paginated list responses are displayed. The
// printer owns the fields of each view.
type ListConfig struct {
	SuccessMessage string // Message to show on successful retrieval
	EmptyMessage   string // Message to show when list is empty
	// Detailed selects the detailed list view, for the items that have one
	// (messages list --show-details).
	Detailed bool
	// ItemName names the items in the "Showing" line of a view that lists
	// different items for different commands (memberships: "lists" or
	// "members").
	ItemName string
}

// SingleConfig configures how single item responses are displayed
type SingleConfig struct {
	SuccessMessage string // Message to show on successful retrieval
	EmptyMessage   string // Message to show when item is nil
}

// ResultConfig configures how the result of a create, update, delete or
// send is displayed.
type ResultConfig struct {
	SuccessMessage string // Message to show on success
}

// StatsConfig configures how statistics responses are displayed
type StatsConfig struct {
	Title string // Title for the statistics display
}

// AuthResult is the outcome of auth login, switch or logout.
type AuthResult struct {
	Profile string // The profile the command acted on; empty for logout --all
	Message string // What the command did, such as "Switched to profile 'production'"
}

// CheckConfig configures how check/verification responses are displayed
type CheckConfig struct {
	FoundMessage    string // Message to show when item is found
	NotFoundMessage string // Message to show when item is not found
}

// Custom types for complex scenarios that don't map directly to SDK types

// AuthStatus represents the current authentication status. It never holds
// the API key: like the logs, it identifies the credentials by account ID.
type AuthStatus struct {
	Profile   string             // Currently active profile name
	AccountID string             // The profile's account ID
	Account   *responses.Account // Account information, nil when it could not be fetched
	// Valid is whether the API accepted the credentials, or nil when the
	// check could not tell, such as when the API could not be reached.
	Valid *bool
	// Error is why the check of the credentials failed, if it did.
	Error string
	// Note says why the account is missing from a check that passed, such
	// as a key without the scope to read the account.
	Note string
}

// SMTPSendResult represents the result of an SMTP send operation: a sent
// message, or a test that passed or failed. A send that fails returns its
// error without printing a result.
type SMTPSendResult struct {
	Success bool // Whether the send or the test was successful
	// MessageID is the Message-ID header of a sent message
	MessageID string
	Error     string // The failure of a failed test
	TestMode  bool   // Whether this was a test send
	// Connected is whether a test connected to the server and read its
	// greeting. A test that connected and failed failed the login, and
	// Error says why.
	Connected bool
	// Authenticated is whether a test logged in, which it does only when
	// credentials are given
	Authenticated bool
}

// handlerBase provides common functionality for all response handlers
type handlerBase struct {
	writer    io.Writer
	errWriter io.Writer // where table, plain and CSV output print errors
}

// HandleError prints err to errWriter, as table, plain and CSV output show
// errors, escaped as EscapeDisplayText escapes, followed by what to do about
// it, when ErrorHint knows, and returns it.
func (h *handlerBase) HandleError(err error) error {
	fmt.Fprintf(h.errWriter, "Error: %v%s\n", err, ErrorHint(err))
	return err
}

// ErrorHint returns what to do about err, to follow its text, or "": how long
// the API asked to wait, when its error answer had a Retry-After header, as a
// 429 or a 503 may, and what to check when the API could not be reached. An
// error whose text already names the wait, a WaitNamer, gets no hint.
func ErrorHint(err error) string {
	var apiErr *api.APIError
	if stderrors.As(err, &apiErr) {
		var named WaitNamer
		if apiErr.RetryAfter > 0 && !stderrors.As(err, &named) {
			return fmt.Sprintf(" (the API asked to wait %d s)", apiErr.RetryAfter)
		}
		return ""
	}
	if clierrors.IsAPIUnreachable(err) {
		return " (check the network connection and the API URL: auth login --api-url or the profile's api_url)"
	}
	return ""
}

// WaitNamer is an error whose text names the wait the API asked for, such as
// that of a send the API asked to retry too late, so HandleError adds no
// hint that names it again.
type WaitNamer interface {
	error
	NamesTheWait()
}

// NewResponseHandler creates the response handler for format, in any case, or
// returns a validation error when the format is not supported. Results go to
// writer. Errors go to errWriter in table, plain and CSV formats; JSON output
// prints errors to writer, since they are the API's response or the CLI's
// error object.
func NewResponseHandler(format string, writer, errWriter io.Writer) (ResponseHandler, error) {
	if writer == nil {
		writer = os.Stdout
	}
	if errWriter == nil {
		errWriter = os.Stderr
	}

	// Errors are for display in every format: their text can hold the API's
	// or a server's message
	errWriter = displayWriter{w: errWriter}
	base := handlerBase{writer: writer, errWriter: errWriter}
	// Table and plain output are for display: they escape control characters
	display := handlerBase{writer: displayWriter{w: writer}, errWriter: errWriter}

	switch strings.ToLower(format) {
	case "json":
		return &jsonHandler{handlerBase: base}, nil
	case "table":
		return &tableHandler{handlerBase: display}, nil
	case "plain":
		return &plainHandler{handlerBase: display}, nil
	case "csv":
		return &csvHandler{handlerBase: base}, nil
	default:
		return nil, clierrors.NewValidationError(fmt.Sprintf("--output must be one of: %s",
			strings.Join(GetSupportedFormats(), ", ")), nil)
	}
}

// GetSupportedFormats returns the list of supported output formats
func GetSupportedFormats() []string {
	return []string{"json", "table", "plain", "csv"}
}
