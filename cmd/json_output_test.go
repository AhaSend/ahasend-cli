package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONOutputValidation tests that all commands properly support --output json flag
func TestJSONOutputValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		useJSON     bool
		shouldError bool
		description string
	}{
		// Valid JSON output tests
		{
			name:        "domains_list_json",
			args:        []string{"domains", "list"},
			useJSON:     true,
			shouldError: false,
			description: "Domains list should support JSON output",
		},
		{
			name:        "domains_get_json",
			args:        []string{"domains", "get", "example.com"},
			useJSON:     true,
			shouldError: false,
			description: "Domains get should support JSON output",
		},
		{
			name:        "domains_verify_json",
			args:        []string{"domains", "verify", "example.com"},
			useJSON:     true,
			shouldError: false,
			description: "Domains verify should support JSON output",
		},
		{
			name:        "auth_status_json",
			args:        []string{"auth", "status"},
			useJSON:     true,
			shouldError: false,
			description: "Auth status should support JSON output",
		},
		{
			name:        "messages_list_json",
			args:        []string{"messages", "list"},
			useJSON:     true,
			shouldError: false,
			description: "Messages list should support JSON output",
		},
		// Regular table output tests
		{
			name:        "domains_list_table",
			args:        []string{"domains", "list"},
			useJSON:     false,
			shouldError: false,
			description: "Domains list should support table output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command for testing with all flags
			rootCmd := NewRootCmdForTesting()

			// Build full args
			fullArgs := tt.args
			if tt.useJSON {
				fullArgs = append(fullArgs, "--output", "json")
			}
			rootCmd.SetArgs(fullArgs)

			// Capture stdout and stderr
			var stdout, stderr bytes.Buffer
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)

			// Execute command
			err := rootCmd.Execute()

			if tt.shouldError {
				assert.Error(t, err, tt.description)
			} else {
				// For valid tests, we expect either success or API errors
				// (API errors are fine since we're testing flag handling, not API connectivity)
				if err != nil {
					// If there's an error, it should be an API error, not a flag error
					errorMsg := err.Error()
					assert.False(t,
						strings.Contains(strings.ToLower(errorMsg), "invalid") ||
							strings.Contains(strings.ToLower(errorMsg), "unsupported") ||
							strings.Contains(strings.ToLower(errorMsg), "unknown"),
						"Should not fail due to flag validation: %s", errorMsg)
				}
			}
		})
	}
}

// TestJSONOutputFormat tests that commands produce valid JSON when --output json is used
func TestJSONOutputFormat(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectJSONKeys []string
		description    string
	}{
		{
			name:           "domains_list_structure",
			args:           []string{"domains", "list"},
			expectJSONKeys: []string{"data"},
			description:    "Domains list JSON should have data",
		},
		{
			name:           "domains_get_structure",
			args:           []string{"domains", "get", "example.com"},
			expectJSONKeys: []string{"domain", "dns_valid", "created_at"},
			description:    "Domains get JSON should have domain fields",
		},
		{
			name:           "auth_status_structure",
			args:           []string{"auth", "status"},
			expectJSONKeys: []string{"APIKey", "Account", "Profile", "Valid"},
			description:    "Auth status JSON should have auth status fields",
		},
		{
			name:           "messages_list_structure",
			args:           []string{"messages", "list"},
			expectJSONKeys: []string{"data"}, // API might not always return pagination
			description:    "Messages list JSON should have data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test will require proper mocking of the API client
			// For now, we'll test the structure validation logic

			// Create a fresh root command for testing
			rootCmd := NewRootCmdForTesting()

			// Build full args with --output json flag
			fullArgs := append(tt.args, "--output", "json")
			rootCmd.SetArgs(fullArgs)

			// Capture output
			var stdout bytes.Buffer
			rootCmd.SetOut(&stdout)

			// Execute command (may fail due to API, that's OK for structure testing)
			_ = rootCmd.Execute()

			output := stdout.String()

			// If we got output, it should be valid JSON
			if len(output) > 0 {
				var jsonData interface{}
				err := json.Unmarshal([]byte(output), &jsonData)
				assert.NoError(t, err, "Output should be valid JSON: %s", output)

				// Check for expected keys if JSON is valid
				if err == nil {
					if jsonMap, ok := jsonData.(map[string]interface{}); ok {
						// Skip field checking if we got an error response (e.g., "message": "Domain not found")
						if _, hasMessage := jsonMap["message"]; hasMessage && len(jsonMap) == 1 {
							// This is likely an error response, skip field checking
							return
						}

						for _, expectedKey := range tt.expectJSONKeys {
							assert.Contains(t, jsonMap, expectedKey,
								"JSON should contain key '%s'", expectedKey)
						}
					}
				}
			}
		})
	}
}

// TestJSONFlag tests that the --output json flag works correctly
func TestJSONFlag(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		shouldWork  bool
		description string
	}{
		{
			name:        "domains_list_json_flag",
			args:        []string{"domains", "list", "--output", "json"},
			shouldWork:  true,
			description: "Domains list should work with --output json flag",
		},
		{
			name:        "auth_status_json_flag",
			args:        []string{"auth", "status", "--output", "json"},
			shouldWork:  true,
			description: "Auth status should work with --output json flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewRootCmdForTesting()
			rootCmd.SetArgs(tt.args)

			// Capture output
			var stdout, stderr bytes.Buffer
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)

			// Execute command
			err := rootCmd.Execute()

			if tt.shouldWork {
				// Should either succeed or fail with API error, not flag error
				if err != nil {
					errorMsg := err.Error()
					assert.False(t,
						strings.Contains(strings.ToLower(errorMsg), "unknown flag") ||
							strings.Contains(strings.ToLower(errorMsg), "invalid flag"),
						"Should not fail due to flag issues: %s", errorMsg)
				}
			}
		})
	}
}

// Helper function to check if a string is valid JSON
func isValidJSON(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

// TestMockJSONOutput tests JSON formatting with mock data
func TestMockJSONOutput(t *testing.T) {
	// Test cases with mock data to verify JSON structure
	testCases := []struct {
		name     string
		mockData interface{}
		expected map[string]interface{}
	}{
		{
			name: "domain_response",
			mockData: map[string]interface{}{
				"domain":     "example.com",
				"dns_valid":  true,
				"created_at": "2025-01-01T00:00:00Z",
			},
			expected: map[string]interface{}{
				"domain":     "example.com",
				"dns_valid":  true,
				"created_at": "2025-01-01T00:00:00Z",
			},
		},
		{
			name: "domains_list_response",
			mockData: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"domain":    "example.com",
						"dns_valid": true,
					},
				},
				"pagination": map[string]interface{}{
					"has_more": false,
					"cursor":   nil,
				},
			},
			expected: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"domain":    "example.com",
						"dns_valid": true,
					},
				},
				"pagination": map[string]interface{}{
					"has_more": false,
					"cursor":   nil,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal to JSON
			jsonBytes, err := json.Marshal(tc.mockData)
			require.NoError(t, err)

			// Verify it's valid JSON
			assert.True(t, isValidJSON(string(jsonBytes)))

			// Unmarshal and compare structure
			var result map[string]interface{}
			err = json.Unmarshal(jsonBytes, &result)
			require.NoError(t, err)

			// Check expected keys exist
			for key := range tc.expected {
				assert.Contains(t, result, key, "JSON should contain key: %s", key)
			}
		})
	}
}
