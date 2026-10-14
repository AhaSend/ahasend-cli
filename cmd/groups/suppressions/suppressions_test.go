package suppressions

import (
	"bytes"
	"strings"
	"testing"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
				"List suppressed email addresses, with cursor pagination",
				"--limit",
				"--after",
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
				"Create a suppression entry to prevent sending emails",
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
				"Delete the suppressions of an email address",
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

func TestCreateRejectsInvalidEmailBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "", "userexample.com", "--expires", "30d")
	require.Error(t, err)
	assert.Equal(t, `invalid email value "userexample.com": not an email address`, err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *authCalled)
}

func TestEmailCommandsRejectInvalidEmailBeforeAuthentication(t *testing.T) {
	tests := map[string]struct {
		cmd  *cobra.Command
		args []string
		name string
	}{
		"check":        {NewCheckCommand(), []string{"userexample.com"}, "email"},
		"delete":       {NewDeleteCommand(), []string{"userexample.com", "--force"}, "email"},
		"list --email": {NewListCommand(), []string{"--email", "userexample.com"}, "--email"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			_, _, err := testutil.RunCommand(t, test.cmd, "plain", "", test.args...)
			require.Error(t, err)
			assert.Equal(t, `invalid `+test.name+` value "userexample.com": not an email address`, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *authCalled)
		})
	}
}

func TestListSendsOnlyTheFiltersGiven(t *testing.T) {
	domain := "example.com"
	tests := map[string]struct {
		args []string
		want *string
	}{
		"without --domain": {nil, nil},
		"with --domain":    {[]string{"--domain", domain}, &domain},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			var sent requests.GetSuppressionsParams
			mockClient.On("ListSuppressions", mock.Anything).Run(func(args mock.Arguments) {
				sent = args.Get(0).(requests.GetSuppressionsParams)
			}).Return(&responses.PaginatedSuppressionsResponse{}, nil)

			_, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "plain", test.args...)
			require.NoError(t, err)
			assert.Equal(t, test.want, sent.Domain)
			assert.Nil(t, sent.Email)
		})
	}
}

func TestCreateRejectsLongReasonBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "", "user@example.com", "--expires", "30d",
		"--reason", strings.Repeat("r", 256))
	require.Error(t, err)
	assert.Equal(t, "--reason must be at most 255 characters", err.Error())
	assert.False(t, *authCalled)
}

func TestCreateCountsReasonCharactersNotBytes(t *testing.T) {
	reason := strings.Repeat("é", 255)
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSuppression", mock.MatchedBy(func(req requests.CreateSuppressionRequest) bool {
		return req.Reason != nil && *req.Reason == reason
	})).Return(&responses.CreateSuppressionResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "", "user@example.com", "--expires", "30d", "--reason", reason)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)

	_, _, err = testutil.RunCommand(t, NewCreateCommand(), "plain", "", "user@example.com", "--expires", "30d", "--reason", reason+"é")
	require.Error(t, err)
	assert.Equal(t, "--reason must be at most 255 characters", err.Error())
}

func TestDeleteDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", "user@example.com", "--domain", "example.com")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete the suppression of user@example.com for domain example.com? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedDeletesSuppression(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSuppression", "user@example.com", (*string)(nil)).Return(&common.SuccessResponse{Message: "deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "user@example.com")
	require.NoError(t, err)
	assert.Equal(t, "deleted\n", stdout)
	assert.Contains(t, stderr, "Are you sure you want to delete every suppression of user@example.com, on every domain, "+
		"including unsubscribes and complaints? (y/N): ", "without --domain, the prompt says what goes")
	mockClient.AssertExpectations(t)
}

func TestWipeDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewWipeCommand(), "plain", "n\n")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to wipe all suppressions? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
	assert.Contains(t, stderr, suppressionBackupHint)
}

// TestWipeBackupHintPagesThroughEverySuppression checks that the backup hint
// of the prompt and the help pages with --after, since one list call saves
// one page only.
func TestWipeBackupHintPagesThroughEverySuppression(t *testing.T) {
	assert.Contains(t, suppressionBackupHint, "ahasend suppressions list --limit 100 --output json")
	assert.Contains(t, suppressionBackupHint, "--after")
	assert.Contains(t, NewWipeCommand().Long, suppressionBackupHint)
}

func TestWipeAcceptedWipesSuppressions(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("WipeSuppressions", (*string)(nil)).Return(&common.SuccessResponse{Message: "deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewWipeCommand(), "plain", "y\n")
	require.NoError(t, err)
	assert.Equal(t, "deleted\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestCreateRequiresExpiresBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "", "user@example.com")
	require.Error(t, err)
	assert.Equal(t, "--expires is required", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *authCalled)
}

func TestCreateRejectsBadExpiresBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "", "user@example.com", "--expires", "soon")
	require.Error(t, err)
	assert.Equal(t, `invalid --expires value "soon": use RFC 3339 or a relative time such as 30d or 1y`, err.Error())
	assert.False(t, *authCalled)
}

func TestCreateExpiresHelpSaysRequired(t *testing.T) {
	assert.True(t, strings.HasSuffix(NewCreateCommand().Flags().Lookup("expires").Usage, "(required)"))
}

// TestCommandsRejectEmptyDomainBeforePrompt checks that an empty --domain
// fails instead of acting on, or reading, every domain of the account.
func TestCommandsRejectEmptyDomainBeforePrompt(t *testing.T) {
	tests := map[string]struct {
		cmd  *cobra.Command
		args []string
	}{
		"wipe":   {NewWipeCommand(), []string{"--domain", ""}},
		"delete": {NewDeleteCommand(), []string{"user@example.com", "--domain", ""}},
		"create": {NewCreateCommand(), []string{"user@example.com", "--domain", "", "--expires", "30d"}},
		"check":  {NewCheckCommand(), []string{"user@example.com", "--domain", ""}},
		"list":   {NewListCommand(), []string{"--domain", ""}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			_, stderr, err := testutil.RunCommand(t, test.cmd, "plain", "y\n", test.args...)
			require.Error(t, err)
			assert.Equal(t, "--domain cannot be empty", err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.NotContains(t, stderr, "Are you sure")
			assert.False(t, *authCalled)
		})
	}
}

// TestDeleteAcceptsEveryValidEmail checks that a suppression of an address
// with an apostrophe or non-ASCII characters can be deleted.
func TestDeleteAcceptsEveryValidEmail(t *testing.T) {
	for _, email := range []string{"o'brien@example.com", "jörg@exämple.com"} {
		t.Run(email, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("DeleteSuppression", email, (*string)(nil)).Return(&common.SuccessResponse{Message: "deleted"}, nil)

			_, err := testutil.RunWithClient(t, mockClient, NewDeleteCommand(), "plain", email, "--force")
			require.NoError(t, err)
			mockClient.AssertExpectations(t)
		})
	}
}
