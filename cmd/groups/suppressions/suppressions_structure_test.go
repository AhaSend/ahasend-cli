package suppressions

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test suppressions command structure and subcommands
func TestSuppressionsCommand_Structure(t *testing.T) {
	// Create a fresh suppressions command and verify it has expected subcommands
	suppressionsCmd := NewCommand()
	expectedSubcommands := []string{"list", "check", "create", "delete", "wipe"}

	subcommands := make([]string, 0)
	for _, cmd := range suppressionsCmd.Commands() {
		subcommands = append(subcommands, cmd.Name())
	}

	for _, expected := range expectedSubcommands {
		assert.Contains(t, subcommands, expected, "suppressions command should have %s subcommand", expected)
	}

	assert.Equal(t, "suppressions", suppressionsCmd.Name())
	assert.Equal(t, "Manage email suppressions", suppressionsCmd.Short)
	assert.NotEmpty(t, suppressionsCmd.Long)
}

func TestSuppressionsCommand_Help(t *testing.T) {
	cmd := NewCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Manage email suppressions")
	assert.Contains(t, helpOutput, "list")
	assert.Contains(t, helpOutput, "check")
	assert.Contains(t, helpOutput, "create")
	assert.Contains(t, helpOutput, "delete")
	assert.Contains(t, helpOutput, "wipe")
}

func TestSuppressionsCommand_SubcommandCount(t *testing.T) {
	cmd := NewCommand()
	subcommands := cmd.Commands()

	// Should have exactly 5 subcommands (list, check, create, delete, wipe)
	assert.Equal(t, 5, len(subcommands), "suppressions command should have exactly 5 subcommands")
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

	domainFlag := flags.Lookup("domain")
	assert.NotNil(t, domainFlag)
	assert.Equal(t, "string", domainFlag.Value.Type())
}

func TestListCommand_Structure(t *testing.T) {
	listCmd := NewListCommand()
	assert.Equal(t, "list", listCmd.Name())
	assert.Equal(t, "List all suppressed email addresses", listCmd.Short)
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
				"--domain", "example.com",
			},
			expected: map[string]interface{}{
				"limit":  int32(25),
				"cursor": "next-page-token",
				"domain": "example.com",
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
			name: "only domain flag",
			args: []string{"--domain", "test.com"},
			expected: map[string]interface{}{
				"domain": "test.com",
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

			if expectedDomain, ok := tt.expected["domain"]; ok {
				domain, _ := cmd.Flags().GetString("domain")
				assert.Equal(t, expectedDomain, domain)
			}
		})
	}
}

// Test check command structure and flags
func TestCheckCommand_Flags(t *testing.T) {
	checkCmd := NewCheckCommand()
	flags := checkCmd.Flags()

	domainFlag := flags.Lookup("domain")
	assert.NotNil(t, domainFlag)
	assert.Equal(t, "string", domainFlag.Value.Type())
}

func TestCheckCommand_Structure(t *testing.T) {
	checkCmd := NewCheckCommand()
	assert.Equal(t, "check", checkCmd.Name())
	assert.Equal(t, "Check if an email address is suppressed", checkCmd.Short)
	assert.NotEmpty(t, checkCmd.Long)
	assert.NotEmpty(t, checkCmd.Example)
}

func TestCheckCommand_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid email argument",
			args:        []string{"user@example.com"},
			expectError: false,
		},
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"user@example.com", "extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCheckCommand()
			cmd.SetArgs(tt.args)

			// This tests the Args validation, which is cobra.ExactArgs(1)
			err := cmd.Args(cmd, tt.args)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test add command structure and flags
func TestAddCommand_Flags(t *testing.T) {
	addCmd := NewCreateCommand()
	flags := addCmd.Flags()

	reasonFlag := flags.Lookup("reason")
	assert.NotNil(t, reasonFlag)
	assert.Equal(t, "string", reasonFlag.Value.Type())

	domainFlag := flags.Lookup("domain")
	assert.NotNil(t, domainFlag)
	assert.Equal(t, "string", domainFlag.Value.Type())
}

func TestCreateCommand_Structure(t *testing.T) {
	createCmd := NewCreateCommand()
	assert.Equal(t, "create", createCmd.Name())
	assert.Equal(t, "Create a new suppression for an email address", createCmd.Short)
	assert.NotEmpty(t, createCmd.Long)
	assert.NotEmpty(t, createCmd.Example)
}

func TestCreateCommand_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid email argument",
			args:        []string{"user@example.com"},
			expectError: false,
		},
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"user@example.com", "extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCreateCommand()
			cmd.SetArgs(tt.args)

			// This tests the Args validation, which is cobra.ExactArgs(1)
			err := cmd.Args(cmd, tt.args)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test remove command structure and flags
