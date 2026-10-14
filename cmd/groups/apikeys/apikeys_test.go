package apikeys

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestListCommand_FlagDefaults(t *testing.T) {
	cmd := NewListCommand()

	// Parse flags with no arguments to get defaults
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err)

	limit, _ := cmd.Flags().GetInt32("limit")
	assert.Equal(t, int32(0), limit)

	after, _ := cmd.Flags().GetString("after")
	assert.Equal(t, "", after)
}

func TestGetCommand_RequiresKeyID(t *testing.T) {
	getCmd := NewGetCommand()

	// Test that command requires exactly one argument (key ID)
	getCmd.SetArgs([]string{})
	err := getCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argument is required")

	// Test with too many arguments
	getCmd.SetArgs([]string{"key1", "key2"})
	err = getCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected argument")
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

// TestCreateCommand_ReadsScopesAsRepeatableValues checks that --scope reads
// as every repeatable flag of the CLI does: given several times, split on
// commas, spaces around each item dropped, and a scope in double quotes kept
// whole.
func TestCreateCommand_ReadsScopesAsRepeatableValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"given twice", []string{"--scope", "messages:send:all", "--scope", "domains:read"},
			[]string{"messages:send:all", "domains:read"}},
		{"comma-separated", []string{"--scope", "messages:send:all, domains:read"},
			[]string{"messages:send:all", "domains:read"}},
		{"quoted", []string{"--scope", `"messages:send:{a.example.com}",domains:read`},
			[]string{"messages:send:{a.example.com}", "domains:read"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCreateCommand()
			require.NoError(t, cmd.ParseFlags(tt.args))

			scopes, err := cmdutil.ReadRepeatable(cmd, "scope", "")

			require.NoError(t, err)
			assert.Equal(t, tt.want, scopes)
		})
	}
}

func TestUpdateCommand_RequiresKeyID(t *testing.T) {
	updateCmd := NewUpdateCommand()

	// Test that command requires exactly one argument (key ID)
	updateCmd.SetArgs([]string{})
	err := updateCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argument is required")

	// Test with too many arguments
	updateCmd.SetArgs([]string{"key1", "key2"})
	err = updateCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected argument")
}

func TestDeleteCommand_RequiresKeyID(t *testing.T) {
	deleteCmd := NewDeleteCommand()

	// Test that command requires exactly one argument (key ID)
	deleteCmd.SetArgs([]string{})
	err := deleteCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argument is required")

	// Test with too many arguments
	deleteCmd.SetArgs([]string{"key1", "key2"})
	err = deleteCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected argument")
}

// Test scope validation using the shared validator
func TestAPIKey_ScopeValidation(t *testing.T) {
	validScopes := []string{
		// Legacy static scopes preserved through the extraction
		"messages:send:all",
		"messages:cancel:all",
		"messages:read:all",
		"domains:read",
		"domains:write",
		"domains:delete:all",
		"webhooks:read:all",
		"webhooks:write:all",
		"routes:read:all",
		"routes:write:all",
		"suppressions:read",
		"suppressions:write",
		"api-keys:read",
		"api-keys:write",
		"api-keys:delete",
		// Legacy dynamic domain scope
		"messages:send:{example.com}",
		// New sub-account scopes
		"sub-accounts:read",
		"sub-accounts:write",
		"sub-accounts:delete",
		"sub-accounts:suspend",
		"sub-accounts:usage",
		"sub-account-api-keys:read",
		"sub-account-api-keys:write",
		"sub-account-api-keys:delete",
		// Template, campaign, contact and list scopes
		"templates:read",
		"templates:write",
		"templates:delete",
		"campaigns:read",
		"campaigns:write",
		"campaigns:delete",
		"campaigns:send",
		"contacts:read",
		"contacts:write",
		"contacts:delete",
		"lists:read",
		"lists:write",
		"lists:delete",
	}

	invalidScopes := []string{
		"invalid",
		"messages:invalid",
		"",
		"write",          // too generic
		"domains:delete", // not a valid operation
		"sub-accounts:invalid",
	}

	for _, scope := range validScopes {
		t.Run("valid_"+scope, func(t *testing.T) {
			assert.NoError(t, validation.ValidateScope(scope, "ahasend apikeys create"), "scope %s should be valid", scope)
		})
	}

	for _, scope := range invalidScopes {
		t.Run("invalid_"+scope, func(t *testing.T) {
			assert.Error(t, validation.ValidateScope(scope, "ahasend apikeys create"), "scope %s should be invalid", scope)
		})
	}
}

