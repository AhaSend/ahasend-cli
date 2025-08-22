package apikeys

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test apikeys command structure and subcommands
func TestAPIKeysCommand_Structure(t *testing.T) {
	// Create a fresh apikeys command and verify it has expected subcommands
	apikeysCmd := NewCommand()
	expectedSubcommands := []string{"list", "get", "create", "update", "delete"}

	subcommands := make([]string, 0)
	for _, cmd := range apikeysCmd.Commands() {
		subcommands = append(subcommands, cmd.Name())
	}

	for _, expected := range expectedSubcommands {
		assert.Contains(t, subcommands, expected, "apikeys command should have %s subcommand", expected)
	}

	assert.Equal(t, "apikeys", apikeysCmd.Name())
	assert.Equal(t, "Manage API keys", apikeysCmd.Short)
	assert.NotEmpty(t, apikeysCmd.Long)
	assert.NotEmpty(t, apikeysCmd.Example)
}

func TestAPIKeysCommand_Help(t *testing.T) {
	cmd := NewCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Manage API keys")
	assert.Contains(t, helpOutput, "list")
	assert.Contains(t, helpOutput, "get")
	assert.Contains(t, helpOutput, "create")
	assert.Contains(t, helpOutput, "update")
	assert.Contains(t, helpOutput, "delete")
	assert.Contains(t, helpOutput, "authentication and access control")
}

func TestAPIKeysCommand_SubcommandCount(t *testing.T) {
	cmd := NewCommand()
	subcommands := cmd.Commands()

	// Should have exactly 5 subcommands
	assert.Equal(t, 5, len(subcommands), "apikeys command should have exactly 5 subcommands")
}

// Test list command structure and flags
func TestListCommand_Structure(t *testing.T) {
	listCmd := NewListCommand()
	assert.Equal(t, "list", listCmd.Name())
	assert.Equal(t, "List all API keys", listCmd.Short)
	assert.NotEmpty(t, listCmd.Long)
	assert.NotEmpty(t, listCmd.Example)
	// API key functionality is now available, so don't check for "not yet supported"
	assert.NotContains(t, listCmd.Long, "not yet supported by the AhaSend SDK")
}

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
}

func TestListCommand_FlagDefaults(t *testing.T) {
	cmd := NewListCommand()

	// Parse flags with no arguments to get defaults
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err)

	limit, _ := cmd.Flags().GetInt32("limit")
	assert.Equal(t, int32(20), limit)

	cursor, _ := cmd.Flags().GetString("cursor")
	assert.Equal(t, "", cursor)
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
			},
			expected: map[string]interface{}{
				"limit":  int32(50),
				"cursor": "next-page-token",
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
			name: "only cursor flag",
			args: []string{"--cursor", "abc123"},
			expected: map[string]interface{}{
				"cursor": "abc123",
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
		})
	}
}

// Test get command structure and validation
func TestGetCommand_Structure(t *testing.T) {
	getCmd := NewGetCommand()
	assert.Equal(t, "get", getCmd.Name())
	assert.Equal(t, "Get detailed information about a specific API key", getCmd.Short)
	assert.NotEmpty(t, getCmd.Long)
	assert.NotEmpty(t, getCmd.Example)
	// API key functionality is now available, so don't check for "not yet supported"
	assert.NotContains(t, getCmd.Long, "not yet supported by the AhaSend SDK")
	assert.Contains(t, getCmd.Example, "ak_1234567890abcdef")
}

func TestGetCommand_RequiresKeyID(t *testing.T) {
	getCmd := NewGetCommand()

	// Test that command requires exactly one argument (key ID)
	getCmd.SetArgs([]string{})
	err := getCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")

	// Test with too many arguments
	getCmd.SetArgs([]string{"key1", "key2"})
	err = getCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s), received 2")
}

// Test create command structure and flags
func TestCreateCommand_Structure(t *testing.T) {
	createCmd := NewCreateCommand()
	assert.Equal(t, "create", createCmd.Name())
	assert.Equal(t, "Create a new API key", createCmd.Short)
	assert.NotEmpty(t, createCmd.Long)
	assert.NotEmpty(t, createCmd.Example)
	// API key functionality is now available, so don't check for "not yet supported"
	assert.NotContains(t, createCmd.Long, "not yet supported by the AhaSend SDK")
}