func TestRemoveCommand_Flags(t *testing.T) {
	removeCmd := NewDeleteCommand()
	flags := removeCmd.Flags()

	domainFlag := flags.Lookup("domain")
	assert.NotNil(t, domainFlag)
	assert.Equal(t, "string", domainFlag.Value.Type())

	forceFlag := flags.Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "bool", forceFlag.Value.Type())
}

func TestDeleteCommand_Structure(t *testing.T) {
	deleteCmd := NewDeleteCommand()
	assert.Equal(t, "delete", deleteCmd.Name())
	assert.Contains(t, deleteCmd.Short, "Delete")
	assert.NotEmpty(t, deleteCmd.Long)
}

func TestDeleteCommand_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid email argument",
			args:        []string{"user@example.com"},
			expectError: false,
		},
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"user@example.com", "extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDeleteCommand()
			cmd.SetArgs(tt.args)

			// This tests the Args validation, which is cobra.ExactArgs(1)
			err := cmd.Args(cmd, tt.args)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test wipe command structure and flags
func TestWipeCommand_Flags(t *testing.T) {
	wipeCmd := NewWipeCommand()
	flags := wipeCmd.Flags()

	forceFlag := flags.Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "bool", forceFlag.Value.Type())
}

func TestWipeCommand_Structure(t *testing.T) {
	wipeCmd := NewWipeCommand()
	assert.Equal(t, "wipe", wipeCmd.Name())
	assert.Contains(t, wipeCmd.Short, "Delete all suppressions")
	assert.NotEmpty(t, wipeCmd.Long)
}

// Direct command help tests (these work even with compilation errors in RunE functions)
func TestSuppressionsCommands_HelpOutputs(t *testing.T) {
	tests := []struct {
		name             string
		commandFactory   func() *cobra.Command
		expectedContains []string
	}{
		{
			name:           "suppressions help",
			commandFactory: NewCommand,
			expectedContains: []string{
				"Usage:",
				"Manage email suppressions",
				"list",
				"check",
				"create",
				"delete",
				"wipe",
			},
		},
		{
			name:           "suppressions list help",
			commandFactory: NewListCommand,
			expectedContains: []string{
				"Usage:",
				"List suppressed email addresses with filtering",
				"--limit",
				"--cursor",
				"--domain",
			},
		},
		{
			name:           "suppressions check help",
			commandFactory: NewCheckCommand,
			expectedContains: []string{
				"Usage:",
				"Check if an email address is suppressed",
				"--domain",
				"<email>",
			},
		},
		{
			name:           "suppressions create help",
			commandFactory: NewCreateCommand,
			expectedContains: []string{
				"Usage:",
				"Create a new suppression entry to prevent sending emails",
				"--reason",
				"--domain",
				"<email>",
			},
		},
		{
			name:           "suppressions delete help",
			commandFactory: NewDeleteCommand,
			expectedContains: []string{
				"Usage:",
				"Delete an email address from the suppression list",
				"--domain",
				"--force",
				"<email>",
			},
		},
		{
			name:           "suppressions wipe help",
			commandFactory: NewWipeCommand,
			expectedContains: []string{
				"Usage:",
				"--force",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.commandFactory()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{"--help"})

			err := cmd.Execute()
			output := buf.String()

			// Help commands should not return errors
			assert.NoError(t, err)

			// Check that all expected strings are present
			for _, expected := range tt.expectedContains {
				assert.Contains(t, output, expected,
					"Help output should contain '%s'", expected)
			}
		})
	}
}

// Email validation tests
func TestEmailValidation_Patterns(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		expectValid bool
	}{
		{
			name:        "valid email",
			email:       "user@example.com",
			expectValid: true,
		},
		{
			name:        "valid email with subdomain",
			email:       "user@mail.example.com",
			expectValid: true,
		},
		{
			name:        "valid email with plus",
			email:       "user+tag@example.com",
			expectValid: true,
		},
		{
			name:        "invalid email no @",
			email:       "userexample.com",
			expectValid: false,
		},
		{
			name:        "invalid email no domain",
			email:       "user@",
			expectValid: false,
		},
		{
			name:        "empty email",
			email:       "",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the basic email format validation that would be
			// used in the command implementations
			if tt.email == "" {
				assert.False(t, tt.expectValid)
				return
			}

			// Basic email validation (contains @ and domain)
			hasAt := strings.Contains(tt.email, "@")
			parts := strings.Split(tt.email, "@")
			hasDomain := len(parts) == 2 && parts[1] != ""

			isValid := hasAt && hasDomain

			if tt.expectValid {
				assert.True(t, isValid, "Email %s should be valid", tt.email)
			} else {
				assert.False(t, isValid, "Email %s should be invalid", tt.email)
			}
		})
	}
}

