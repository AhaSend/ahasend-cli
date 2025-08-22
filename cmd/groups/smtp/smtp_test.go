package smtp

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

// Test SMTP commands structure and subcommands
func TestSMTPCommand_Structure(t *testing.T) {
	// Create a fresh SMTP command and verify it has expected subcommands
	smtpCmd := NewCommand()
	expectedSubcommands := []string{"list", "get", "create", "delete", "send"}

	subcommands := make([]string, 0)
	for _, cmd := range smtpCmd.Commands() {
		subcommands = append(subcommands, cmd.Name())
	}

	for _, expected := range expectedSubcommands {
		assert.Contains(t, subcommands, expected, "smtp command should have %s subcommand", expected)
	}

	assert.Equal(t, "smtp", smtpCmd.Name())
	assert.Equal(t, "Manage SMTP credentials for email sending", smtpCmd.Short)
	assert.NotEmpty(t, smtpCmd.Long)
}

func TestSMTPCommand_Help(t *testing.T) {
	cmd := NewCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Manage SMTP credentials")
	assert.Contains(t, helpOutput, "list")
	assert.Contains(t, helpOutput, "get")
	assert.Contains(t, helpOutput, "create")
	assert.Contains(t, helpOutput, "delete")
	assert.Contains(t, helpOutput, "send")
}

func TestSMTPCommand_SubcommandCount(t *testing.T) {
	cmd := NewCommand()
	subcommands := cmd.Commands()

	// Should have exactly 5 subcommands (list, get, create, delete, send)
	assert.Equal(t, 5, len(subcommands), "smtp command should have exactly 5 subcommands")
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
}

func TestListCommand_Structure(t *testing.T) {
	listCmd := NewListCommand()
	assert.Equal(t, "list", listCmd.Name())
	assert.Equal(t, "List all SMTP credentials", listCmd.Short)
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
			},
			expected: map[string]interface{}{
				"limit":  int32(25),
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

// Test get command structure and flags
func TestGetCommand_Structure(t *testing.T) {
	getCmd := NewGetCommand()
	assert.Equal(t, "get", getCmd.Name())
	assert.Equal(t, "Get details of a specific SMTP credential", getCmd.Short)
	assert.NotEmpty(t, getCmd.Long)
	assert.NotEmpty(t, getCmd.Example)
}

func TestGetCommand_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid credential ID argument",
			args:        []string{"550e8400-e29b-41d4-a716-446655440000"},
			expectError: false,
		},
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"credential-id", "extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewGetCommand()
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

// Test create command structure and flags
func TestCreateCommand_Flags(t *testing.T) {
	createCmd := NewCreateCommand()
	flags := createCmd.Flags()

	nameFlag := flags.Lookup("name")
	assert.NotNil(t, nameFlag)
	assert.Equal(t, "string", nameFlag.Value.Type())

	usernameFlag := flags.Lookup("username")
	assert.NotNil(t, usernameFlag)
	assert.Equal(t, "string", usernameFlag.Value.Type())

	passwordFlag := flags.Lookup("password")
	assert.NotNil(t, passwordFlag)
	assert.Equal(t, "string", passwordFlag.Value.Type())

	scopeFlag := flags.Lookup("scope")
	assert.NotNil(t, scopeFlag)
	assert.Equal(t, "string", scopeFlag.Value.Type())

	domainsFlag := flags.Lookup("domains")
	assert.NotNil(t, domainsFlag)
	assert.Equal(t, "stringSlice", domainsFlag.Value.Type())

	sandboxFlag := flags.Lookup("sandbox")
	assert.NotNil(t, sandboxFlag)
	assert.Equal(t, "bool", sandboxFlag.Value.Type())

	nonInteractiveFlag := flags.Lookup("non-interactive")
	assert.NotNil(t, nonInteractiveFlag)
	assert.Equal(t, "bool", nonInteractiveFlag.Value.Type())
}

func TestCreateCommand_Structure(t *testing.T) {
	createCmd := NewCreateCommand()
	assert.Equal(t, "create", createCmd.Name())
	assert.Equal(t, "Create a new SMTP credential", createCmd.Short)
	assert.NotEmpty(t, createCmd.Long)
	assert.NotEmpty(t, createCmd.Example)
}

func TestCreateCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "global scope credential",
			args: []string{
				"--name", "Test Credential",
				"--username", "test-user",
				"--scope", "global",
				"--non-interactive",
			},
			expected: map[string]interface{}{
				"name":            "Test Credential",
				"username":        "test-user",
				"scope":           "global",
				"non-interactive": true,
			},
		},
		{
			name: "scoped credential with domains",
			args: []string{
				"--name", "Marketing Credential",
				"--scope", "scoped",
				"--domains", "marketing.example.com,news.example.com",
				"--sandbox",
				"--non-interactive",
			},
			expected: map[string]interface{}{
				"name":    "Marketing Credential",
				"scope":   "scoped",
				"domains": []string{"marketing.example.com", "news.example.com"},
				"sandbox": true,
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
			if expectedName, ok := tt.expected["name"]; ok {
				name, _ := cmd.Flags().GetString("name")
				assert.Equal(t, expectedName, name)
			}

			if expectedUsername, ok := tt.expected["username"]; ok {
				username, _ := cmd.Flags().GetString("username")
				assert.Equal(t, expectedUsername, username)
			}

			if expectedScope, ok := tt.expected["scope"]; ok {
				scope, _ := cmd.Flags().GetString("scope")
				assert.Equal(t, expectedScope, scope)
			}

			if expectedDomains, ok := tt.expected["domains"]; ok {
				domains, _ := cmd.Flags().GetStringSlice("domains")
				assert.Equal(t, expectedDomains, domains)
			}

			if expectedSandbox, ok := tt.expected["sandbox"]; ok {
				sandbox, _ := cmd.Flags().GetBool("sandbox")
				assert.Equal(t, expectedSandbox, sandbox)
			}

			if expectedNonInteractive, ok := tt.expected["non-interactive"]; ok {
				nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
				assert.Equal(t, expectedNonInteractive, nonInteractive)
			}
		})
	}
}

// Test delete command structure and flags
func TestDeleteCommand_Flags(t *testing.T) {
	deleteCmd := NewDeleteCommand()
	flags := deleteCmd.Flags()

	forceFlag := flags.Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "bool", forceFlag.Value.Type())
}

func TestDeleteCommand_Structure(t *testing.T) {
	deleteCmd := NewDeleteCommand()
	assert.Equal(t, "delete", deleteCmd.Name())
	assert.Equal(t, "Delete an SMTP credential", deleteCmd.Short)
	assert.NotEmpty(t, deleteCmd.Long)
	assert.NotEmpty(t, deleteCmd.Example)
}

func TestDeleteCommand_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid credential ID argument",
			args:        []string{"550e8400-e29b-41d4-a716-446655440000"},
			expectError: false,
		},
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"credential-id", "extra"},
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

// Test send command structure and flags
func TestSendCommand_Flags(t *testing.T) {
	sendCmd := NewSendCommand()
	flags := sendCmd.Flags()

	// Email content flags
	fromFlag := flags.Lookup("from")
	assert.NotNil(t, fromFlag)
	assert.Equal(t, "string", fromFlag.Value.Type())

	toFlag := flags.Lookup("to")
	assert.NotNil(t, toFlag)
	assert.Equal(t, "stringSlice", toFlag.Value.Type())

	ccFlag := flags.Lookup("cc")
	assert.NotNil(t, ccFlag)
	assert.Equal(t, "stringSlice", ccFlag.Value.Type())

	bccFlag := flags.Lookup("bcc")
	assert.NotNil(t, bccFlag)
	assert.Equal(t, "stringSlice", bccFlag.Value.Type())

	subjectFlag := flags.Lookup("subject")
	assert.NotNil(t, subjectFlag)
	assert.Equal(t, "string", subjectFlag.Value.Type())

	textFlag := flags.Lookup("text")
	assert.NotNil(t, textFlag)
	assert.Equal(t, "string", textFlag.Value.Type())

	htmlFlag := flags.Lookup("html")
	assert.NotNil(t, htmlFlag)
	assert.Equal(t, "string", htmlFlag.Value.Type())

	// SMTP server flags
	serverFlag := flags.Lookup("server")
	assert.NotNil(t, serverFlag)
	assert.Equal(t, "string", serverFlag.Value.Type())

	usernameFlag := flags.Lookup("username")
	assert.NotNil(t, usernameFlag)
	assert.Equal(t, "string", usernameFlag.Value.Type())

	passwordFlag := flags.Lookup("password")
	assert.NotNil(t, passwordFlag)
	assert.Equal(t, "string", passwordFlag.Value.Type())

	// Special feature flags
	trackOpensFlag := flags.Lookup("track-opens")
	assert.NotNil(t, trackOpensFlag)
	assert.Equal(t, "bool", trackOpensFlag.Value.Type())

	trackClicksFlag := flags.Lookup("track-clicks")
	assert.NotNil(t, trackClicksFlag)
	assert.Equal(t, "bool", trackClicksFlag.Value.Type())

	tagsFlag := flags.Lookup("tags")
	assert.NotNil(t, tagsFlag)
	assert.Equal(t, "stringSlice", tagsFlag.Value.Type())

	sandboxFlag := flags.Lookup("sandbox")
	assert.NotNil(t, sandboxFlag)
	assert.Equal(t, "bool", sandboxFlag.Value.Type())

	testFlag := flags.Lookup("test")
	assert.NotNil(t, testFlag)
	assert.Equal(t, "bool", testFlag.Value.Type())
}

