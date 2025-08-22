package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestErrorOutputFormat tests that all commands properly handle errors in both JSON and table formats
func TestErrorOutputFormat(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		useJSON     bool
		expectError bool
		description string
	}{
		// Domain commands - invalid domain names
		{
			name:        "domains_get_invalid_domain_json",
			args:        []string{"domains", "get", "invalid-domain-that-does-not-exist.com"},
			useJSON:     true,
			expectError: false, // JSON mode returns API response with exit code 0
			description: "Domains get with invalid domain should return API JSON response",
		},
		{
			name:        "domains_get_invalid_domain_table",
			args:        []string{"domains", "get", "invalid-domain-that-does-not-exist.com"},
			useJSON:     false,
			expectError: true,
			description: "Domains get with invalid domain should return table error",
		},
		{
			name:        "domains_verify_invalid_domain_json",
			args:        []string{"domains", "verify", "invalid-domain-that-does-not-exist.com"},
			useJSON:     true,
			expectError: false, // JSON mode returns API response with exit code 0
			description: "Domains verify with invalid domain should return API JSON response",
		},
		{
			name:        "domains_verify_invalid_domain_table",
			args:        []string{"domains", "verify", "invalid-domain-that-does-not-exist.com"},
			useJSON:     false,
			expectError: true,
			description: "Domains verify with invalid domain should return table error",
		},
		// Messages send - validation errors
		{
			name:        "messages_send_no_from_json",
			args:        []string{"messages", "send", "--to", "test@example.com", "--subject", "Test", "--text", "Test"},
			useJSON:     true,
			expectError: true,
			description: "Messages send without --from should return JSON error",
		},
		{
			name:        "messages_send_no_from_table",
			args:        []string{"messages", "send", "--to", "test@example.com", "--subject", "Test", "--text", "Test"},
			useJSON:     false,
			expectError: true,
			description: "Messages send without --from should return table error",
		},
		{
			name:        "messages_send_invalid_from_json",
			args:        []string{"messages", "send", "--from", "invalid-email", "--to", "test@example.com", "--subject", "Test", "--text", "Test"},
			useJSON:     true,
			expectError: true,
			description: "Messages send with invalid --from should return JSON error",
		},
		{
			name:        "messages_send_invalid_from_table",
			args:        []string{"messages", "send", "--from", "invalid-email", "--to", "test@example.com", "--subject", "Test", "--text", "Test"},
			useJSON:     false,
			expectError: true,
			description: "Messages send with invalid --from should return table error",
		},
		// Messages send - missing content
		{
			name:        "messages_send_no_content_json",
			args:        []string{"messages", "send", "--from", "sender@example.com", "--to", "test@example.com", "--subject", "Test"},
			useJSON:     true,
			expectError: true,
			description: "Messages send without content should return JSON error",
		},
		{
			name:        "messages_send_no_content_table",
			args:        []string{"messages", "send", "--from", "sender@example.com", "--to", "test@example.com", "--subject", "Test"},
			useJSON:     false,
			expectError: true,
			description: "Messages send without content should return table error",
		},
		// Messages send - invalid recipients file
		{
			name:        "messages_send_invalid_recipients_file_json",
			args:        []string{"messages", "send", "--from", "sender@example.com", "--recipients", "nonexistent-file.json", "--subject", "Test", "--text", "Test"},
			useJSON:     true,
			expectError: true,
			description: "Messages send with invalid recipients file should return JSON error",
		},
		{
			name:        "messages_send_invalid_recipients_file_table",
			args:        []string{"messages", "send", "--from", "sender@example.com", "--recipients", "nonexistent-file.json", "--subject", "Test", "--text", "Test"},
			useJSON:     false,
			expectError: true,
			description: "Messages send with invalid recipients file should return table error",
		},
		// Messages list - validation errors
		{
			name:        "messages_list_invalid_limit_json",
			args:        []string{"messages", "list", "--limit", "500"},
			useJSON:     true,
			expectError: true,
			description: "Messages list with invalid limit should return JSON error",
		},
		{
			name:        "messages_list_invalid_limit_table",
			args:        []string{"messages", "list", "--limit", "500"},
			useJSON:     false,
			expectError: true,
			description: "Messages list with invalid limit should return table error",
		},
		// Auth status - no authentication
		{
			name:        "auth_status_no_auth_json",
			args:        []string{"auth", "status"},
			useJSON:     true,
			expectError: false, // This might not error, just show no profiles
			description: "Auth status without authentication should return JSON response",
		},
		{
			name:        "auth_status_no_auth_table",
			args:        []string{"auth", "status"},
			useJSON:     false,
			expectError: false, // This might not error, just show no profiles
			description: "Auth status without authentication should return table response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command for testing
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

			stdoutStr := stdout.String()
			stderrStr := stderr.String()
			combinedOutput := stdoutStr + stderrStr

			if tt.expectError {
				// Command should either error or produce error output
				hasError := err != nil ||
					strings.Contains(strings.ToLower(combinedOutput), "error") ||
					strings.Contains(strings.ToLower(combinedOutput), "failed") ||
					strings.Contains(strings.ToLower(combinedOutput), "invalid")

				assert.True(t, hasError, "Expected error but got none. Output: %s", combinedOutput)

				if tt.useJSON {
					// Check if output contains JSON error structure
					if len(combinedOutput) > 0 {
						// Should either be valid JSON or contain JSON-like error structure
						var jsonData interface{}
						jsonErr := json.Unmarshal([]byte(combinedOutput), &jsonData)
						if jsonErr != nil {
							// If not valid JSON, at least shouldn't contain table/emoji formatting
							assert.False(t,
								strings.Contains(combinedOutput, "✗") ||
									strings.Contains(combinedOutput, "❌") ||
									strings.Contains(combinedOutput, "⚠") ||
									strings.Contains(combinedOutput, "📁"),
								"JSON output should not contain emojis/table formatting: %s", combinedOutput)
						}
					}
				} else {
					// Table format - can contain emojis and formatting
					// Just verify it's not JSON
					if len(combinedOutput) > 0 {
						var jsonData interface{}
						jsonErr := json.Unmarshal([]byte(combinedOutput), &jsonData)
						assert.Error(t, jsonErr, "Table output should not be valid JSON: %s", combinedOutput)
					}
				}
			}

			// Log the output for debugging
			t.Logf("Command: %v", fullArgs)
			t.Logf("Error: %v", err)
			t.Logf("Stdout: %s", stdoutStr)
			t.Logf("Stderr: %s", stderrStr)
		})
	}
}

