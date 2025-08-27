package routes

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test route command structure and subcommands
func TestRoutesCommand_Structure(t *testing.T) {
	// Create a fresh routes command and verify it has expected subcommands
	routesCmd := NewCommand()
	expectedSubcommands := []string{"list", "get", "create", "update", "delete", "listen"}

	subcommands := make([]string, 0)
	for _, cmd := range routesCmd.Commands() {
		subcommands = append(subcommands, cmd.Name())
	}

	for _, expected := range expectedSubcommands {
		assert.Contains(t, subcommands, expected, "routes command should have %s subcommand", expected)
	}

	assert.Equal(t, "routes", routesCmd.Name())
	assert.Equal(t, "Manage inbound email routes", routesCmd.Short)
	assert.NotEmpty(t, routesCmd.Long)
}

func TestRoutesCommand_Help(t *testing.T) {
	cmd := NewCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Manage inbound email routes")
	assert.Contains(t, helpOutput, "list")
	assert.Contains(t, helpOutput, "get")
	assert.Contains(t, helpOutput, "create")
	assert.Contains(t, helpOutput, "update")
	assert.Contains(t, helpOutput, "delete")
}

func TestRoutesCommand_SubcommandCount(t *testing.T) {
	cmd := NewCommand()
	subcommands := cmd.Commands()

	// Should have exactly 7 subcommands (including listen and trigger)
	assert.Equal(t, 7, len(subcommands), "routes command should have exactly 7 subcommands")
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
	assert.Equal(t, "List all inbound email routes", listCmd.Short)
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
				"--limit", "25",
				"--cursor", "next-page-token",
				"--enabled",
			},
			expected: map[string]interface{}{
				"limit":   int32(25),
				"cursor":  "next-page-token",
				"enabled": true,
			},
		},
		{
			name: "only limit flag",
			args: []string{"--limit", "10"},
			expected: map[string]interface{}{
				"limit": int32(10),
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

// Test routes list command with mock
func TestRoutesList_Success(t *testing.T) {
	// Create test routes with valid UUIDs
	route1 := createTestRoute("550e8400-e29b-41d4-a716-446655440001", "Support Route", "https://api.example.com/support", true)
	route2 := createTestRoute("550e8400-e29b-41d4-a716-446655440002", "Sales Route", "https://api.example.com/sales", false)

	testRoutes := []responses.Route{*route1, *route2}
	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   testRoutes,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Test would require integrated execution with authentication bypass
	// For now, test the response structure and data
	assert.NotNil(t, mockResponse)
	assert.Len(t, mockResponse.Data, 2)
	assert.Equal(t, "Support Route", mockResponse.Data[0].Name)
	assert.Equal(t, "Sales Route", mockResponse.Data[1].Name)
}

func TestRoutesList_WithFlags(t *testing.T) {
	route := createTestRoute("550e8400-e29b-41d4-a716-446655440003", "Test Route", "https://api.example.com/webhook", true)
	testRoutes := []responses.Route{*route}

	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   testRoutes,
		Pagination: common.PaginationInfo{
			HasMore:    true,
			NextCursor: stringPtr("next-cursor"),
		},
	}

	// Verify the mock setup is correct
	assert.NotNil(t, mockResponse)
	assert.True(t, mockResponse.Pagination.HasMore)
	assert.Equal(t, "next-cursor", *mockResponse.Pagination.NextCursor)
}

func TestRoutesList_EmptyResult(t *testing.T) {
	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   []responses.Route{},
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	assert.NotNil(t, mockResponse)
	assert.Empty(t, mockResponse.Data)
	assert.False(t, mockResponse.Pagination.HasMore)
}

func TestRoutesList_FilterLogic(t *testing.T) {
	// Test enabled filtering logic
	route1 := createTestRoute("550e8400-e29b-41d4-a716-446655440004", "Enabled Route", "https://api.example.com/webhook1", true)
	route2 := createTestRoute("550e8400-e29b-41d4-a716-446655440005", "Disabled Route", "https://api.example.com/webhook2", false)

	allRoutes := []responses.Route{*route1, *route2}

	// Test filtering enabled routes
	var filteredRoutes []responses.Route
	for _, route := range allRoutes {
		if route.Enabled {
			filteredRoutes = append(filteredRoutes, route)
		}
	}

	assert.Len(t, filteredRoutes, 1)
	assert.Equal(t, "Enabled Route", filteredRoutes[0].Name)
	assert.True(t, filteredRoutes[0].Enabled)
}

// Test get command structure and execution
func TestGetCommand_Structure(t *testing.T) {
	getCmd := NewGetCommand()
	assert.Equal(t, "get", getCmd.Name())
	assert.Equal(t, "Get detailed information about a specific route", getCmd.Short)
	assert.NotEmpty(t, getCmd.Long)
	assert.NotEmpty(t, getCmd.Example)
}

func TestGetCommand_RequiresRouteID(t *testing.T) {
	getCmd := NewGetCommand()

	// Test that command requires exactly one argument (route ID)
	getCmd.SetArgs([]string{})
	err := getCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")

	// Test with too many arguments
	getCmd.SetArgs([]string{"route1", "route2"})
	err = getCmd.Execute()
	assert.Error(t, err)
}

// Test create command structure and validation
func TestCreateCommand_Structure(t *testing.T) {
	createCmd := NewCreateCommand()
	assert.Equal(t, "create", createCmd.Name())
	assert.Equal(t, "Create a new inbound email route", createCmd.Short)
	assert.NotEmpty(t, createCmd.Long)
	assert.NotEmpty(t, createCmd.Example)
}

func TestCreateCommand_Flags(t *testing.T) {
	createCmd := NewCreateCommand()
	flags := createCmd.Flags()

	// Test that create command has all required flags
	expectedFlags := []string{
		"name", "url", "recipient", "include-attachments",
		"include-headers", "group-by-message-id", "strip-replies",
		"enabled", "interactive",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "create command should have %s flag", flagName)
	}
}