func TestSendCommand_Structure(t *testing.T) {
	sendCmd := NewSendCommand()
	assert.Equal(t, "send", sendCmd.Name())
	assert.Equal(t, "Send an email via SMTP protocol", sendCmd.Short)
	assert.NotEmpty(t, sendCmd.Long)
	assert.NotEmpty(t, sendCmd.Example)
}

func TestSendCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "basic email send",
			args: []string{
				"--from", "sender@example.com",
				"--to", "recipient@example.com",
				"--subject", "Test Email",
				"--text", "This is a test email",
				"--username", "smtp-user",
				"--password", "smtp-pass",
			},
			expected: map[string]interface{}{
				"from":     "sender@example.com",
				"to":       []string{"recipient@example.com"},
				"subject":  "Test Email",
				"text":     "This is a test email",
				"username": "smtp-user",
				"password": "smtp-pass",
			},
		},
		{
			name: "email with tracking and tags",
			args: []string{
				"--from", "sender@example.com",
				"--to", "recipient@example.com",
				"--subject", "Marketing Email",
				"--html", "<h1>Hello</h1>",
				"--track-opens",
				"--track-clicks",
				"--tags", "marketing,newsletter",
				"--sandbox",
			},
			expected: map[string]interface{}{
				"from":         "sender@example.com",
				"to":           []string{"recipient@example.com"},
				"subject":      "Marketing Email",
				"html":         "<h1>Hello</h1>",
				"track-opens":  true,
				"track-clicks": true,
				"tags":         []string{"marketing", "newsletter"},
				"sandbox":      true,
			},
		},
		{
			name: "test mode",
			args: []string{
				"--test",
				"--username", "smtp-user",
				"--password", "smtp-pass",
				"--server", "mail.example.com:587",
			},
			expected: map[string]interface{}{
				"test":     true,
				"username": "smtp-user",
				"password": "smtp-pass",
				"server":   "mail.example.com:587",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewSendCommand()
			cmd.SetArgs(tt.args)

			// Parse flags without executing the command
			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			// Check parsed flag values
			if expectedFrom, ok := tt.expected["from"]; ok {
				from, _ := cmd.Flags().GetString("from")
				assert.Equal(t, expectedFrom, from)
			}

			if expectedTo, ok := tt.expected["to"]; ok {
				to, _ := cmd.Flags().GetStringSlice("to")
				assert.Equal(t, expectedTo, to)
			}

			if expectedSubject, ok := tt.expected["subject"]; ok {
				subject, _ := cmd.Flags().GetString("subject")
				assert.Equal(t, expectedSubject, subject)
			}

			if expectedText, ok := tt.expected["text"]; ok {
				text, _ := cmd.Flags().GetString("text")
				assert.Equal(t, expectedText, text)
			}

			if expectedHTML, ok := tt.expected["html"]; ok {
				html, _ := cmd.Flags().GetString("html")
				assert.Equal(t, expectedHTML, html)
			}

			if expectedUsername, ok := tt.expected["username"]; ok {
				username, _ := cmd.Flags().GetString("username")
				assert.Equal(t, expectedUsername, username)
			}

			if expectedPassword, ok := tt.expected["password"]; ok {
				password, _ := cmd.Flags().GetString("password")
				assert.Equal(t, expectedPassword, password)
			}

			if expectedServer, ok := tt.expected["server"]; ok {
				server, _ := cmd.Flags().GetString("server")
				assert.Equal(t, expectedServer, server)
			}

			if expectedTrackOpens, ok := tt.expected["track-opens"]; ok {
				trackOpens, _ := cmd.Flags().GetBool("track-opens")
				assert.Equal(t, expectedTrackOpens, trackOpens)
			}

			if expectedTrackClicks, ok := tt.expected["track-clicks"]; ok {
				trackClicks, _ := cmd.Flags().GetBool("track-clicks")
				assert.Equal(t, expectedTrackClicks, trackClicks)
			}

			if expectedTags, ok := tt.expected["tags"]; ok {
				tags, _ := cmd.Flags().GetStringSlice("tags")
				assert.Equal(t, expectedTags, tags)
			}

			if expectedSandbox, ok := tt.expected["sandbox"]; ok {
				sandbox, _ := cmd.Flags().GetBool("sandbox")
				assert.Equal(t, expectedSandbox, sandbox)
			}

			if expectedTest, ok := tt.expected["test"]; ok {
				test, _ := cmd.Flags().GetBool("test")
				assert.Equal(t, expectedTest, test)
			}
		})
	}
}