func TestCreateCommand_PassesIdempotencyKey(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateAPIKey", mock.AnythingOfType("requests.CreateAPIKeyRequest"), testutil.IdempotencyKeyArg("retry-1")).
		Return(&responses.APIKey{}, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewCreateCommand(), "plain",
		"--label", "CI", "--scope", "messages:send:all", "--idempotency-key", "retry-1")

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_LeavesTheIdempotencyKeyToTheSDKWhenOmitted(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateAPIKey", mock.AnythingOfType("requests.CreateAPIKeyRequest")).Return(&responses.APIKey{}, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewCreateCommand(), "plain", "--label", "CI", "--scope", "messages:send:all")

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// IP allow list -----------------------------------------------------------------

const testAPIKeyID = mocks.FixtureAPIKeyID

// keyWithIPAllowList returns the fixture API key, usable from 203.0.113.0/24.
func keyWithIPAllowList() *responses.APIKey {
	key := mocks.NewAPIKey()
	key.IPAllowList = []string{"203.0.113.0/24"}
	return key
}

func TestCreateCommand_SendsIPAllowList(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateAPIKey", requests.CreateAPIKeyRequest{
		Label:       "Production servers",
		Scopes:      []string{"messages:send:all"},
		IPAllowList: []string{"203.0.113.0/24", "2001:db8::1"},
	}).Return(keyWithIPAllowList(), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewCreateCommand(), "plain",
		"--label", "Production servers", "--scope", "messages:send:all",
		"--ip-allow-list", "203.0.113.0/24", "--ip-allow-list", "2001:db8::1")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_OmittedIPAllowListIsNil(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateAPIKey", mock.MatchedBy(func(req requests.CreateAPIKeyRequest) bool {
		return req.IPAllowList == nil
	})).Return(keyWithIPAllowList(), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewCreateCommand(), "plain", "--label", "CI", "--scope", "messages:send:all")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateCommand_ReplacesIPAllowList(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateAPIKey", uuid.MustParse(testAPIKeyID),
		requests.UpdateAPIKeyRequest{IPAllowList: &[]string{"203.0.113.0/24"}}).
		Return(keyWithIPAllowList(), nil)

	out, err := testutil.RunWithClient(t, mockClient, NewUpdateCommand(), "plain", testAPIKeyID, "--ip-allow-list", "203.0.113.0/24")
	require.NoError(t, err)
	assert.Contains(t, out, "IP Allow List: 203.0.113.0/24")
	mockClient.AssertExpectations(t)
}

func TestUpdateCommand_ClearsIPAllowList(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateAPIKey", uuid.MustParse(testAPIKeyID),
		requests.UpdateAPIKeyRequest{IPAllowList: &[]string{}}).
		Return(keyWithIPAllowList(), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewUpdateCommand(), "plain", testAPIKeyID, "--clear-ip-allow-list")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateCommand_IPAllowListAndClearRejected(t *testing.T) {
	mockClient := &mocks.MockClient{}

	_, err := testutil.RunWithClient(t, mockClient, NewUpdateCommand(), "plain", testAPIKeyID,
		"--ip-allow-list", "203.0.113.0/24", "--clear-ip-allow-list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--ip-allow-list and --clear-ip-allow-list cannot be used together")
	mockClient.AssertNotCalled(t, "UpdateAPIKey", mock.Anything, mock.Anything)
}

func TestUpdateCommand_IPAllowListValidatedBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{testAPIKeyID, "--ip-allow-list", "203.0.113.0/24", "--clear-ip-allow-list"},
			"--ip-allow-list and --clear-ip-allow-list cannot be used together"},
		{[]string{testAPIKeyID, "--ip-allow-list", ""},
			"--ip-allow-list cannot be empty; use --clear-ip-allow-list to clear it"},
		{[]string{testAPIKeyID, "--ip-allow-list", "203.0.113.0/24,"},
			"--ip-allow-list cannot be empty; use --clear-ip-allow-list to clear it"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewUpdateCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

func TestCreateCommand_EmptyIPAllowListRejectedBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)
	_, err := testutil.RunPlain(t, NewCreateCommand(),
		"--label", "CI", "--scope", "messages:send:all", "--ip-allow-list", "")
	require.Error(t, err)
	assert.Equal(t, "--ip-allow-list cannot be empty", err.Error())
	assert.False(t, *authCalled)
}

