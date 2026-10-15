package smtp

import (
	"bytes"
	"strings"
	"testing"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
				"List the SMTP credentials in your account",
				"--limit",
				"--after",
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
				"Create an SMTP credential",
				"--name",
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

// TestParseServerSplitsHostAndPort checks parseServer, which a server
// without a port gives port 587.
func TestParseServerSplitsHostAndPort(t *testing.T) {
	tests := map[string]struct {
		host string
		port int
	}{
		"send.ahasend.com:587":  {"send.ahasend.com", 587},
		"mail.example.com":      {"mail.example.com", 587},
		"mail.example.com:2525": {"mail.example.com", 2525},
		"127.0.0.1:25":          {"127.0.0.1", 25},
		"[::1]:465":             {"::1", 465},
	}
	for server, want := range tests {
		t.Run(server, func(t *testing.T) {
			host, port, err := parseServer(server)
			require.NoError(t, err)
			assert.Equal(t, want.host, host)
			assert.Equal(t, want.port, port)
		})
	}
}

func TestParseServerRejectsAServerItCannotDial(t *testing.T) {
	const badPort = "the port must be a number from 1 to 65535"
	tests := map[string]string{
		"":                       "--server cannot be empty",
		":587":                   `invalid --server value ":587": no host; use a host and port such as send.ahasend.com:587`,
		"mail.example.com:smtp":  `invalid --server value "mail.example.com:smtp": ` + badPort,
		"mail.example.com:0":     `invalid --server value "mail.example.com:0": ` + badPort,
		"mail.example.com:99999": `invalid --server value "mail.example.com:99999": ` + badPort,
		"mail.example.com:":      `invalid --server value "mail.example.com:": ` + badPort,
	}
	for server, message := range tests {
		t.Run(server, func(t *testing.T) {
			_, _, err := parseServer(server)
			require.Error(t, err)
			assert.Equal(t, message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
		})
	}
}

// TestSendServerDefaultsToAhaSendOnPort587 checks the --server default
// without connecting to it.
func TestSendServerDefaultsToAhaSendOnPort587(t *testing.T) {
	server := NewSendCommand().Flags().Lookup("server")
	require.NotNil(t, server)
	assert.Equal(t, "send.ahasend.com:587", server.DefValue)

	host, port, err := parseServer(server.DefValue)
	require.NoError(t, err)
	assert.Equal(t, "send.ahasend.com", host)
	assert.Equal(t, 587, port)
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
				"--server", "127.0.0.1:1",
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
				"--server", "127.0.0.1:1",
			},
			expectError: true, // Test mode reports the failed connection and exits nonzero
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
	assert.Contains(t, output, "the command asks for it;\n--non-interactive never asks")
	assert.Contains(t, output, "--non-interactive")
}

func TestDeleteDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", "11111111-1111-1111-1111-111111111111")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete SMTP credential 11111111-1111-1111-1111-111111111111? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedDeletesWithoutFetchingTheCredential(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSMTPCredential", uuid.MustParse("11111111-1111-1111-1111-111111111111")).Return(&common.SuccessResponse{Message: "deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	assert.Equal(t, "deleted\n", stdout)
	mockClient.AssertExpectations(t)
	mockClient.AssertNotCalled(t, "GetSMTPCredential", mock.Anything)
}

func TestDeleteForceShorthandSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSMTPCredential", uuid.MustParse("11111111-1111-1111-1111-111111111111")).Return(&common.SuccessResponse{Message: "deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "", "11111111-1111-1111-1111-111111111111", "-f")
	require.NoError(t, err)
	assert.Equal(t, "deleted\n", stdout)
	mockClient.AssertExpectations(t)
}