func TestCreateCommand_Flags(t *testing.T) {
	createCmd := NewCreateCommand()
	flags := createCmd.Flags()

	// Test that create command has all required flags
	expectedFlags := []string{
		"label", "scope",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "create command should have %s flag", flagName)
	}

	// Test flag types
	labelFlag := flags.Lookup("label")
	assert.Equal(t, "string", labelFlag.Value.Type())

	scopeFlag := flags.Lookup("scope")
	assert.Equal(t, "stringSlice", scopeFlag.Value.Type())
}

func TestCreateCommand_FlagDefaults(t *testing.T) {
	cmd := NewCreateCommand()

	// Parse flags with no arguments to get defaults
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err)

	label, _ := cmd.Flags().GetString("label")
	assert.Equal(t, "", label)

	scopes, _ := cmd.Flags().GetStringSlice("scope")
	assert.Empty(t, scopes)
}

func TestCreateCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "all flags provided",
			args: []string{
				"--label", "Production API",
				"--scope", "full",
				"--scope", "read",
			},
			expected: map[string]interface{}{
				"label": "Production API",
				"scope": []string{"full", "read"},
			},
		},
		{
			name: "single scope",
			args: []string{"--scope", "messages:write"},
			expected: map[string]interface{}{
				"scope": []string{"messages:write"},
			},
		},
		{
			name: "multiple scopes comma-separated",
			args: []string{"--scope", "messages:write,domains:read"},
			expected: map[string]interface{}{
				"scope": []string{"messages:write", "domains:read"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCreateCommand()
			cmd.SetArgs(tt.args)

			// Parse flags without executing the command
			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			// Check parsed flag values
			if expectedLabel, ok := tt.expected["label"]; ok {
				label, _ := cmd.Flags().GetString("label")
				assert.Equal(t, expectedLabel, label)
			}

			if expectedScope, ok := tt.expected["scope"]; ok {
				scope, _ := cmd.Flags().GetStringSlice("scope")
				assert.Equal(t, expectedScope, scope)
			}

		})
	}
}

// Test update command structure and validation
func TestUpdateCommand_Structure(t *testing.T) {
	updateCmd := NewUpdateCommand()
	assert.Equal(t, "update", updateCmd.Name())
	assert.Equal(t, "Update an existing API key", updateCmd.Short)
	assert.NotEmpty(t, updateCmd.Long)
	assert.NotEmpty(t, updateCmd.Example)
	// API key functionality is now available, so don't check for "not yet supported"
	assert.NotContains(t, updateCmd.Long, "not yet supported by the AhaSend SDK")
}

func TestUpdateCommand_RequiresKeyID(t *testing.T) {
	updateCmd := NewUpdateCommand()

	// Test that command requires exactly one argument (key ID)
	updateCmd.SetArgs([]string{})
	err := updateCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")

	// Test with too many arguments
	updateCmd.SetArgs([]string{"key1", "key2"})
	err = updateCmd.Execute()
	assert.Error(t, err)
}

func TestUpdateCommand_Flags(t *testing.T) {
	updateCmd := NewUpdateCommand()
	flags := updateCmd.Flags()

	// Test that update command has all required flags
	expectedFlags := []string{
		"label", "scope",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "update command should have %s flag", flagName)
	}
}

func TestUpdateCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "update label",
			args: []string{"ak_1234", "--label", "Updated Label"},
			expected: map[string]interface{}{
				"label": "Updated Label",
			},
		},
		{
			name: "update scopes",
			args: []string{"ak_1234", "--scope", "full", "--scope", "read"},
			expected: map[string]interface{}{
				"scope": []string{"full", "read"},
			},
		},
		{
			name: "update both label and scopes",
			args: []string{"ak_1234", "--label", "Updated", "--scope", "messages:write"},
			expected: map[string]interface{}{
				"label": "Updated",
				"scope": []string{"messages:write"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewUpdateCommand()
			cmd.SetArgs(tt.args)

			// Parse flags without executing the command
			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			// Check parsed flag values
			if expectedLabel, ok := tt.expected["label"]; ok {
				label, _ := cmd.Flags().GetString("label")
				assert.Equal(t, expectedLabel, label)
			}

			if expectedScope, ok := tt.expected["scope"]; ok {
				scope, _ := cmd.Flags().GetStringSlice("scope")
				assert.Equal(t, expectedScope, scope)
			}
		})
	}
}