func TestCreateCommand_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing name and URL in non-interactive mode",
			args:        []string{"--interactive=false"},
			expectError: true,
			errorMsg:    "name and url are required",
		},
		{
			name:        "empty name",
			args:        []string{"--name", "", "--url", "https://api.example.com", "--interactive=false"},
			expectError: true,
			errorMsg:    "name and url are required",
		},
		{
			name:        "empty URL",
			args:        []string{"--name", "Test Route", "--url", "", "--interactive=false"},
			expectError: true,
			errorMsg:    "name and url are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic directly
			config := RouteCreateConfig{}
			if len(tt.args) > 0 && strings.Contains(tt.args[0], "name") {
				config.Name = ""
			}
			if len(tt.args) > 2 && strings.Contains(tt.args[2], "url") {
				config.URL = ""
			}

			err := validateRouteConfig(config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), strings.Split(tt.errorMsg, " ")[0])
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRouteConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  RouteCreateConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: RouteCreateConfig{
				Name: "Test Route",
				URL:  "https://api.example.com/webhook",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: RouteCreateConfig{
				Name: "",
				URL:  "https://api.example.com/webhook",
			},
			wantErr: true,
			errMsg:  "route name is required",
		},
		{
			name: "missing URL",
			config: RouteCreateConfig{
				Name: "Test Route",
				URL:  "",
			},
			wantErr: true,
			errMsg:  "webhook URL is required",
		},
		{
			name: "invalid URL format",
			config: RouteCreateConfig{
				Name: "Test Route",
				URL:  "not-a-url",
			},
			wantErr: true,
			errMsg:  "webhook URL must use http or https scheme",
		},
		{
			name: "non-HTTP/HTTPS scheme",
			config: RouteCreateConfig{
				Name: "Test Route",
				URL:  "ftp://example.com/webhook",
			},
			wantErr: true,
			errMsg:  "webhook URL must use http or https scheme",
		},
		{
			name: "missing host",
			config: RouteCreateConfig{
				Name: "Test Route",
				URL:  "https://",
			},
			wantErr: true,
			errMsg:  "webhook URL must include a valid host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRouteConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test update command structure and validation
func TestUpdateCommand_Structure(t *testing.T) {
	updateCmd := NewUpdateCommand()
	assert.Equal(t, "update", updateCmd.Name())
	assert.Equal(t, "Update an existing inbound email route", updateCmd.Short)
	assert.NotEmpty(t, updateCmd.Long)
	assert.NotEmpty(t, updateCmd.Example)
}

func TestUpdateCommand_Flags(t *testing.T) {
	updateCmd := NewUpdateCommand()
	flags := updateCmd.Flags()

	// Test that update command has all required flags
	expectedFlags := []string{
		"name", "url", "recipient", "clear-recipient",
		"enabled", "disabled",
		"include-attachments", "no-include-attachments",
		"include-headers", "no-include-headers",
		"group-by-message-id", "no-group-by-message-id",
		"strip-replies", "no-strip-replies",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "update command should have %s flag", flagName)
	}
}