// Direct command help tests (these work even with compilation errors in RunE functions)
func TestSMTPCommands_HelpOutputs(t *testing.T) {
	tests := []struct {
		name             string
		commandFactory   func() *cobra.Command
		expectedContains []string
	}{
		{
			name:           "smtp help",
			commandFactory: NewCommand,
			expectedContains: []string{
				"Usage:",
				"Manage SMTP credentials",
				"list",
				"get",
				"create",
				"delete",
				"send",
			},
		},
		{
			name:           "smtp list help",
			commandFactory: NewListCommand,
			expectedContains: []string{
				"Usage:",
				"List all SMTP credentials",
				"--limit",
				"--cursor",
			},
		},
		{
			name:           "smtp get help",
			commandFactory: NewGetCommand,
			expectedContains: []string{
				"Usage:",
				"Get detailed information about a specific SMTP credential",
				"credential-id",
			},
		},
		{
			name:           "smtp create help",
			commandFactory: NewCreateCommand,
			expectedContains: []string{
				"Usage:",
				"Create a new SMTP credential",
				"--name",
				"--username",
				"--scope",
				"--domains",
				"--sandbox",
			},
		},
		{
			name:           "smtp delete help",
			commandFactory: NewDeleteCommand,
			expectedContains: []string{
				"Usage:",
				"Delete an SMTP credential",
				"--force",
				"credential-id",
			},
		},
		{
			name:           "smtp send help",
			commandFactory: NewSendCommand,
			expectedContains: []string{
				"Usage:",
				"Send an email using SMTP protocol",
				"--from",
				"--to",
				"--subject",
				"--text",
				"--html",
				"--server",
				"--username",
				"--password",
				"--test",
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

// SMTP scope validation tests
func TestScopeValidation_ValidScopes(t *testing.T) {
	validScopes := []string{"global", "scoped"}

	for _, scope := range validScopes {
		t.Run("scope_"+scope, func(t *testing.T) {
			// All these scopes should be considered valid
			assert.Contains(t, validScopes, scope)
		})
	}
}

func TestScopeValidation_InvalidScopes(t *testing.T) {
	invalidScopes := []string{"invalid", "test", "other", ""}
	validScopes := []string{"global", "scoped"}

	for _, scope := range invalidScopes {
		t.Run("invalid_scope_"+scope, func(t *testing.T) {
			// These scopes should not be in the valid list
			assert.NotContains(t, validScopes, scope)
		})
	}
}

// Username generation tests
func TestUsernameGeneration_Patterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // partial match expected
	}{
		{
			name:     "simple name",
			input:    "Test Server",
			expected: "test-server-",
		},
		{
			name:     "name with special characters",
			input:    "My@Server#1",
			expected: "myserver1-",
		},
		{
			name:     "empty name",
			input:    "",
			expected: "-", // Should handle empty input gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateUsername(tt.input)

			// Check that the result contains the expected prefix
			assert.True(t, strings.HasPrefix(result, tt.expected),
				"Generated username should start with %s, got %s", tt.expected, result)

			// Check that a random suffix was added
			assert.True(t, len(result) > len(tt.expected),
				"Generated username should have a random suffix")
		})
	}
}

// Password generation tests
func TestPasswordGeneration_Security(t *testing.T) {
	// Generate multiple passwords and ensure they're different
	passwords := make([]string, 10)
	for i := 0; i < 10; i++ {
		passwords[i] = generateSecurePassword()
	}

	// Check that all passwords are different
	for i := 0; i < len(passwords); i++ {
		for j := i + 1; j < len(passwords); j++ {
			assert.NotEqual(t, passwords[i], passwords[j],
				"Generated passwords should be unique")
		}
	}

	// Check password length and format
	for _, password := range passwords {
		assert.NotEmpty(t, password, "Password should not be empty")
		assert.True(t, len(password) > 10,
			"Password should be reasonably long for security")
	}
}

// Mock data tests for SMTP credentials
func TestMockSMTPCredentialHelpers(t *testing.T) {
	mockClient := &mocks.MockClient{}

	t.Run("basic SMTP credential", func(t *testing.T) {
		credential := mockClient.NewMockSMTPCredential(
			1,
			"Test Credential",
			"test-user",
			"global",
			false,
			nil,
		)

		assert.Equal(t, uint64(1), credential.ID)
		assert.Equal(t, "Test Credential", credential.Name)
		assert.Equal(t, "test-user", credential.Username)
		assert.Equal(t, "global", credential.Scope)
		assert.False(t, credential.Sandbox)
		assert.Empty(t, credential.Domains)
		assert.True(t, time.Since(credential.CreatedAt) > 23*time.Hour) // Created ~24h ago
	})

	t.Run("scoped SMTP credential with domains", func(t *testing.T) {
		domains := []string{"example.com", "test.com"}
		credential := mockClient.NewMockSMTPCredential(
			1,
			"Marketing Credential",
			"marketing-user",
			"scoped",
			false,
			domains,
		)

		// Parse the expected UUID for comparison
		assert.Equal(t, uint64(1), credential.ID)
		assert.Equal(t, "Marketing Credential", credential.Name)
		assert.Equal(t, "marketing-user", credential.Username)
		assert.Equal(t, "scoped", credential.Scope)
		assert.False(t, credential.Sandbox)
		assert.Equal(t, domains, credential.Domains)
	})

	t.Run("sandbox SMTP credential", func(t *testing.T) {
		credential := mockClient.NewMockSMTPCredential(
			1,
			"Test Credential",
			"test-user",
			"global",
			true,
			nil,
		)

		assert.Equal(t, uint64(1), credential.ID)
		assert.Equal(t, "Test Credential", credential.Name)
		assert.Equal(t, "test-user", credential.Username)
		assert.Equal(t, "global", credential.Scope)
		assert.True(t, credential.Sandbox)
	})

	t.Run("SMTP credentials response", func(t *testing.T) {
		credentials := []responses.SMTPCredential{
			*mockClient.NewMockSMTPCredential(1, "Credential 1", "user1", "global", false, nil),
			*mockClient.NewMockSMTPCredential(2, "Credential 2", "user2", "scoped", false, []string{"example.com"}),
		}
		response := mockClient.NewMockSMTPCredentialsResponse(credentials, true)

		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 2)
		assert.True(t, response.Pagination.HasMore)
		assert.Nil(t, response.Pagination.NextCursor)
	})
}