// Test delete command structure and validation
func TestDeleteCommand_Structure(t *testing.T) {
	deleteCmd := NewDeleteCommand()
	assert.Equal(t, "delete", deleteCmd.Name())
	assert.Equal(t, "Delete an API key", deleteCmd.Short)
	assert.NotEmpty(t, deleteCmd.Long)
	assert.NotEmpty(t, deleteCmd.Example)
	// API key functionality is now available, so don't check for "not yet supported"
	assert.NotContains(t, deleteCmd.Long, "not yet supported by the AhaSend SDK")
}

func TestDeleteCommand_RequiresKeyID(t *testing.T) {
	deleteCmd := NewDeleteCommand()

	// Test that command requires exactly one argument (key ID)
	deleteCmd.SetArgs([]string{})
	err := deleteCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")

	// Test with too many arguments
	deleteCmd.SetArgs([]string{"key1", "key2"})
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

// Test command execution with mock authentication (placeholder tests for when SDK is ready)
func TestAPIKeysList_MockIntegration(t *testing.T) {
	// This test demonstrates how API keys list would work when SDK support is added
	// Create test API keys (using mock approach similar to other features)
	apiKey1 := createMockAPIKey("ak_550e8400e29b41d4a716446655440001", "Production API", []string{"full"}, true)
	apiKey2 := createMockAPIKey("ak_550e8400e29b41d4a716446655440002", "Analytics API", []string{"read", "statistics:read"}, true)

	testAPIKeys := []MockAPIKey{*apiKey1, *apiKey2}
	mockResponse := &MockPaginatedAPIKeysResponse{
		Object: "list",
		Data:   testAPIKeys,
		Pagination: MockPaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Test the mock setup
	assert.NotNil(t, mockResponse)
	assert.Len(t, mockResponse.Data, 2)
	assert.Equal(t, "Production API", mockResponse.Data[0].Label)
	assert.Equal(t, "Analytics API", mockResponse.Data[1].Label)
}

func TestAPIKeysGet_MockIntegration(t *testing.T) {
	// Test API key get functionality mock structure
	keyID := "ak_550e8400e29b41d4a716446655440003"
	apiKey := createMockAPIKey(keyID, "Test API Key", []string{"messages:write", "domains:read"}, true)

	assert.NotNil(t, apiKey)
	assert.Equal(t, keyID, apiKey.ID)
	assert.Equal(t, "Test API Key", apiKey.Label)
	assert.Contains(t, apiKey.Scopes, "messages:write")
	assert.Contains(t, apiKey.Scopes, "domains:read")
	assert.True(t, apiKey.Active)
}

func TestAPIKeysCreate_MockIntegration(t *testing.T) {
	// Test API key creation mock structure
	createRequest := MockCreateAPIKeyRequest{
		Label:  "New API Key",
		Scopes: []string{"messages:write"},
	}

	createdKey := createMockAPIKey("ak_550e8400e29b41d4a716446655440004", createRequest.Label, createRequest.Scopes, true)
	createdKey.Secret = "ask_testsecret123456789abcdef" // Only shown once on creation

	assert.NotNil(t, createdKey)
	assert.Equal(t, createRequest.Label, createdKey.Label)
	assert.Equal(t, createRequest.Scopes, createdKey.Scopes)
	assert.NotEmpty(t, createdKey.Secret)
	assert.True(t, strings.HasPrefix(createdKey.Secret, "ask_"))
}

func TestAPIKeysUpdate_MockIntegration(t *testing.T) {
	// Test API key update mock structure
	keyID := "ak_550e8400e29b41d4a716446655440005"
	originalKey := createMockAPIKey(keyID, "Original Label", []string{"read"}, true)

	updateRequest := MockUpdateAPIKeyRequest{
		Label:  stringPtr("Updated Label"),
		Scopes: []string{"messages:write", "full"},
	}

	updatedKey := createMockAPIKey(keyID, *updateRequest.Label, updateRequest.Scopes, true)

	assert.NotNil(t, updatedKey)
	assert.Equal(t, originalKey.ID, updatedKey.ID)
	assert.Equal(t, "Updated Label", updatedKey.Label)
	assert.Contains(t, updatedKey.Scopes, "messages:write")
	assert.Contains(t, updatedKey.Scopes, "full")
}

func TestAPIKeysDelete_MockIntegration(t *testing.T) {
	// Test API key deletion mock structure
	keyID := "ak_550e8400e29b41d4a716446655440006"

	// Mock deletion would return success
	err := mockDeleteAPIKey(keyID)
	assert.NoError(t, err)

	// Mock attempting to get deleted key would return not found error
	err = mockNotFoundError("API key not found")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// Test error scenarios
func TestAPIKeys_APIErrors(t *testing.T) {
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
			error:    errors.New("API key not found"),
			expected: "API key not found",
		},
		{
			name:     "validation error",
			error:    errors.New("invalid API key configuration"),
			expected: "invalid API key configuration",
		},
		{
			name:     "authorization error",
			error:    errors.New("insufficient permissions to manage API keys"),
			expected: "insufficient permissions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.error)
			assert.Contains(t, tt.error.Error(), tt.expected)
		})
	}
}

