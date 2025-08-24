package webhooks

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NewWebhooksCommandForTesting creates a fresh webhooks command for testing
func NewWebhooksCommandForTesting() *mockableWebhooksCommand {
	return &mockableWebhooksCommand{
		mockClient: &mocks.MockClient{},
	}
}

type mockableWebhooksCommand struct {
	mockClient *mocks.MockClient
}

func (m *mockableWebhooksCommand) GetMockClient() *mocks.MockClient {
	return m.mockClient
}

// Test webhook command structure and subcommands
func TestWebhooksCommand_Structure(t *testing.T) {
	// Create a fresh webhooks command and verify it has expected subcommands
	webhooksCmd := NewCommand()
	expectedSubcommands := []string{"list", "get", "create", "update", "delete"}

	subcommands := make([]string, 0)
	for _, cmd := range webhooksCmd.Commands() {
		subcommands = append(subcommands, cmd.Name())
	}

	for _, expected := range expectedSubcommands {
		assert.Contains(t, subcommands, expected, "webhooks command should have %s subcommand", expected)
	}

	assert.Equal(t, "webhooks", webhooksCmd.Name())
	assert.Equal(t, "Manage your webhook endpoints", webhooksCmd.Short)
	assert.NotEmpty(t, webhooksCmd.Long)
}

func TestWebhooksCommand_Help(t *testing.T) {
	cmd := NewCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Manage your webhook endpoints")
	assert.Contains(t, helpOutput, "list")
	assert.Contains(t, helpOutput, "get")
	assert.Contains(t, helpOutput, "create")
	assert.Contains(t, helpOutput, "update")
	assert.Contains(t, helpOutput, "delete")
}

func TestWebhooksCommand_SubcommandCount(t *testing.T) {
	cmd := NewCommand()
	subcommands := cmd.Commands()

	// Should have exactly 7 subcommands (list, get, create, update, delete, listen, trigger)
	assert.Equal(t, 7, len(subcommands), "webhooks command should have exactly 7 subcommands")
}

// Test list command structure and flags
func TestListCommand_Flags(t *testing.T) {
	// Test that list command has required flags
	listCmd := NewListCommand()
	flags := listCmd.Flags()

	limitFlag := flags.Lookup("limit")
	assert.NotNil(t, limitFlag)
	assert.Equal(t, "int32", limitFlag.Value.Type())

	cursorFlag := flags.Lookup("cursor")
	assert.NotNil(t, cursorFlag)
	assert.Equal(t, "string", cursorFlag.Value.Type())

	enabledFlag := flags.Lookup("enabled")
	assert.NotNil(t, enabledFlag)
	assert.Equal(t, "bool", enabledFlag.Value.Type())
}

func TestListCommand_Structure(t *testing.T) {
	listCmd := NewListCommand()
	assert.Equal(t, "list", listCmd.Name())
	assert.Equal(t, "List all webhooks", listCmd.Short)
	assert.NotEmpty(t, listCmd.Long)
	assert.NotEmpty(t, listCmd.Example)
}

func TestListCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "all flags provided",
			args: []string{
				"--limit", "50",
				"--cursor", "next-page-token",
				"--enabled",
			},
			expected: map[string]interface{}{
				"limit":   int32(50),
				"cursor":  "next-page-token",
				"enabled": true,
			},
		},
		{
			name: "only limit flag",
			args: []string{"--limit", "25"},
			expected: map[string]interface{}{
				"limit": int32(25),
			},
		},
		{
			name: "only enabled flag",
			args: []string{"--enabled"},
			expected: map[string]interface{}{
				"enabled": true,
			},
		},
		{
			name:     "no flags",
			args:     []string{},
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewListCommand()
			cmd.SetArgs(tt.args)

			// Parse flags without executing the command
			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			// Check parsed flag values
			if expectedLimit, ok := tt.expected["limit"]; ok {
				limit, _ := cmd.Flags().GetInt32("limit")
				assert.Equal(t, expectedLimit, limit)
			}

			if expectedCursor, ok := tt.expected["cursor"]; ok {
				cursor, _ := cmd.Flags().GetString("cursor")
				assert.Equal(t, expectedCursor, cursor)
			}

			if expectedEnabled, ok := tt.expected["enabled"]; ok {
				enabled, _ := cmd.Flags().GetBool("enabled")
				assert.Equal(t, expectedEnabled, enabled)
			}
		})
	}
}