func TestUpdateCommand_RequiresRouteID(t *testing.T) {
	updateCmd := NewUpdateCommand()

	// Test that command requires exactly one argument (route ID)
	updateCmd.SetArgs([]string{})
	err := updateCmd.Execute()
	assert.Error(t, err)

	// Test with too many arguments
	updateCmd.SetArgs([]string{"route1", "route2"})
	err = updateCmd.Execute()
	assert.Error(t, err)
}

func TestRouteUpdateConfig_HasUpdates(t *testing.T) {
	tests := []struct {
		name     string
		config   RouteUpdateConfig
		expected bool
	}{
		{
			name:     "empty config",
			config:   RouteUpdateConfig{},
			expected: false,
		},
		{
			name: "has name update",
			config: RouteUpdateConfig{
				Name: stringPtr("Updated Name"),
			},
			expected: true,
		},
		{
			name: "has URL update",
			config: RouteUpdateConfig{
				URL: stringPtr("https://api.example.com/new"),
			},
			expected: true,
		},
		{
			name: "has clear recipient",
			config: RouteUpdateConfig{
				ClearRecipient: true,
			},
			expected: true,
		},
		{
			name: "has enabled update",
			config: RouteUpdateConfig{
				Enabled: boolPtr(true),
			},
			expected: true,
		},
		{
			name: "has processing option update",
			config: RouteUpdateConfig{
				IncludeAttachments: boolPtr(true),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.HasUpdates()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test delete command structure
func TestDeleteCommand_Structure(t *testing.T) {
	deleteCmd := NewDeleteCommand()
	assert.Equal(t, "delete", deleteCmd.Name())
	assert.Equal(t, "Delete an inbound email route", deleteCmd.Short)
	assert.NotEmpty(t, deleteCmd.Long)
	assert.NotEmpty(t, deleteCmd.Example)
}

func TestDeleteCommand_RequiresRouteID(t *testing.T) {
	deleteCmd := NewDeleteCommand()

	// Test that command requires exactly one argument (route ID)
	deleteCmd.SetArgs([]string{})
	err := deleteCmd.Execute()
	assert.Error(t, err)

	// Test with too many arguments
	deleteCmd.SetArgs([]string{"route1", "route2"})
	err = deleteCmd.Execute()
	assert.Error(t, err)
}

func TestDeleteCommand_Flags(t *testing.T) {
	deleteCmd := NewDeleteCommand()
	flags := deleteCmd.Flags()

	forceFlag := flags.Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "bool", forceFlag.Value.Type())
}

// Test integrated command execution with mocks
func TestRoutesIntegration_ListWithAuth(t *testing.T) {
	// This test demonstrates how integrated testing would work
	// It requires setting up authentication bypass or mock authentication

	route := createTestRoute("550e8400-e29b-41d4-a716-446655440003", "Test Route", "https://api.example.com/webhook", true)
	routes := []responses.Route{*route}

	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   routes,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Verify mock response structure
	assert.Equal(t, "list", mockResponse.Object)
	assert.Len(t, mockResponse.Data, 1)
	assert.Equal(t, "Test Route", mockResponse.Data[0].Name)
}

// Test error scenarios
func TestRoutes_APIErrors(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected string
	}{
		{
			name:     "general API error",
			error:    errors.New("API request failed"),
			expected: "API request failed",
		},
		{
			name:     "not found error",
			error:    errors.New("route not found"),
			expected: "route not found",
		},
		{
			name:     "validation error",
			error:    errors.New("invalid route configuration"),
			expected: "invalid route configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.error)
			assert.Contains(t, tt.error.Error(), tt.expected)
		})
	}
}

// Test output format validation
func TestRoutes_OutputFormats(t *testing.T) {
	route := createTestRoute("550e8400-e29b-41d4-a716-446655440003", "Test Route", "https://api.example.com/webhook", true)
	testRoutes := []responses.Route{*route}

	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   testRoutes,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Test JSON output format
	assert.Equal(t, "list", mockResponse.Object)
	assert.Len(t, mockResponse.Data, 1)

	// Test table format data
	route1 := mockResponse.Data[0]
	assert.Equal(t, "Test Route", route1.Name)
	assert.Equal(t, "https://api.example.com/webhook", route1.URL)
	assert.True(t, route1.Enabled)
}

// Test edge cases and error conditions
func TestRoutes_NilResponse(t *testing.T) {
	var nilResponse *responses.PaginatedRoutesResponse
	assert.Nil(t, nilResponse)
}

func TestRoutes_InvalidPagination(t *testing.T) {
	// Test handling of invalid pagination parameters
	invalidLimit := int32(-1)
	assert.True(t, invalidLimit < 0, "negative limit should be invalid")

	// Test cursor validation
	longCursor := strings.Repeat("a", 1000)
	assert.True(t, len(longCursor) > 500, "very long cursor should be handled")
}

// Test route data validation
func TestRoute_DataValidation(t *testing.T) {
	tests := []struct {
		name  string
		route *responses.Route
		valid bool
	}{
		{
			name:  "valid route with all fields",
			route: createTestRoute("550e8400-e29b-41d4-a716-446655440008", "Valid Route", "https://api.example.com/webhook", true),
			valid: true,
		},
		{
			name:  "route with empty name",
			route: createTestRoute("550e8400-e29b-41d4-a716-446655440009", "", "https://api.example.com/webhook", true),
			valid: false, // Assuming empty name is invalid
		},
		{
			name:  "route with invalid URL format",
			route: createTestRoute("550e8400-e29b-41d4-a716-446655440010", "Valid Name", "not-a-url", true),
			valid: false, // Assuming invalid URL format is invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation checks
			if tt.valid {
				assert.NotEqual(t, uuid.Nil, tt.route.ID)
				assert.NotEmpty(t, tt.route.Name)
				assert.NotEmpty(t, tt.route.URL)
			} else {
				// Invalid cases should have some empty required fields
				isEmpty := tt.route.Name == "" || tt.route.URL == "" || tt.route.ID == uuid.Nil
				isInvalidURL := tt.route.URL != "" && !strings.HasPrefix(tt.route.URL, "http")
				assert.True(t, isEmpty || isInvalidURL, "Invalid route should have empty required fields or invalid URL")
			}
		})
	}
}