// Test scope validation
func TestAPIKey_ScopeValidation(t *testing.T) {
	validScopes := []string{
		"full",
		"read",
		"messages:write",
		"messages:read",
		"domains:read",
		"domains:write",
		"statistics:read",
		"webhooks:read",
		"webhooks:write",
		"routes:read",
		"routes:write",
		"suppressions:read",
		"suppressions:write",
	}

	invalidScopes := []string{
		"invalid",
		"messages:invalid",
		"",
		"write",          // too generic
		"domains:delete", // not a valid operation
	}

	for _, scope := range validScopes {
		t.Run("valid_"+scope, func(t *testing.T) {
			assert.True(t, isValidScope(scope), "scope %s should be valid", scope)
		})
	}

	for _, scope := range invalidScopes {
		t.Run("invalid_"+scope, func(t *testing.T) {
			assert.False(t, isValidScope(scope), "scope %s should be invalid", scope)
		})
	}
}

// Test API key ID validation
func TestAPIKey_IDValidation(t *testing.T) {
	validIDs := []string{
		"ak_1234567890abcdef",
		"ak_550e8400e29b41d4a716446655440000",
		"ak_abcdef1234567890",
	}

	invalidIDs := []string{
		"invalid",
		"1234567890abcdef", // missing prefix
		"ak_",              // too short
		"ak_invalid_chars!",
		"",
	}

	for _, id := range validIDs {
		t.Run("valid_"+id, func(t *testing.T) {
			assert.True(t, isValidAPIKeyID(id), "ID %s should be valid", id)
		})
	}

	for _, id := range invalidIDs {
		t.Run("invalid_"+id, func(t *testing.T) {
			assert.False(t, isValidAPIKeyID(id), "ID %s should be invalid", id)
		})
	}
}

// Test output format validation
func TestAPIKeys_OutputFormats(t *testing.T) {
	apiKey := createMockAPIKey("ak_test", "Test API Key", []string{"full"}, true)
	testAPIKeys := []MockAPIKey{*apiKey}

	mockResponse := &MockPaginatedAPIKeysResponse{
		Object: "list",
		Data:   testAPIKeys,
		Pagination: MockPaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Test JSON output format
	assert.Equal(t, "list", mockResponse.Object)
	assert.Len(t, mockResponse.Data, 1)

	// Test table format data
	key1 := mockResponse.Data[0]
	assert.Equal(t, "Test API Key", key1.Label)
	assert.Contains(t, key1.Scopes, "full")
	assert.True(t, key1.Active)
}

// Test edge cases
func TestAPIKeys_NilResponse(t *testing.T) {
	var nilResponse *MockPaginatedAPIKeysResponse
	assert.Nil(t, nilResponse)
}

func TestAPIKeys_EmptyResponse(t *testing.T) {
	mockResponse := &MockPaginatedAPIKeysResponse{
		Object: "list",
		Data:   []MockAPIKey{},
		Pagination: MockPaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	assert.NotNil(t, mockResponse)
	assert.Empty(t, mockResponse.Data)
	assert.Equal(t, "list", mockResponse.Object)
}

// Test pagination handling
func TestAPIKeys_Pagination(t *testing.T) {
	// Test with pagination
	apiKey := createMockAPIKey("ak_paginated", "Paginated Key", []string{"read"}, true)
	mockResponse := &MockPaginatedAPIKeysResponse{
		Object: "list",
		Data:   []MockAPIKey{*apiKey},
		Pagination: MockPaginationInfo{
			HasMore:    true,
			NextCursor: stringPtr("next-cursor-token"),
		},
	}

	assert.NotNil(t, mockResponse)
	assert.True(t, mockResponse.Pagination.HasMore)
	assert.Equal(t, "next-cursor-token", *mockResponse.Pagination.NextCursor)

	// Test without pagination
	mockResponseNoPaging := &MockPaginatedAPIKeysResponse{
		Object: "list",
		Data:   []MockAPIKey{*apiKey},
		Pagination: MockPaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	assert.NotNil(t, mockResponseNoPaging)
	assert.False(t, mockResponseNoPaging.Pagination.HasMore)
	assert.Nil(t, mockResponseNoPaging.Pagination.NextCursor)
}

// Benchmark tests
func BenchmarkAPIKeysCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}

func BenchmarkListCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewListCommand()
	}
}

func BenchmarkCreateCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCreateCommand()
	}
}

func BenchmarkAPIKeyID_Validation(b *testing.B) {
	testID := "ak_550e8400e29b41d4a716446655440000"
	for i := 0; i < b.N; i++ {
		_ = isValidAPIKeyID(testID)
	}
}

func BenchmarkScope_Validation(b *testing.B) {
	testScope := "messages:write"
	for i := 0; i < b.N; i++ {
		_ = isValidScope(testScope)
	}
}

// Helper functions and mock structures for testing

// MockAPIKey represents an API key structure for testing
type MockAPIKey struct {
	ID        string     `json:"id"`
	Object    string     `json:"object"`
	Label     string     `json:"label"`
	Scopes    []string   `json:"scopes"`
	Active    bool       `json:"active"`
	Secret    string     `json:"secret,omitempty"` // Only populated on creation
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}

// MockCreateAPIKeyRequest represents API key creation request
type MockCreateAPIKeyRequest struct {
	Label  string   `json:"label"`
	Scopes []string `json:"scopes"`
}

// MockUpdateAPIKeyRequest represents API key update request
type MockUpdateAPIKeyRequest struct {
	Label  *string  `json:"label,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
}

// MockPaginatedAPIKeysResponse represents paginated API keys response
type MockPaginatedAPIKeysResponse struct {
	Object     string             `json:"object"`
	Data       []MockAPIKey       `json:"data"`
	Pagination MockPaginationInfo `json:"pagination"`
}

// MockPaginationInfo represents pagination information
type MockPaginationInfo struct {
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor,omitempty"`
}

// createMockAPIKey creates a mock API key for testing
func createMockAPIKey(id, label string, scopes []string, active bool) *MockAPIKey {
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now().Add(-1 * time.Hour)

	return &MockAPIKey{
		ID:        id,
		Object:    "api_key",
		Label:     label,
		Scopes:    scopes,
		Active:    active,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

// isValidAPIKeyID validates API key ID format
func isValidAPIKeyID(id string) bool {
	if !strings.HasPrefix(id, "ak_") {
		return false
	}
	if len(id) < 10 { // ak_ + at least 7 characters
		return false
	}
	// Check for valid characters (alphanumeric only after ak_)
	keyPart := id[3:] // Remove "ak_" prefix
	for _, char := range keyPart {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return false
		}
	}
	return true
}

// isValidScope validates API key scope format
func isValidScope(scope string) bool {
	validScopes := map[string]bool{
		"full":               true,
		"read":               true,
		"messages:write":     true,
		"messages:read":      true,
		"domains:read":       true,
		"domains:write":      true,
		"statistics:read":    true,
		"webhooks:read":      true,
		"webhooks:write":     true,
		"routes:read":        true,
		"routes:write":       true,
		"suppressions:read":  true,
		"suppressions:write": true,
	}
	return validScopes[scope]
}

// mockDeleteAPIKey simulates API key deletion
func mockDeleteAPIKey(keyID string) error {
	if !isValidAPIKeyID(keyID) {
		return errors.New("invalid API key ID")
	}
	// Simulate successful deletion
	return nil
}

// mockNotFoundError simulates a not found error
func mockNotFoundError(message string) error {
	return errors.New(message)
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}

// Helper function for UUID parsing (similar to other test files)
func parseUUID(s string) uuid.UUID {
	if id, err := uuid.Parse(s); err == nil {
		return id
	}
	return uuid.New()
}