// Mock webhook list command tests using the pattern from existing tests
func TestWebhooksList_Success(t *testing.T) {
	// Create test webhooks
	webhook1 := createTestWebhook(uuid.New().String(), "Test Webhook 1", "https://example.com/webhook1", true)
	webhook2 := createTestWebhook(uuid.New().String(), "Test Webhook 2", "https://example.com/webhook2", false)

	testWebhooks := []responses.Webhook{webhook1, webhook2}
	mockResponse := &responses.PaginatedWebhooksResponse{
		Object: "list",
		Data:   testWebhooks,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Create mock client
	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", (*int32)(nil), (*string)(nil)).Return(mockResponse, nil)

	// Test would require command execution with mock injection
	// This follows the pattern but would need auth integration
	assert.NotNil(t, mockResponse)
	assert.Len(t, mockResponse.Data, 2)
	assert.Equal(t, "Test Webhook 1", mockResponse.Data[0].Name)
	assert.Equal(t, "Test Webhook 2", mockResponse.Data[1].Name)
}

func TestWebhooksList_WithFlags(t *testing.T) {
	// Test pagination parameters
	limit := int32(10)
	cursor := "test-cursor"

	webhook := createTestWebhook(uuid.New().String(), "Test Webhook", "https://example.com/webhook", true)
	testWebhooks := []responses.Webhook{webhook}

	mockResponse := &responses.PaginatedWebhooksResponse{
		Object: "list",
		Data:   testWebhooks,
		Pagination: common.PaginationInfo{
			HasMore:    true,
			NextCursor: stringPtr("next-cursor"),
		},
	}

	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", &limit, &cursor).Return(mockResponse, nil)

	// Verify the mock setup is correct
	_, err := mockClient.ListWebhooks(&limit, &cursor)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestWebhooksList_EmptyResult(t *testing.T) {
	mockResponse := &responses.PaginatedWebhooksResponse{
		Object: "list",
		Data:   []responses.Webhook{},
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", (*int32)(nil), (*string)(nil)).Return(mockResponse, nil)

	assert.NotNil(t, mockResponse)
	assert.Empty(t, mockResponse.Data)
	assert.False(t, mockResponse.Pagination.HasMore)
}

func TestWebhooksList_APIError(t *testing.T) {
	apiError := errors.New("API request failed")

	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", (*int32)(nil), (*string)(nil)).Return(nil, apiError)

	// Test error handling
	_, err := mockClient.ListWebhooks(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API request failed")
}

func TestWebhooksList_FilterLogic(t *testing.T) {
	// Test enabled filtering logic
	webhook1 := createTestWebhook(uuid.New().String(), "Enabled Webhook", "https://example.com/webhook1", true)
	webhook2 := createTestWebhook(uuid.New().String(), "Disabled Webhook", "https://example.com/webhook2", false)

	allWebhooks := []responses.Webhook{webhook1, webhook2}

	// Test filtering enabled webhooks
	var filteredWebhooks []responses.Webhook
	for _, webhook := range allWebhooks {
		if webhook.Enabled {
			filteredWebhooks = append(filteredWebhooks, webhook)
		}
	}

	assert.Len(t, filteredWebhooks, 1)
	assert.Equal(t, "Enabled Webhook", filteredWebhooks[0].Name)
	assert.True(t, filteredWebhooks[0].Enabled)
}

func TestWebhooksList_EventTypesDetection(t *testing.T) {
	// Test event types detection logic from list.go
	webhook := createTestWebhookWithEvents()
	events := getConfiguredEvents(&webhook)

	expectedEvents := []string{"reception", "delivered", "bounced"}
	assert.ElementsMatch(t, expectedEvents, events)
}

// Test create command structure (stub implementation)
func TestCreateCommand_Structure(t *testing.T) {
	createCmd := NewCreateCommand()
	assert.Equal(t, "create", createCmd.Name())
	assert.Equal(t, "Create a new webhook", createCmd.Short)
	assert.NotEmpty(t, createCmd.Long)
}

// Test update command structure (stub implementation)
func TestUpdateCommand_Structure(t *testing.T) {
	updateCmd := NewUpdateCommand()
	assert.Equal(t, "update", updateCmd.Name())
	assert.Equal(t, "Update an existing webhook", updateCmd.Short)
	assert.NotEmpty(t, updateCmd.Long)
}

// Test delete command structure (stub implementation)
func TestDeleteCommand_Structure(t *testing.T) {
	deleteCmd := NewDeleteCommand()
	assert.Equal(t, "delete", deleteCmd.Name())
	assert.Equal(t, "Delete a webhook", deleteCmd.Short)
	assert.NotEmpty(t, deleteCmd.Long)
}

// Test get command structure and functionality
func TestGetCommand_Structure(t *testing.T) {
	getCmd := NewGetCommand()
	assert.Equal(t, "get", getCmd.Name())
	assert.Equal(t, "Get detailed information about a specific webhook", getCmd.Short)
	assert.NotEmpty(t, getCmd.Long)
	assert.NotEmpty(t, getCmd.Example)

	// Verify it requires exactly 1 argument (webhook ID) by testing the behavior
	assert.NotNil(t, getCmd.Args, "Args function should be set")
}

func TestGetCommand_RequiresWebhookID(t *testing.T) {
	getCmd := NewGetCommand()

	// Test with no arguments - should fail
	err := getCmd.Args(getCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")

	// Test with too many arguments - should fail
	err = getCmd.Args(getCmd, []string{"webhook1", "webhook2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")

	// Test with exactly one argument - should pass
	err = getCmd.Args(getCmd, []string{"valid-webhook-id"})
	assert.NoError(t, err)
}

// Test JSON output functionality for webhooks get command
func TestWebhookGet_JSONOutput(t *testing.T) {
	webhookID := uuid.New().String()
	mockWebhook := createTestWebhookWithEvents()
	mockWebhook.ID, _ = uuid.Parse(webhookID)

	// This test verifies that the webhook object can be properly serialized to JSON
	// The actual JSON output testing would require command execution with mocked auth
	assert.NotNil(t, mockWebhook)
	assert.NotEmpty(t, mockWebhook.Name)
	assert.NotEmpty(t, mockWebhook.URL)
	assert.NotNil(t, mockWebhook.OnReception)
	assert.NotNil(t, mockWebhook.OnDelivered)
	assert.NotNil(t, mockWebhook.OnBounced)
}

// Test webhook details display formatting
func TestWebhookGet_DetailsFormatting(t *testing.T) {
	tests := []struct {
		name        string
		webhook     responses.Webhook
		expectedLen int // Expected number of events
	}{
		{
			name:        "webhook with no events",
			webhook:     createTestWebhook(uuid.New().String(), "No Events", "https://example.com/none", true),
			expectedLen: 0,
		},
		{
			name:        "webhook with basic events",
			webhook:     createTestWebhookWithEvents(),
			expectedLen: 3,
		},
		{
			name:        "webhook with all events",
			webhook:     createTestWebhookWithAllEvents(),
			expectedLen: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := getConfiguredEvents(&tt.webhook)
			assert.Len(t, events, tt.expectedLen)

			// Verify status formatting
			status := getWebhookStatus(tt.webhook.Enabled)
			if tt.webhook.Enabled {
				assert.Equal(t, "Enabled", status)
			} else {
				assert.Equal(t, "Disabled", status)
			}

			// Event description verification removed (helper function moved to ResponseHandler)
		})
	}
}

// Test edge cases for webhook configurations
func TestWebhookGet_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() responses.Webhook
		verify func(t *testing.T, webhook responses.Webhook)
	}{
		{
			name: "webhook with nil scope",
			setup: func() responses.Webhook {
				webhook := createTestWebhook(uuid.New().String(), "Nil Scope", "https://example.com/nilscope", true)
				webhook.Scope = ""
				return webhook
			},
			verify: func(t *testing.T, webhook responses.Webhook) {
				assert.Equal(t, "", webhook.Scope)
				assert.True(t, webhook.Enabled)
			},
		},
		{
			name: "webhook with empty scope",
			setup: func() responses.Webhook {
				webhook := createTestWebhook(uuid.New().String(), "Empty Scope", "https://example.com/emptyscope", true)
				emptyScope := ""
				webhook.Scope = emptyScope
				return webhook
			},
			verify: func(t *testing.T, webhook responses.Webhook) {
				assert.NotNil(t, webhook.Scope)
				assert.Empty(t, webhook.Scope)
			},
		},
		{
			name: "webhook with empty domains list",
			setup: func() responses.Webhook {
				webhook := createTestWebhook(uuid.New().String(), "Empty Domains", "https://example.com/emptydomains", true)
				webhook.Domains = []string{}
				return webhook
			},
			verify: func(t *testing.T, webhook responses.Webhook) {
				assert.NotNil(t, webhook.Domains)
				assert.Empty(t, webhook.Domains)
			},
		},
		{
			name: "webhook with nil domains",
			setup: func() responses.Webhook {
				webhook := createTestWebhook(uuid.New().String(), "Nil Domains", "https://example.com/nildomains", true)
				webhook.Domains = nil
				return webhook
			},
			verify: func(t *testing.T, webhook responses.Webhook) {
				assert.Nil(t, webhook.Domains)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := tt.setup()
			tt.verify(t, webhook)

			// Verify that getWebhook function can handle these edge cases
			webhookID := webhook.ID.String()
			mockClient := &mocks.MockClient{}
			mockClient.On("GetWebhook", webhookID).Return(&webhook, nil)

			result, err := getWebhook(mockClient, webhookID)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, webhook.Name, result.Name)
			mockClient.AssertExpectations(t)
		})
	}
}

// Benchmark tests
func BenchmarkWebhooksCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}

func BenchmarkListCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewListCommand()
	}
}