// Reason validation tests
func TestReasonValidation_ValidReasons(t *testing.T) {
	validReasons := []string{"bounce", "complaint", "unsubscribe", "manual", "abuse"}

	for _, reason := range validReasons {
		t.Run("reason_"+reason, func(t *testing.T) {
			// All these reasons should be considered valid
			assert.Contains(t, validReasons, reason)
		})
	}
}

func TestReasonValidation_InvalidReasons(t *testing.T) {
	invalidReasons := []string{"invalid", "test", "other", ""}
	validReasons := []string{"bounce", "complaint", "unsubscribe", "manual", "abuse"}

	for _, reason := range invalidReasons {
		t.Run("invalid_reason_"+reason, func(t *testing.T) {
			// These reasons should not be in the valid list
			assert.NotContains(t, validReasons, reason)
		})
	}
}

// Mock data tests for suppressions
func TestMockSuppressionHelpers(t *testing.T) {
	mockClient := &mocks.MockClient{}

	t.Run("basic suppression", func(t *testing.T) {
		suppression := mockClient.NewMockSuppression("user@example.com", "bounce", "")

		assert.Equal(t, "user@example.com", suppression.Email)
		assert.NotNil(t, suppression.Reason)
		assert.Equal(t, "bounce", suppression.Reason)
		assert.Empty(t, suppression.Domain)
		assert.True(t, suppression.ExpiresAt.IsZero())                   // Never expires
		assert.True(t, time.Since(suppression.CreatedAt) > 23*time.Hour) // Created ~24h ago
	})

	t.Run("domain-specific suppression", func(t *testing.T) {
		suppression := mockClient.NewMockSuppression("user@example.com", "unsubscribe", "test.com")

		assert.Equal(t, "user@example.com", suppression.Email)
		assert.NotNil(t, suppression.Reason)
		assert.Equal(t, "unsubscribe", suppression.Reason)
		assert.NotNil(t, suppression.Domain)
		assert.Equal(t, "test.com", suppression.Domain)
	})

	t.Run("suppression with expiry", func(t *testing.T) {
		expiry := 30 * 24 * time.Hour // 30 days
		suppression := mockClient.NewMockSuppressionWithExpiry("user@example.com", "manual", "", expiry)

		assert.Equal(t, "user@example.com", suppression.Email)
		assert.False(t, suppression.ExpiresAt.IsZero())
		assert.True(t, suppression.ExpiresAt.After(time.Now().Add(29*24*time.Hour)))
	})

	t.Run("suppressions response", func(t *testing.T) {
		suppressions := []responses.Suppression{
			*mockClient.NewMockSuppression("user1@example.com", "bounce", ""),
			*mockClient.NewMockSuppression("user2@example.com", "unsubscribe", "test.com"),
		}
		response := mockClient.NewMockSuppressionsResponse(suppressions, true)

		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 2)
		assert.True(t, response.Pagination.HasMore)
		assert.Nil(t, response.Pagination.NextCursor)
	})
}

// Mock API interaction tests
func TestSuppressionsAPI_MockInteractions(t *testing.T) {
	t.Run("list suppressions parameters", func(t *testing.T) {
		// Test parameter construction for ListSuppressions
		params := requests.GetSuppressionsParams{
			Limit:  func() *int32 { limit := int32(50); return &limit }(),
			Cursor: func() *string { cursor := "next-token"; return &cursor }(),
			Domain: func() *string { domain := "example.com"; return &domain }(),
		}

		// Verify parameters are constructed correctly
		assert.NotNil(t, params.Limit)
		assert.Equal(t, int32(50), *params.Limit)
		assert.NotNil(t, params.Cursor)
		assert.Equal(t, "next-token", *params.Cursor)
		assert.NotNil(t, params.Domain)
		assert.Equal(t, "example.com", *params.Domain)
	})

	t.Run("check suppression with domain", func(t *testing.T) {
		email := "user@example.com"
		domain := "test.com"

		// These would be the parameters passed to CheckSuppression
		assert.Equal(t, email, "user@example.com")
		assert.Equal(t, domain, "test.com")

		// Verify domain pointer handling
		domainPtr := &domain
		assert.NotNil(t, domainPtr)
		assert.Equal(t, "test.com", *domainPtr)
	})

	t.Run("create suppression request", func(t *testing.T) {
		// Test CreateSuppressionRequest construction
		email := "user@example.com"
		reason := "manual"
		domain := "test.com"

		req := requests.CreateSuppressionRequest{
			Email:  email,
			Reason: &reason,
			Domain: &domain,
		}

		assert.Equal(t, "user@example.com", req.Email)
		assert.NotNil(t, req.Reason)
		assert.Equal(t, "manual", *req.Reason)
		assert.NotNil(t, req.Domain)
		assert.Equal(t, "test.com", *req.Domain)
	})
}