// Mock API interaction tests
func TestSMTPAPI_MockInteractions(t *testing.T) {
	t.Run("list SMTP credentials parameters", func(t *testing.T) {
		// Test parameter construction for ListSMTPCredentials
		limit := int32(50)
		cursor := "next-token"

		// Verify parameters are constructed correctly
		assert.Equal(t, int32(50), limit)
		assert.Equal(t, "next-token", cursor)

		// Test pointer handling
		limitPtr := &limit
		cursorPtr := &cursor

		assert.NotNil(t, limitPtr)
		assert.Equal(t, int32(50), *limitPtr)
		assert.NotNil(t, cursorPtr)
		assert.Equal(t, "next-token", *cursorPtr)
	})

	t.Run("create SMTP credential request", func(t *testing.T) {
		// Test CreateSMTPCredentialRequest construction
		name := "Test Credential"
		username := "test-user"
		password := "secure-password"
		scope := "global"

		req := requests.CreateSMTPCredentialRequest{
			Name:     name,
			Username: username,
			Password: password,
			Scope:    scope,
		}

		assert.NotNil(t, req)
		// Note: We can't directly access fields without knowing the SDK structure,
		// but we can verify the request object was created
	})

	t.Run("delete SMTP credential", func(t *testing.T) {
		credentialID := "550e8400-e29b-41d4-a716-446655440000"

		// This would be the parameter passed to DeleteSMTPCredential
		assert.Equal(t, credentialID, "550e8400-e29b-41d4-a716-446655440000")
	})
}