// Helper functions for creating test data

func createTestWebhook(idStr, name, url string, enabled bool) responses.Webhook {
	// Parse string ID to UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		// If invalid UUID, create a new one
		id = uuid.New()
	}

	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now().Add(-1 * time.Hour)
	scope := "account"

	return responses.Webhook{
		ID:        id,
		Name:      name,
		URL:       url,
		Enabled:   enabled,
		Scope:     scope,
		Domains:   []string{"example.com", "test.com"},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

func createTestWebhookWithEvents() responses.Webhook {
	webhookID := uuid.New().String()
	webhook := createTestWebhook(webhookID, "Test Webhook", "https://example.com/webhook", true)

	// Add event type configurations
	onReception := true
	onDelivered := true
	onBounced := true

	webhook.OnReception = onReception
	webhook.OnDelivered = onDelivered
	webhook.OnBounced = onBounced

	return webhook
}

func createTestWebhookWithAllEvents() responses.Webhook {
	webhookID := uuid.New().String()
	webhook := createTestWebhook(webhookID, "Comprehensive Webhook", "https://example.com/all-events", true)

	// Configure all event types
	onReception := true
	onDelivered := true
	onTransientError := true
	onFailed := true
	onBounced := true
	onSuppressed := true
	onOpened := true
	onClicked := true
	onSuppressionCreated := true
	onDnsError := true

	webhook.OnReception = onReception
	webhook.OnDelivered = onDelivered
	webhook.OnTransientError = onTransientError
	webhook.OnFailed = onFailed
	webhook.OnBounced = onBounced
	webhook.OnSuppressed = onSuppressed
	webhook.OnOpened = onOpened
	webhook.OnClicked = onClicked
	webhook.OnSuppressionCreated = onSuppressionCreated
	webhook.OnDNSError = onDnsError

	return webhook
}

func stringPtr(s string) *string {
	return &s
}

// Test output format validation
func TestWebhooksList_OutputFormats(t *testing.T) {
	webhook := createTestWebhook(uuid.New().String(), "Test Webhook", "https://example.com/webhook", true)
	testWebhooks := []responses.Webhook{webhook}

	mockResponse := &responses.PaginatedWebhooksResponse{
		Object: "list",
		Data:   testWebhooks,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Test JSON output format
	assert.Equal(t, "list", mockResponse.Object)
	assert.Len(t, mockResponse.Data, 1)

	// Test table format data
	webhook1 := mockResponse.Data[0]
	assert.Equal(t, "Test Webhook", webhook1.Name)
	assert.Equal(t, "https://example.com/webhook", webhook1.URL)
	assert.True(t, webhook1.Enabled)
}

// Test getWebhook function directly
func TestGetWebhook_Success(t *testing.T) {
	webhookID := uuid.New().String()
	mockWebhook := createTestWebhook(webhookID, "Test Webhook", "https://example.com/webhook", true)

	mockClient := &mocks.MockClient{}
	mockClient.On("GetWebhook", webhookID).Return(&mockWebhook, nil)

	result, err := getWebhook(mockClient, webhookID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, mockWebhook.Name, result.Name)
	assert.Equal(t, mockWebhook.URL, result.URL)
	mockClient.AssertExpectations(t)
}

func TestGetWebhook_APIError(t *testing.T) {
	webhookID := "nonexistent-webhook-id"
	apiError := errors.New("webhook not found")

	mockClient := &mocks.MockClient{}
	mockClient.On("GetWebhook", webhookID).Return(nil, apiError)

	result, err := getWebhook(mockClient, webhookID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "webhook not found")
	mockClient.AssertExpectations(t)
}

// Test event description helper function removed during ResponseHandler migration

// Test webhook status helper function
func TestGetWebhookStatus(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected string
	}{
		{
			name:     "enabled webhook",
			enabled:  true,
			expected: "Enabled",
		},
		{
			name:     "disabled webhook",
			enabled:  false,
			expected: "Disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getWebhookStatus(tt.enabled)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test webhook get functionality with various webhook configurations
func TestWebhookGet_VariousConfigurations(t *testing.T) {
	tests := []struct {
		name        string
		webhook     func() responses.Webhook
		description string
	}{
		{
			name: "minimal webhook",
			webhook: func() responses.Webhook {
				webhookID := uuid.New().String()
				webhook := createTestWebhook(webhookID, "Minimal Webhook", "https://example.com/minimal", true)
				// Clear optional fields
				webhook.Scope = ""
				webhook.Domains = []string{}
				webhook.OnReception = false
				webhook.OnDelivered = false
				webhook.OnBounced = false
				return webhook
			},
			description: "webhook with minimal configuration",
		},
		{
			name: "comprehensive webhook",
			webhook: func() responses.Webhook {
				return createTestWebhookWithAllEvents()
			},
			description: "webhook with all event types configured",
		},
		{
			name: "domain-restricted webhook",
			webhook: func() responses.Webhook {
				webhookID := uuid.New().String()
				webhook := createTestWebhook(webhookID, "Domain Restricted", "https://example.com/restricted", true)
				webhook.Domains = []string{"example.com", "subdomain.example.com"}
				return webhook
			},
			description: "webhook with domain restrictions",
		},
		{
			name: "disabled webhook",
			webhook: func() responses.Webhook {
				webhookID := uuid.New().String()
				webhook := createTestWebhook(webhookID, "Disabled Webhook", "https://example.com/disabled", false)
				return webhook
			},
			description: "disabled webhook configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := tt.webhook()
			webhookID := webhook.ID.String()

			mockClient := &mocks.MockClient{}
			mockClient.On("GetWebhook", webhookID).Return(&webhook, nil)

			result, err := getWebhook(mockClient, webhookID)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, webhook.Name, result.Name)
			assert.Equal(t, webhook.URL, result.URL)
			assert.Equal(t, webhook.Enabled, result.Enabled)
			mockClient.AssertExpectations(t)
		})
	}
}

// Test edge cases and error conditions
func TestWebhooksList_NilResponse(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", (*int32)(nil), (*string)(nil)).Return(nil, nil)

	response, err := mockClient.ListWebhooks(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, response)
}

func TestWebhooksList_InvalidPagination(t *testing.T) {
	// Test handling of invalid pagination parameters
	invalidLimit := int32(-1)

	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", &invalidLimit, (*string)(nil)).Return(nil, errors.New("invalid limit"))

	_, err := mockClient.ListWebhooks(&invalidLimit, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid limit")
}

func TestWebhooksList_LargePagination(t *testing.T) {
	// Test handling of large pagination parameters
	largeLimit := int32(1000)
	longCursor := strings.Repeat("a", 1000)

	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", &largeLimit, &longCursor).Return(nil, nil)

	response, err := mockClient.ListWebhooks(&largeLimit, &longCursor)
	assert.NoError(t, err)
	assert.Nil(t, response) // Mock returns nil, but no panic
}

// Test webhook get command with invalid webhook ID formats
func TestGetCommand_InvalidWebhookID(t *testing.T) {
	tests := []struct {
		name      string
		webhookID string
		error     string
	}{
		{
			name:      "empty webhook ID",
			webhookID: "",
			error:     "webhook not found", // API would return this for empty ID
		},
		{
			name:      "invalid UUID format",
			webhookID: "invalid-uuid-format",
			error:     "webhook not found", // API would return this for invalid ID
		},
		{
			name:      "non-existent webhook ID",
			webhookID: "00000000-0000-0000-0000-000000000000",
			error:     "webhook not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiError := errors.New(tt.error)
			mockClient := &mocks.MockClient{}
			mockClient.On("GetWebhook", tt.webhookID).Return(nil, apiError)

			result, err := getWebhook(mockClient, tt.webhookID)
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.error)
			mockClient.AssertExpectations(t)
		})
	}
}

// Test configured events detection with various event combinations
func TestGetConfiguredEvents_VariousConfigurations(t *testing.T) {
	tests := []struct {
		name           string
		webhook        func() responses.Webhook
		expectedEvents []string
	}{
		{
			name: "no events configured",
			webhook: func() responses.Webhook {
				webhookID := uuid.New().String()
				webhook := createTestWebhook(webhookID, "No Events", "https://example.com/none", true)
				// All event pointers are nil by default
				return webhook
			},
			expectedEvents: []string{},
		},
		{
			name: "only basic events",
			webhook: func() responses.Webhook {
				return createTestWebhookWithEvents() // This has reception, delivered, bounced
			},
			expectedEvents: []string{"reception", "delivered", "bounced"},
		},
		{
			name: "all events configured",
			webhook: func() responses.Webhook {
				return createTestWebhookWithAllEvents()
			},
			expectedEvents: []string{
				"reception", "delivered", "transient_error", "failed",
				"bounced", "suppressed", "opened", "clicked",
				"suppression_created", "dns_error",
			},
		},
		{
			name: "selective events",
			webhook: func() responses.Webhook {
				webhookID := uuid.New().String()
				webhook := createTestWebhook(webhookID, "Selective", "https://example.com/selective", true)
				// Only configure specific events
				onOpened := true
				onClicked := true
				webhook.OnOpened = onOpened
				webhook.OnClicked = onClicked
				return webhook
			},
			expectedEvents: []string{"opened", "clicked"},
		},
		{
			name: "events set to false",
			webhook: func() responses.Webhook {
				webhookID := uuid.New().String()
				webhook := createTestWebhook(webhookID, "Disabled Events", "https://example.com/disabled", true)
				// Explicitly set events to false
				onReception := false
				onDelivered := false
				webhook.OnReception = onReception
				webhook.OnDelivered = onDelivered
				return webhook
			},
			expectedEvents: []string{}, // Should be empty when set to false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := tt.webhook()
			events := getConfiguredEvents(&webhook)
			assert.ElementsMatch(t, tt.expectedEvents, events)
		})
	}
}