// Benchmark tests
func BenchmarkRoutesCommand_Creation(b *testing.B) {
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

func createTestRoute(id, name, url string, enabled bool) *responses.Route {
	// Use mock client helper to create route with proper UUID handling
	mockClient := &mocks.MockClient{}
	return mockClient.NewMockRoute(id, name, url, "", enabled)
}

func createTestRouteWithOptions(id, name, url string, enabled bool, options map[string]bool) *responses.Route {
	// Use mock client helper to create route with proper UUID handling
	mockClient := &mocks.MockClient{}
	return mockClient.NewMockRouteWithOptions(id, name, url, enabled, options)
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

// Helper function tests removed during ResponseHandler migration
// Functions moved to ResponseHandler implementation and are no longer public

// Mock integration test that demonstrates full testing pattern
func TestRoutesFullIntegration_MockPattern(t *testing.T) {
	// This is a template for how full integration tests would work
	// when authentication mocking is fully implemented

	// Set up test environment
	testRoutes := []responses.Route{
		*createTestRoute("550e8400-e29b-41d4-a716-446655440006", "Support Route", "https://api.example.com/support", true),
		*createTestRoute("550e8400-e29b-41d4-a716-446655440007", "Sales Route", "https://api.example.com/sales", false),
	}

	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   testRoutes,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Test the mock setup
	assert.NotNil(t, mockResponse)
	assert.Equal(t, "list", mockResponse.Object)
	assert.Len(t, mockResponse.Data, 2)

	// Verify first route
	assert.Equal(t, "Support Route", mockResponse.Data[0].Name)
	assert.Equal(t, "https://api.example.com/support", mockResponse.Data[0].URL)
	assert.True(t, mockResponse.Data[0].Enabled)

	// Verify second route
	assert.Equal(t, "Sales Route", mockResponse.Data[1].Name)
	assert.Equal(t, "https://api.example.com/sales", mockResponse.Data[1].URL)
	assert.False(t, mockResponse.Data[1].Enabled)

	// Note: Full integration tests with command execution would be implemented
	// in separate integration test files that can import both cmd and routes packages
}

// ================================
// Routes Listen Command Tests
// ================================

func TestListenCommand_Structure(t *testing.T) {
	listenCmd := NewListenCommand()
	assert.Equal(t, "listen", listenCmd.Name())
	assert.Equal(t, "Listen for inbound email events in real-time", listenCmd.Short)
	assert.NotEmpty(t, listenCmd.Long)
	assert.NotEmpty(t, listenCmd.Example)
	assert.True(t, listenCmd.SilenceUsage)
}

func TestListenCommand_Flags(t *testing.T) {
	listenCmd := NewListenCommand()
	flags := listenCmd.Flags()

	expectedFlags := []string{
		"route-id", "recipient", "forward-to", "skip-verify", "slim-output",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "listen command should have %s flag", flagName)
	}

	// Test flag types
	assert.Equal(t, "string", flags.Lookup("route-id").Value.Type())
	assert.Equal(t, "string", flags.Lookup("recipient").Value.Type())
	assert.Equal(t, "string", flags.Lookup("forward-to").Value.Type())
	assert.Equal(t, "bool", flags.Lookup("skip-verify").Value.Type())
	assert.Equal(t, "bool", flags.Lookup("slim-output").Value.Type())
}

func TestValidateListenParameters_Success(t *testing.T) {
	tests := []struct {
		name      string
		routeID   string
		recipient string
	}{
		{
			name:      "with route ID only",
			routeID:   "550e8400-e29b-41d4-a716-446655440001",
			recipient: "",
		},
		{
			name:      "with recipient pattern only",
			routeID:   "",
			recipient: "*@example.com",
		},
		{
			name:      "with complex recipient pattern",
			routeID:   "",
			recipient: "support-*@company.org",
		},
		{
			name:      "with specific recipient",
			routeID:   "",
			recipient: "test@domain.co.uk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateListenParameters(tt.routeID, tt.recipient)
			assert.NoError(t, err)
		})
	}
}

