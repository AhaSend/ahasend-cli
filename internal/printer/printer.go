// Package printer provides a type-safe output formatting system for the AhaSend CLI.
//
// This package implements a ResponseHandler interface that provides complete type safety
// by using concrete SDK response types instead of interface{} for all operations.
// The handler is instantiated based on the --output flag and passed to commands.
//
// Supported formats:
//   - JSON: Machine-readable format for automation and APIs
//   - Table: Human-readable tabular format with proper alignment
//   - Plain: Simple key-value format for scripts and debugging
//   - CSV: Comma-separated values for data processing
//
// Usage:
//
//	handler := printer.GetResponseHandler(cmd)
//	response, err := client.ListDomains(...)
//	if err != nil {
//		return handler.HandleError(err)
//	}
//	return handler.HandleDomainList(response, printer.ListConfig{
//		SuccessMessage: "Successfully retrieved domains",
//		EmptyMessage:   "No domains found",
//		ShowPagination: true,
//	})
package printer

import (
	"fmt"
	"io"
	"os"

	"github.com/AhaSend/ahasend-go/models/responses"
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
	HandleCreateMessage(response *responses.CreateMessageResponse, config CreateConfig) error
	HandleCancelMessage(response *CancelMessageResponse, config SimpleConfig) error

	// Webhook responses
	HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error
	HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error
	HandleCreateWebhook(webhook *responses.Webhook, config CreateConfig) error
	HandleUpdateWebhook(webhook *responses.Webhook, config UpdateConfig) error
	HandleDeleteWebhook(success bool, config DeleteConfig) error
	HandleTriggerWebhook(webhookID string, events []string, config TriggerConfig) error

	// Route responses
	HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error
	HandleSingleRoute(route *responses.Route, config SingleConfig) error
	HandleCreateRoute(route *responses.Route, config CreateConfig) error
	HandleUpdateRoute(route *responses.Route, config UpdateConfig) error
	HandleDeleteRoute(success bool, config DeleteConfig) error

	// Suppression responses
	HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error
	HandleSingleSuppression(suppression *responses.Suppression, config SingleConfig) error
	HandleCreateSuppression(response *responses.CreateSuppressionResponse, config CreateConfig) error
	HandleDeleteSuppression(success bool, config DeleteConfig) error
	HandleWipeSuppression(count int, config WipeConfig) error
	HandleCheckSuppression(suppression *responses.Suppression, found bool, config CheckConfig) error

	// SMTP responses
	HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error
	HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error
	HandleCreateSMTP(credential *responses.SMTPCredential, config CreateConfig) error
	HandleDeleteSMTP(success bool, config DeleteConfig) error
	HandleSMTPSend(result *SMTPSendResult, config SMTPSendConfig) error

	// API Key responses
	HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error
	HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error
	HandleCreateAPIKey(key *responses.APIKey, config CreateConfig) error
	HandleUpdateAPIKey(key *responses.APIKey, config UpdateConfig) error
	HandleDeleteAPIKey(success bool, config DeleteConfig) error

	// Statistics responses
	HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error
	HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error
	HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error

	// Auth responses
	HandleAuthLogin(success bool, profile string, config AuthConfig) error
	HandleAuthLogout(success bool, config AuthConfig) error
	HandleAuthStatus(status *AuthStatus, config AuthConfig) error
	HandleAuthSwitch(newProfile string, config AuthConfig) error

	// Simple success without data
	HandleSimpleSuccess(message string) error

	// Empty state
	HandleEmpty(message string) error

	// Internal methods for format detection and output writing
	GetFormat() string
	SetWriter(w io.Writer)
}

// Configuration types for different response scenarios

// ListConfig configures how paginated list responses are displayed
type ListConfig struct {
	SuccessMessage string   // Message to show on successful retrieval
	EmptyMessage   string   // Message to show when list is empty
	ShowPagination bool     // Whether to show pagination information
	FieldOrder     []string // Optional field ordering for table display
}

// SingleConfig configures how single item responses are displayed
type SingleConfig struct {
	SuccessMessage string   // Message to show on successful retrieval
	EmptyMessage   string   // Message to show when item is nil
	FieldOrder     []string // Optional field ordering for table display
}

// CreateConfig configures how creation responses are displayed
type CreateConfig struct {
	SuccessMessage string   // Message to show on successful creation
	ItemName       string   // Name of the item being created (e.g., "domain", "webhook")
	FieldOrder     []string // Optional field ordering for table display
}

// UpdateConfig configures how update responses are displayed
type UpdateConfig struct {
	SuccessMessage string   // Message to show on successful update
	ItemName       string   // Name of the item being updated (e.g., "domain", "webhook")
	FieldOrder     []string // Optional field ordering for table display
}

// DeleteConfig configures how deletion responses are displayed
type DeleteConfig struct {
	SuccessMessage string // Message to show on successful deletion
	ItemName       string // Name of the item being deleted (e.g., "domain", "webhook")
}

// StatsConfig configures how statistics responses are displayed
type StatsConfig struct {
	Title      string   // Title for the statistics display
	ShowChart  bool     // Whether to show ASCII charts for data
	FieldOrder []string // Optional field ordering for table display
}

// AuthConfig configures how authentication responses are displayed
type AuthConfig struct {
	SuccessMessage string // Message to show on successful auth operation
}