func TestCreateCommand_IPAllowListSplitsOnCommas(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateAPIKey", requests.CreateAPIKeyRequest{
		Label:       "CI",
		Scopes:      []string{"messages:send:all"},
		IPAllowList: []string{"203.0.113.0/24", "2001:db8::1"},
	}).Return(keyWithIPAllowList(), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewCreateCommand(), "plain",
		"--label", "CI", "--scope", "messages:send:all", "--ip-allow-list", "203.0.113.0/24,2001:db8::1")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
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

func BenchmarkScope_Validation(b *testing.B) {
	testScope := "messages:send:all"
	for i := 0; i < b.N; i++ {
		_ = validation.ValidateScope(testScope, "ahasend apikeys create")
	}
}

func TestCommands_CSVIncludesIPAllowList(t *testing.T) {
	key := keyWithIPAllowList()
	tests := []struct {
		name  string
		setup func(*mocks.MockClient)
		cmd   func() *cobra.Command
		args  []string
	}{
		{"list", func(m *mocks.MockClient) {
			m.On("ListAPIKeys", common.PaginationParams{}).
				Return(&responses.PaginatedAPIKeysResponse{Data: []responses.APIKey{*key}}, nil)
		}, NewListCommand, nil},
		{"get", func(m *mocks.MockClient) {
			m.On("GetAPIKey", uuid.MustParse(testAPIKeyID)).Return(key, nil)
		}, NewGetCommand, []string{testAPIKeyID}},
		{"create", func(m *mocks.MockClient) {
			m.On("CreateAPIKey", mock.AnythingOfType("requests.CreateAPIKeyRequest")).Return(key, nil)
		}, NewCreateCommand, []string{"--label", "CI", "--scope", "messages:send:all"}},
		{"update", func(m *mocks.MockClient) {
			m.On("UpdateAPIKey", uuid.MustParse(testAPIKeyID), mock.AnythingOfType("requests.UpdateAPIKeyRequest")).Return(key, nil)
		}, NewUpdateCommand, []string{testAPIKeyID, "--label", "CI"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			tt.setup(mockClient)

			out, err := testutil.RunWithClient(t, mockClient, tt.cmd(), "csv", tt.args...)
			require.NoError(t, err)
			header := strings.SplitN(out, "\n", 2)[0]
			assert.Contains(t, strings.Split(header, ","), "ip_allow_list")
			assert.Contains(t, out, "203.0.113.0/24")
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDeleteDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", "11111111-1111-1111-1111-111111111111")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete API key 11111111-1111-1111-1111-111111111111? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedDeletesAPIKey(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteAPIKey", uuid.MustParse("11111111-1111-1111-1111-111111111111")).Return(&common.SuccessResponse{Message: "deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	assert.Equal(t, "deleted\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestCreateRequiresLabelAndScopeBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--scope", "messages:send:all"}, "--label is required"},
		{[]string{"--label", "CI"}, "--scope is required"},
		{[]string{"--label", "CI", "--scope", ""}, "--scope cannot be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			stdout, _, err := testutil.RunCommand(t, NewCreateCommand(), "json", "", tt.args...)
			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.Empty(t, stdout)
			assert.False(t, *authCalled)
		})
	}
}

func TestCreateRequiredFlagsSayRequiredInHelp(t *testing.T) {
	cmd := NewCreateCommand()
	assert.True(t, strings.HasSuffix(cmd.Flags().Lookup("label").Usage, "(required)"))
	assert.True(t, strings.HasSuffix(cmd.Flags().Lookup("scope").Usage, "(required; can be used multiple times)"))
}

func TestUpdateRejectsMalformedKeyIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewUpdateCommand(), "plain", "", "bad-id", "--label", "x")
	require.Error(t, err)
	assert.Equal(t, `invalid API key ID value "bad-id": not a UUID`, err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *authCalled)
}

func TestUpdateRejectsOverlongLabelBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewUpdateCommand(), "plain", "",
		"fcb3f3bc-4ac8-4330-948d-1671fcf9a768", "--label", strings.Repeat("a", 256))
	require.Error(t, err)
	assert.Equal(t, "--label must be at most 255 characters", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *authCalled)
}

func TestGetRejectsMalformedKeyIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewGetCommand(), "plain", "", "bad-id")
	require.Error(t, err)
	assert.Equal(t, `invalid API key ID value "bad-id": not a UUID`, err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *authCalled)
}