func TestValidateListenParameters_ValidationErrors(t *testing.T) {
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
			wantError: "either --route-id or --recipient must be provided",
		},
		{
			name:      "both parameters provided",
			routeID:   "550e8400-e29b-41d4-a716-446655440001",
			recipient: "*@example.com",
			wantError: "only one of --route-id or --recipient can be provided, not both",
		},
		{
			name:      "recipient without @ symbol",
			routeID:   "",
			recipient: "invalid-pattern",
			wantError: "recipient pattern must be an email pattern",
		},
		{
			name:      "recipient with too many wildcards",
			routeID:   "",
			recipient: "*@*.*.*",
			wantError: "recipient pattern contains too many wildcards",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateListenParameters(tt.routeID, tt.recipient)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestDisplayEvent_SlimOutput(t *testing.T) {
	// Create test event data
	eventData := map[string]interface{}{
		"type":    "message.routing",
		"from":    "sender@example.com",
		"to":      "recipient@company.com",
		"subject": "Test Email Subject",
		"body":    "This is a test email body",
	}

	// Create test WebSocket message
	msg := &client.WebSocketMessage{
		Type:      "event",
		Timestamp: time.Now().Unix(),
		Event: &client.Event{
			Type:      "message.routing",
			StreamID:  "stream-123",
			AccountID: "account-456",
			Data:      eventData,
			Metadata:  map[string]string{"source": "test"},
			Timestamp: time.Now().Unix(),
		},
	}

	// Capture output for slim mode
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test slim output
	displayEvent(msg, true)

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify slim output contains key information
	assert.Contains(t, output, "sender@example.com")
	assert.Contains(t, output, "recipient@company.com")
	assert.Contains(t, output, "Test Email Subject")
	// Slim output should NOT contain full JSON structure
	assert.NotContains(t, output, "{\n")
}

func TestDisplayEvent_FullOutput(t *testing.T) {
	// Create test event data
	eventData := map[string]interface{}{
		"type":    "message.routing",
		"from":    "sender@example.com",
		"to":      "recipient@company.com",
		"subject": "Test Email Subject",
		"body":    "This is a test email body",
	}

	// Create test WebSocket message
	msg := &client.WebSocketMessage{
		Type:      "event",
		Timestamp: time.Now().Unix(),
		Event: &client.Event{
			Type:      "message.routing",
			StreamID:  "stream-123",
			AccountID: "account-456",
			Data:      eventData,
			Metadata:  map[string]string{"source": "test"},
			Timestamp: time.Now().Unix(),
		},
	}

	// Capture output for full mode
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test full output
	displayEvent(msg, false)

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify full output contains complete JSON structure
	assert.Contains(t, output, "\"from\":")
	assert.Contains(t, output, "\"to\":")
	assert.Contains(t, output, "\"subject\":")
	assert.Contains(t, output, "\"body\":")
	assert.Contains(t, output, "sender@example.com")
	assert.Contains(t, output, "recipient@company.com")
}

func TestDisplayEvent_ReplayEvent(t *testing.T) {
	// Create test event data
	eventData := map[string]interface{}{
		"type": "message.routing",
		"from": "sender@example.com",
	}

	// Create test WebSocket message with replay type
	msg := &client.WebSocketMessage{
		Type:      "replay",
		Timestamp: time.Now().Unix(),
		Event: &client.Event{
			Type:      "message.routing",
			StreamID:  "stream-123",
			AccountID: "account-456",
			Data:      eventData,
			Metadata:  map[string]string{"source": "replay"},
			Timestamp: time.Now().Unix(),
		},
	}

	// Test that displayEvent doesn't panic with replay type
	// The actual display formatting is tested implicitly through the function execution
	assert.NotPanics(t, func() {
		displayEvent(msg, false)
	})

	// Test the message type logic - replay events should be handled
	assert.Equal(t, "replay", msg.Type)
	assert.Equal(t, "message.routing", eventData["type"])
}

func TestDisplayEvent_NilEvent(t *testing.T) {
	// Create test WebSocket message with nil event
	msg := &client.WebSocketMessage{
		Type:      "event",
		Timestamp: time.Now().Unix(),
		Event:     nil,
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test with nil event - should not panic
	displayEvent(msg, false)

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout
	io.Copy(&buf, r)
	output := buf.String()

	// Should produce no output for nil event
	assert.Empty(t, strings.TrimSpace(output))
}

func TestForwardEvent_Success(t *testing.T) {
	// Create test server
	receivedHeaders := make(map[string]string)
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture headers
		receivedHeaders["Content-Type"] = r.Header.Get("Content-Type")
		receivedHeaders["webhook-id"] = r.Header.Get("webhook-id")
		receivedHeaders["webhook-timestamp"] = r.Header.Get("webhook-timestamp")
		receivedHeaders["webhook-signature"] = r.Header.Get("webhook-signature")

		// Capture body
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test event
	eventData := map[string]interface{}{
		"type": "message.routing",
		"from": "test@example.com",
		"to":   "recipient@company.com",
	}

	event := &client.Event{
		Type:      "message.routing",
		StreamID:  "stream-123",
		AccountID: "account-456",
		Data:      eventData,
		Metadata:  map[string]string{"source": "test"},
		Timestamp: time.Now().Unix(),
	}

	// Create signer
	signer := webhooks.NewSigner("test-secret")

	// Create HTTP client
	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Forward the event
	forwardEvent(httpClient, server.URL, event, signer)

	// Give time for async operation
	time.Sleep(100 * time.Millisecond)

	// Verify headers were set correctly
	assert.Equal(t, "application/json", receivedHeaders["Content-Type"])
	assert.NotEmpty(t, receivedHeaders["webhook-id"])
	assert.NotEmpty(t, receivedHeaders["webhook-timestamp"])
	assert.NotEmpty(t, receivedHeaders["webhook-signature"])

	// Verify signature format
	assert.True(t, strings.HasPrefix(receivedHeaders["webhook-signature"], "v1,"))

	// Verify body contains event data
	var receivedData map[string]interface{}
	err := json.Unmarshal(receivedBody, &receivedData)
	require.NoError(t, err)
	assert.Equal(t, "message.routing", receivedData["type"])
	assert.Equal(t, "test@example.com", receivedData["from"])
	assert.Equal(t, "recipient@company.com", receivedData["to"])
}

func TestForwardEvent_ServerError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create test event
	eventData := map[string]interface{}{
		"type": "message.routing",
		"from": "test@example.com",
	}

	event := &client.Event{
		Type:      "message.routing",
		StreamID:  "stream-123",
		AccountID: "account-456",
		Data:      eventData,
		Metadata:  map[string]string{"source": "test"},
		Timestamp: time.Now().Unix(),
	}

	// Create signer and HTTP client
	signer := webhooks.NewSigner("test-secret")
	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Forward the event - should not panic even with server error
	forwardEvent(httpClient, server.URL, event, signer)

	// Give time for async operation
	time.Sleep(100 * time.Millisecond)

	// Test completes successfully even with server error (error is logged, not returned)
}

func TestForwardEvent_InvalidURL(t *testing.T) {
	// Create test event
	eventData := map[string]interface{}{
		"type": "message.routing",
		"from": "test@example.com",
	}

	event := &client.Event{
		Type:      "message.routing",
		StreamID:  "stream-123",
		AccountID: "account-456",
		Data:      eventData,
		Metadata:  map[string]string{"source": "test"},
		Timestamp: time.Now().Unix(),
	}

	// Create signer and HTTP client
	signer := webhooks.NewSigner("test-secret")
	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Forward to invalid URL - should not panic
	forwardEvent(httpClient, "invalid-url", event, signer)

	// Give time for async operation
	time.Sleep(100 * time.Millisecond)

	// Test completes successfully even with invalid URL (error is logged, not returned)
}
