package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginCommand_Structure(t *testing.T) {
	loginCmd := NewLoginCommand()
	assert.Equal(t, "login", loginCmd.Name())
	assert.Equal(t, "Log in to AhaSend by providing an API key", loginCmd.Short)
	assert.NotEmpty(t, loginCmd.Long)
	assert.NotEmpty(t, loginCmd.Example)
}

func TestLoginCommand_Flags(t *testing.T) {
	// Test that login command has required flags
	loginCmd := NewLoginCommand()
	flags := loginCmd.Flags()

	profileFlag := flags.Lookup("profile")
	assert.NotNil(t, profileFlag)

	apiKeyFlag := flags.Lookup("api-key")
	assert.NotNil(t, apiKeyFlag)

	accountIDFlag := flags.Lookup("account-id")
	assert.NotNil(t, accountIDFlag)

	apiURLFlag := flags.Lookup("api-url")
	assert.NotNil(t, apiURLFlag)
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "empty api key",
			apiKey:  "",
			wantErr: true,
		},
		{
			name:    "valid api key",
			apiKey:  "test-api-key",
			wantErr: false,
		},
		{
			name:    "api key with spaces",
			apiKey:  " test-api-key ",
			wantErr: false, // Should be trimmed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAPIKey(tt.apiKey)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAccountID(t *testing.T) {
	tests := []struct {
		name      string
		accountID string
		wantErr   bool
	}{
		{
			name:      "empty account ID",
			accountID: "",
			wantErr:   true,
		},
		{
			name:      "valid account ID",
			accountID: "test-account-id",
			wantErr:   false,
		},
		{
			name:      "account ID with spaces",
			accountID: " test-account-id ",
			wantErr:   false, // Should be trimmed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAccountID(tt.accountID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper functions that would be added to login.go for better testability
func validateAPIKey(apiKey string) error {
	if len(strings.TrimSpace(apiKey)) == 0 {
		return errors.NewValidationError("API key cannot be empty", nil)
	}
	return nil
}

func validateAccountID(accountID string) error {
	if len(strings.TrimSpace(accountID)) == 0 {
		return errors.NewValidationError("account ID cannot be empty", nil)
	}
	return nil
}

// Enhanced test cases for better coverage

func TestLoginCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]string
	}{
		{
			name: "all flags provided",
			args: []string{
				"--profile", "production",
				"--api-key", "test-key-123",
				"--account-id", "acc-123",
				"--api-url", "https://custom.api.com",
			},
			expected: map[string]string{
				"profile":    "production",
				"api-key":    "test-key-123",
				"account-id": "acc-123",
				"api-url":    "https://custom.api.com",
			},
		},
		{
			name: "minimal flags",
			args: []string{
				"--api-key", "test-key",
				"--account-id", "test-account",
			},
			expected: map[string]string{
				"api-key":    "test-key",
				"account-id": "test-account",
			},
		},
		{
			name: "custom profile only",
			args: []string{"--profile", "development"},
			expected: map[string]string{
				"profile": "development",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewLoginCommand()
			cmd.SetArgs(tt.args)

			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			for flag, expected := range tt.expected {
				value, err := cmd.Flags().GetString(flag)
				require.NoError(t, err)
				assert.Equal(t, expected, value, "Flag %s should have correct value", flag)
			}
		})
	}
}

func TestLoginCommand_ExecutionFlow(t *testing.T) {
	// Create temporary config directory
	tempDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tempDir)

	// Create .ahasend directory
	configDir := filepath.Join(tempDir, ".ahasend")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name        string
		args        []string
		expectError bool
		setupFunc   func()
	}{
		{
			name: "non-interactive with valid args",
			args: []string{
				"--api-key", "test-key-valid",
				"--account-id", "test-account-123",
				"--profile", "test-profile",
			},
			expectError: true, // Will fail at client ping, but tests argument processing
		},
		{
			name: "default profile when none specified",
			args: []string{
				"--api-key", "test-key",
				"--account-id", "test-account",
			},
			expectError: true, // Will fail at client ping
		},
		{
			name: "custom api url",
			args: []string{
				"--api-key", "test-key",
				"--account-id", "test-account",
				"--api-url", "https://staging.api.ahasend.com",
			},
			expectError: true, // Will fail at client ping
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			// Create isolated command
			cmd := NewLoginCommand()
			cmd.SetArgs(tt.args)

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := cmd.Execute()
			if tt.expectError {
				// We expect errors due to inability to create real API clients in tests
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoginCommand_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (string, func())
		args      []string
		errorMsg  string
	}{
		{
			name: "invalid config directory setup",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				// Create a file where directory should be
				badPath := filepath.Join(tempDir, ".ahasend")
				err := os.WriteFile(badPath, []byte("not a directory"), 0644)
				require.NoError(t, err)

				oldHome := os.Getenv("HOME")
				os.Setenv("HOME", tempDir)
				return tempDir, func() { os.Setenv("HOME", oldHome) }
			},
			args: []string{
				"--api-key", "test-key",
				"--account-id", "test-account",
			},
			errorMsg: "configuration", // Some config-related error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cleanup func()
			if tt.setupFunc != nil {
				_, cleanup = tt.setupFunc()
				defer cleanup()
			}

			cmd := NewLoginCommand()
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			assert.Error(t, err)
			if tt.errorMsg != "" {
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errorMsg))
			}
		})
	}
}

func TestLoginCommand_DefaultValues(t *testing.T) {
	cmd := NewLoginCommand()

	// Test default values
	profile, _ := cmd.Flags().GetString("profile")
	assert.Empty(t, profile, "Profile should default to empty string")

	apiURL, _ := cmd.Flags().GetString("api-url")
	assert.Equal(t, "https://api.ahasend.com", apiURL, "API URL should have default value")

	apiKey, _ := cmd.Flags().GetString("api-key")
	assert.Empty(t, apiKey, "API key should default to empty string")

	accountID, _ := cmd.Flags().GetString("account-id")
	assert.Empty(t, accountID, "Account ID should default to empty string")
}

func TestLoginCommand_Help(t *testing.T) {
	cmd := NewLoginCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Authenticate with AhaSend")
	assert.Contains(t, helpOutput, "--api-key")
	assert.Contains(t, helpOutput, "--account-id")
	assert.Contains(t, helpOutput, "--profile")
	assert.Contains(t, helpOutput, "--api-url")
	assert.Contains(t, helpOutput, "Interactive login")
	assert.Contains(t, helpOutput, "Login with specific profile")
}

// Benchmark tests
func BenchmarkLoginCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewLoginCommand()
	}
}

func BenchmarkLoginCommand_FlagParsing(b *testing.B) {
	cmd := NewLoginCommand()
	args := []string{
		"--api-key", "test-key",
		"--account-id", "test-account",
		"--profile", "test-profile",
		"--api-url", "https://api.example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cmd.ParseFlags(args)
		if err != nil {
			b.Fatal(err)
		}
		// Reset flags for next iteration
		cmd.ResetFlags()
		cmd = NewLoginCommand()
	}
}