// CheckConfig configures how check/verification responses are displayed
type CheckConfig struct {
	FoundMessage    string   // Message to show when item is found
	NotFoundMessage string   // Message to show when item is not found
	FieldOrder      []string // Optional field ordering for table display
}

// WipeConfig configures how wipe/bulk deletion responses are displayed
type WipeConfig struct {
	SuccessMessage string // Message to show on successful wipe
	ItemName       string // Name of the items being wiped (e.g., "suppressions")
}

// SMTPSendConfig configures how SMTP send responses are displayed
type SMTPSendConfig struct {
	SuccessMessage string // Message to show on successful send
	TestMode       bool   // Whether this was a test send
}

// SimpleConfig configures simple success responses
type SimpleConfig struct {
	SuccessMessage string // Message to show on success
}

// TriggerConfig configures how webhook trigger responses are displayed
type TriggerConfig struct {
	SuccessMessage string // Message to show on successful trigger
}

// Custom types for complex scenarios that don't map directly to SDK types

// AuthStatus represents the current authentication status
type AuthStatus struct {
	Profile string             // Currently active profile name
	APIKey  string             // Masked API key (showing only account ID)
	Account *responses.Account // Full account information
	Valid   bool               // Whether the authentication is valid
}

// SMTPSendResult represents the result of an SMTP send operation
type SMTPSendResult struct {
	Success   bool   // Whether the send was successful
	MessageID string // Message ID if successful
	Error     string // Error message if failed
	TestMode  bool   // Whether this was a test send
}

// CancelMessageResponse represents a message cancellation result
type CancelMessageResponse struct {
	MessageID string // ID of the cancelled message
	Success   bool   // Whether cancellation was successful
	Error     string // Error message if failed
}

// handlerBase provides common functionality for all response handlers
type handlerBase struct {
	writer      io.Writer
	colorOutput bool
}

// SetWriter sets the output writer
func (h *handlerBase) SetWriter(w io.Writer) {
	h.writer = w
}

// GetResponseHandler creates a new response handler based on the format string
func GetResponseHandler(format string, colorOutput bool, writer io.Writer) ResponseHandler {
	if writer == nil {
		writer = os.Stdout
	}

	base := handlerBase{
		writer:      writer,
		colorOutput: colorOutput,
	}

	switch format {
	case "json":
		return &jsonHandler{handlerBase: base}
	case "table":
		return &tableHandler{handlerBase: base}
	case "plain":
		return &plainHandler{handlerBase: base}
	case "csv":
		return &csvHandler{handlerBase: base}
	default:
		// Return a handler that shows an error for unsupported formats
		return &unsupportedHandler{format: format, handlerBase: base}
	}
}

// GetSupportedFormats returns the list of supported output formats
func GetSupportedFormats() []string {
	return []string{"json", "table", "plain", "csv"}
}

// ValidateFormat validates that a format is supported
func ValidateFormat(format string) error {
	supported := GetSupportedFormats()
	for _, f := range supported {
		if f == format {
			return nil
		}
	}
	return fmt.Errorf("unsupported output format: %s (supported: %v)", format, supported)
}

// unsupportedHandler handles unsupported formats gracefully
type unsupportedHandler struct {
	format string
	handlerBase
}

// GetFormat returns the format name
func (h *unsupportedHandler) GetFormat() string {
	return h.format
}

// All methods return unsupported format errors
func (h *unsupportedHandler) HandleError(err error) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSingleDomain(domain *responses.Domain, config SingleConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSingleMessage(message *responses.Message, config SingleConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleCreateMessage(response *responses.CreateMessageResponse, config CreateConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleCancelMessage(response *CancelMessageResponse, config SimpleConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleCreateWebhook(webhook *responses.Webhook, config CreateConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleUpdateWebhook(webhook *responses.Webhook, config UpdateConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleDeleteWebhook(success bool, config DeleteConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleTriggerWebhook(webhookID string, events []string, config TriggerConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSingleRoute(route *responses.Route, config SingleConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleCreateRoute(route *responses.Route, config CreateConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleUpdateRoute(route *responses.Route, config UpdateConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleDeleteRoute(success bool, config DeleteConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSingleSuppression(suppression *responses.Suppression, config SingleConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleCreateSuppression(response *responses.CreateSuppressionResponse, config CreateConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleDeleteSuppression(success bool, config DeleteConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleWipeSuppression(count int, config WipeConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleCheckSuppression(suppression *responses.Suppression, found bool, config CheckConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleCreateSMTP(credential *responses.SMTPCredential, config CreateConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleDeleteSMTP(success bool, config DeleteConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSMTPSend(result *SMTPSendResult, config SMTPSendConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleCreateAPIKey(key *responses.APIKey, config CreateConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleUpdateAPIKey(key *responses.APIKey, config UpdateConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleDeleteAPIKey(success bool, config DeleteConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleAuthLogin(success bool, profile string, config AuthConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleAuthLogout(success bool, config AuthConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleAuthStatus(status *AuthStatus, config AuthConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleAuthSwitch(newProfile string, config AuthConfig) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleSimpleSuccess(message string) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}

func (h *unsupportedHandler) HandleEmpty(message string) error {
	return fmt.Errorf("unsupported output format: %s", h.format)
}