// TestValidationErrorOutputFormat tests validation errors specifically
func TestValidationErrorOutputFormat(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		useJSON     bool
		description string
	}{
		{
			name:        "domains_get_no_args_json",
			args:        []string{"domains", "get"},
			useJSON:     true,
			description: "Domains get without arguments should show JSON usage error",
		},
		{
			name:        "domains_get_no_args_table",
			args:        []string{"domains", "get"},
			useJSON:     false,
			description: "Domains get without arguments should show table usage error",
		},
		{
			name:        "domains_verify_no_args_json",
			args:        []string{"domains", "verify"},
			useJSON:     true,
			description: "Domains verify without arguments should show JSON usage error",
		},
		{
			name:        "domains_verify_no_args_table",
			args:        []string{"domains", "verify"},
			useJSON:     false,
			description: "Domains verify without arguments should show table usage error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command for testing
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

			// Should always error for missing required arguments
			assert.Error(t, err, "Command should error for missing required arguments")

			combinedOutput := stdout.String() + stderr.String()

			// Log the output for debugging
			t.Logf("Command: %v", fullArgs)
			t.Logf("Error: %v", err)
			t.Logf("Combined output: %s", combinedOutput)

			if tt.useJSON {
				// For JSON output, we shouldn't see usage information (SilenceUsage: true)
				// But if there's output, it should be JSON-like or at least not contain table formatting
				if len(combinedOutput) > 0 {
					assert.False(t,
						strings.Contains(combinedOutput, "Usage:") ||
							strings.Contains(combinedOutput, "Examples:") ||
							strings.Contains(combinedOutput, "Flags:"),
						"JSON output should not contain usage information: %s", combinedOutput)
				}
			}
		})
	}
}

// TestNetworkErrorOutputFormat tests network/API errors
func TestNetworkErrorOutputFormat(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		useJSON     bool
		description string
	}{
		{
			name:        "domains_list_with_invalid_api_key_json",
			args:        []string{"domains", "list", "--api-key", "invalid-key", "--account-id", "invalid-account"},
			useJSON:     true,
			description: "Domains list with invalid credentials should return JSON error",
		},
		{
			name:        "domains_list_with_invalid_api_key_table",
			args:        []string{"domains", "list", "--api-key", "invalid-key", "--account-id", "invalid-account"},
			useJSON:     false,
			description: "Domains list with invalid credentials should return table error",
		},
		{
			name:        "messages_list_with_invalid_api_key_json",
			args:        []string{"messages", "list", "--api-key", "invalid-key", "--account-id", "invalid-account"},
			useJSON:     true,
			description: "Messages list with invalid credentials should return JSON error",
		},
		{
			name:        "messages_list_with_invalid_api_key_table",
			args:        []string{"messages", "list", "--api-key", "invalid-key", "--account-id", "invalid-account"},
			useJSON:     false,
			description: "Messages list with invalid credentials should return table error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command for testing
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

			// Should error due to invalid credentials
			assert.Error(t, err, "Command should error with invalid credentials")

			combinedOutput := stdout.String() + stderr.String()

			// Log the output for debugging
			t.Logf("Command: %v", fullArgs)
			t.Logf("Error: %v", err)
			t.Logf("Combined output: %s", combinedOutput)

			if tt.useJSON {
				// JSON errors should not contain emojis or formatted output
				if len(combinedOutput) > 0 {
					assert.False(t,
						strings.Contains(combinedOutput, "✗") ||
							strings.Contains(combinedOutput, "❌") ||
							strings.Contains(combinedOutput, "⚠") ||
							strings.Contains(combinedOutput, "💡"),
						"JSON output should not contain emojis: %s", combinedOutput)
				}
			}
		})
	}
}