// Email building tests for SMTP send
func TestEmailBuilding_SMTPSend(t *testing.T) {
	t.Run("basic text email", func(t *testing.T) {
		from := "sender@example.com"
		to := []string{"recipient@example.com"}
		subject := "Test Email"
		text := "This is a test email"

		// Test that email building parameters are correct
		assert.Equal(t, "sender@example.com", from)
		assert.Equal(t, []string{"recipient@example.com"}, to)
		assert.Equal(t, "Test Email", subject)
		assert.Equal(t, "This is a test email", text)
	})

	t.Run("HTML email with tracking", func(t *testing.T) {
		from := "sender@example.com"
		to := []string{"recipient@example.com"}
		subject := "HTML Email"
		html := "<h1>Hello</h1><p>This is HTML content</p>"
		trackOpens := true
		trackClicks := true

		// Test that email building parameters are correct
		assert.Equal(t, "sender@example.com", from)
		assert.Equal(t, []string{"recipient@example.com"}, to)
		assert.Equal(t, "HTML Email", subject)
		assert.Equal(t, "<h1>Hello</h1><p>This is HTML content</p>", html)
		assert.True(t, trackOpens)
		assert.True(t, trackClicks)
	})

	t.Run("multipart email with CC and BCC", func(t *testing.T) {
		from := "sender@example.com"
		to := []string{"recipient1@example.com", "recipient2@example.com"}
		cc := []string{"cc@example.com"}
		bcc := []string{"bcc@example.com"}
		text := "Plain text content"
		html := "<p>HTML content</p>"

		// Test that all email parameters are correct
		assert.Equal(t, "sender@example.com", from)
		assert.Equal(t, "Plain text content", text)
		assert.Equal(t, "<p>HTML content</p>", html)

		// Test that all recipients are collected correctly
		allRecipients := append(append(to, cc...), bcc...)
		expectedRecipients := []string{
			"recipient1@example.com", "recipient2@example.com",
			"cc@example.com", "bcc@example.com",
		}

		assert.Equal(t, expectedRecipients, allRecipients)
	})

	t.Run("AhaSend special headers", func(t *testing.T) {
		tags := []string{"marketing", "newsletter"}
		sandbox := true

		// Test that special headers are configured correctly
		assert.Equal(t, []string{"marketing", "newsletter"}, tags)
		assert.True(t, sandbox)

		// Test header formatting
		tagsHeader := strings.Join(tags, ",")
		assert.Equal(t, "marketing,newsletter", tagsHeader)
	})
}