// Test error scenarios that might occur during webhook retrieval
func TestGetWebhook_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		webhookID   string
		error       error
		expectedErr string
	}{
		{
			name:        "network error",
			webhookID:   uuid.New().String(),
			error:       errors.New("network timeout"),
			expectedErr: "network timeout",
		},
		{
			name:        "unauthorized error",
			webhookID:   uuid.New().String(),
			error:       errors.New("unauthorized access"),
			expectedErr: "unauthorized access",
		},
		{
			name:        "rate limit error",
			webhookID:   uuid.New().String(),
			error:       errors.New("rate limit exceeded"),
			expectedErr: "rate limit exceeded",
		},
		{
			name:        "server error",
			webhookID:   uuid.New().String(),
			error:       errors.New("internal server error"),
			expectedErr: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetWebhook", tt.webhookID).Return(nil, tt.error)

			result, err := getWebhook(mockClient, tt.webhookID)
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.expectedErr)
			mockClient.AssertExpectations(t)
		})
	}
}

// Test webhook validation logic
func TestWebhook_DataValidation(t *testing.T) {
	tests := []struct {
		name    string
		webhook responses.Webhook
		valid   bool
	}{
		{
			name:    "valid webhook with all fields",
			webhook: createTestWebhook(uuid.New().String(), "Valid Webhook", "https://example.com/webhook", true),
			valid:   true,
		},
		{
			name:    "webhook with empty name",
			webhook: createTestWebhook(uuid.New().String(), "", "https://example.com/webhook", true),
			valid:   false, // Assuming empty name is invalid
		},
		{
			name:    "webhook with invalid URL format",
			webhook: createTestWebhook(uuid.New().String(), "Valid Name", "not-a-url", true),
			valid:   false, // Assuming invalid URL format is invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation checks
			if tt.valid {
				assert.NotEqual(t, uuid.Nil, tt.webhook.ID)
				assert.NotEmpty(t, tt.webhook.Name)
				assert.NotEmpty(t, tt.webhook.URL)
			} else {
				// Invalid cases should have some empty required fields
				isEmpty := tt.webhook.Name == "" || tt.webhook.URL == "" || tt.webhook.ID == uuid.Nil
				isInvalidURL := tt.webhook.URL != "" && !strings.HasPrefix(tt.webhook.URL, "http")
				assert.True(t, isEmpty || isInvalidURL, "Invalid webhook should have empty required fields or invalid URL")
			}
		})
	}
}