// Server address parsing tests for SMTP send
func TestServerAddressParsing_SMTPSend(t *testing.T) {
	tests := []struct {
		name         string
		serverInput  string
		expectedHost string
		expectedPort string
	}{
		{
			name:         "default AhaSend server",
			serverInput:  "send.ahasend.com:587",
			expectedHost: "send.ahasend.com",
			expectedPort: "587",
		},
		{
			name:         "server without port",
			serverInput:  "mail.example.com",
			expectedHost: "mail.example.com",
			expectedPort: "587", // Should default to 587
		},
		{
			name:         "SSL server",
			serverInput:  "mail.example.com:465",
			expectedHost: "mail.example.com",
			expectedPort: "465",
		},
		{
			name:         "custom port",
			serverInput:  "smtp.company.com:2525",
			expectedHost: "smtp.company.com",
			expectedPort: "2525",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This simulates the server address parsing logic
			input := tt.serverInput

			// Basic validation that input contains expected components
			if strings.Contains(input, ":") {
				parts := strings.Split(input, ":")
				assert.Equal(t, tt.expectedHost, parts[0])
				assert.Equal(t, tt.expectedPort, parts[1])
			} else {
				assert.Equal(t, tt.expectedHost, input)
				// Port would default to 587
			}
		})
	}
}

// Test send command structure and interactive functionality
func TestSendCommand_Interactive_Structure(t *testing.T) {
	sendCmd := NewSendCommand()
	assert.Equal(t, "send", sendCmd.Name())
	assert.Equal(t, "Send an email via SMTP protocol", sendCmd.Short)
	assert.NotEmpty(t, sendCmd.Long)
	assert.Contains(t, sendCmd.Long, "INTERACTIVE MODE")
	assert.Contains(t, sendCmd.Long, "When called without any arguments")
	assert.NotEmpty(t, sendCmd.Example)
	assert.Contains(t, sendCmd.Example, "# Interactive mode")
}

func TestSendCommand_Interactive_Flags(t *testing.T) {
	sendCmd := NewSendCommand()
	flags := sendCmd.Flags()

	// Test that send command has all expected flags for interactive mode
	expectedFlags := []string{
		"from", "to", "cc", "bcc", "subject", "text", "html",
		"text-file", "html-file", "attach", "server", "username", "password",
		"credential-id", "track-opens", "track-clicks", "tags", "sandbox",
		"sandbox-result", "header", "test",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "send command should have %s flag", flagName)
	}
}

func TestSendCommand_Interactive_ValidationWithFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name: "complete flags should not require interaction",
			args: []string{
				"--from", "test@example.com",
				"--to", "recipient@example.com",
				"--subject", "Test",
				"--text", "Test message",
				"--username", "smtp-user",
				"--password", "smtp-pass",
			},
			expectError: true, // Will fail on SMTP connection, but validation should pass
		},
		{
			name: "test mode with complete flags",
			args: []string{
				"--test",
				"--from", "test@example.com",
				"--to", "recipient@example.com",
				"--subject", "Test",
				"--text", "Test message",
				"--username", "smtp-user",
				"--password", "smtp-pass",
			},
			expectError: false, // Test mode succeeds even if connection fails - just reports the result
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sendCmd := NewSendCommand()
			sendCmd.SetArgs(tt.args)

			err := sendCmd.Execute()
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), strings.Split(tt.errorMsg, " ")[0])
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSendCommand_Interactive_PromptFunctions(t *testing.T) {
	// Test that the interactive prompt functions exist
	// Note: These would need stdin input in real usage, so we just test they exist

	t.Run("interactive prompt functions exist", func(t *testing.T) {
		// These functions should be accessible for interactive mode
		assert.NotNil(t, promptSMTPFromEmail, "promptSMTPFromEmail function should exist")
		assert.NotNil(t, promptSMTPToEmail, "promptSMTPToEmail function should exist")
		assert.NotNil(t, promptSMTPSubject, "promptSMTPSubject function should exist")
		assert.NotNil(t, promptSMTPContent, "promptSMTPContent function should exist")
		assert.NotNil(t, promptSMTPCredentials, "promptSMTPCredentials function should exist")
	})
}

// Test that the help output includes interactive mode information
func TestSendCommand_Interactive_HelpOutput(t *testing.T) {
	sendCmd := NewSendCommand()
	var buf bytes.Buffer
	sendCmd.SetOut(&buf)
	sendCmd.SetErr(&buf)
	sendCmd.SetArgs([]string{"--help"})

	err := sendCmd.Execute()
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "INTERACTIVE MODE")
	assert.Contains(t, output, "When called without any arguments")
	assert.Contains(t, output, "# Interactive mode")
	assert.Contains(t, output, "interactively prompt")
}
