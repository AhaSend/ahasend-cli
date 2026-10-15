package apikeys

import (
	"bytes"
	"os"
	"strings"
	"testing"

	topapikeys "github.com/AhaSend/ahasend-cli/cmd/groups/apikeys"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testSubAccountID = "11111111-1111-1111-1111-111111111111"
	testKeyID        = "44444444-4444-4444-4444-444444444444"
)

func newTestAPIKeyList() *responses.PaginatedAPIKeysResponse {
	return &responses.PaginatedAPIKeysResponse{
		Object: "list",
		Data:   []responses.APIKey{*mocks.NewAPIKey()},
	}
}

// Group structure -----------------------------------------------------------

// The alias `apikeys` resolves to the same subgroup as `api-keys`, including
// when used as `subaccounts apikeys list`.
func TestAPIKeysCommand_AliasResolves(t *testing.T) {
	parent := &cobra.Command{Use: "subaccounts"}
	parent.AddCommand(NewCommand())

	found, _, err := parent.Find([]string{"apikeys", "list"})
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "list", found.Name())
	assert.Equal(t, "api-keys", found.Parent().Name())
}

// list -----------------------------------------------------------------------

func TestListCommand_RequiresExactlyOneArg(t *testing.T) {
	_, err := testutil.RunCommandCombined(t, NewListCommand(), "plain")
	require.Error(t, err)
	assert.Equal(t, "the sub-account-id argument is required", err.Error())
}

func TestListCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewListCommand(), "plain", "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
}

func TestListCommand_LimitZeroAccepted_NilPagination(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccountAPIKeys", uuid.MustParse(testSubAccountID), common.PaginationParams{}).
		Return(newTestAPIKeyList(), nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewListCommand(), "plain", testSubAccountID, "--limit=0")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Production servers")
	mockClient.AssertExpectations(t)
}

func TestListCommand_LimitMaxAccepted(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccountAPIKeys", uuid.MustParse(testSubAccountID),
		mock.MatchedBy(func(p common.PaginationParams) bool { return p.Limit != nil && *p.Limit == 100 })).
		Return(&responses.PaginatedAPIKeysResponse{Object: "list"}, nil)
	called := testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewListCommand(), "plain", testSubAccountID, "--limit=100")
	require.NoError(t, err)
	assert.True(t, *called)
	mockClient.AssertExpectations(t)
}

// A non-nil list response flows through the shared API-key renderer so JSON
// output stays a verbatim SDK PaginatedAPIKeysResponse.
func TestListCommand_JSONVerbatimPassThrough(t *testing.T) {
	nextCursor := "next-page"
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccountAPIKeys", uuid.MustParse(testSubAccountID), common.PaginationParams{}).
		Return(&responses.PaginatedAPIKeysResponse{
			Object: "list",
			Data:   []responses.APIKey{*mocks.NewAPIKey()},
			Pagination: common.PaginationInfo{
				HasMore:    true,
				NextCursor: &nextCursor,
			},
		}, nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewListCommand(), "json", testSubAccountID)
	require.NoError(t, err)
	assert.True(t, *called)

	assert.Contains(t, out, `"object": "list"`)
	assert.Contains(t, out, "next-page")
	assert.Contains(t, out, "aha-pk-test")
	assert.NotContains(t, out, `"empty"`)
	mockClient.AssertExpectations(t)
}

// get ------------------------------------------------------------------------

func TestGetCommand_RequiresExactlyTwoArgs(t *testing.T) {
	_, err := testutil.RunCommandCombined(t, NewGetCommand(), "plain", testSubAccountID)
	require.Error(t, err)
	assert.Equal(t, "the key-id argument is required", err.Error())
}

func TestGetCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewGetCommand(), "plain", "not-a-uuid", testKeyID)
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
}

func TestGetCommand_BadKeyIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewGetCommand(), "plain", testSubAccountID, "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid API key ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed API key ID")
}

func TestGetCommand_MockBacked(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetSubAccountAPIKey", uuid.MustParse(testSubAccountID), uuid.MustParse(testKeyID)).Return(mocks.NewAPIKey(), nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewGetCommand(), "plain", testSubAccountID, testKeyID)
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Production servers")
	mockClient.AssertExpectations(t)
}

func TestGetCommand_JSONVerbatimPassThrough(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetSubAccountAPIKey", uuid.MustParse(testSubAccountID), uuid.MustParse(testKeyID)).Return(mocks.NewAPIKey(), nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewGetCommand(), "json", testSubAccountID, testKeyID)
	require.NoError(t, err)
	assert.True(t, *called)

	assert.Contains(t, out, `"object": "api_key"`)
	assert.Contains(t, out, "aha-pk-test")
	assert.Contains(t, out, testKeyID)
	mockClient.AssertExpectations(t)
}

func newTestAPIKeyWithSecret() *responses.APIKey {
	key := mocks.NewAPIKey()
	secret := "sk_one_time_secret_value"
	key.SecretKey = &secret
	return key
}

const validScope = "messages:send:all"

// create ----------------------------------------------------------------------

// The nested create help promises the idempotency flag and the 5-minute replay
// window; the top-level apikeys create takes the same flag.
func TestCreateCommand_HelpMentionsIdempotencyAndReplayWindow(t *testing.T) {
	out, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "--idempotency-key")
	assert.Contains(t, out, "5-minute replay window")

	topOut, err := testutil.RunCommandCombined(t, topapikeys.NewCreateCommand(), "plain", "--help")
	require.NoError(t, err)
	assert.Contains(t, topOut, "--idempotency-key")
}

func TestCreateCommand_RequiresLabelAndScope(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{testSubAccountID, "--scope", "messages:send:all"}, "--label is required"},
		{[]string{testSubAccountID, "--label", "CI"}, "--scope is required"},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", tt.args...)
			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *called, "auth resolver must not be reached when required flags are missing")
		})
	}
}

func TestCreateCommand_RequiredFlagsSayRequiredInHelp(t *testing.T) {
	cmd := NewCreateCommand()
	assert.True(t, strings.HasSuffix(cmd.Flags().Lookup("label").Usage, "(required)"))
	assert.True(t, strings.HasSuffix(cmd.Flags().Lookup("scope").Usage, "(required; can be used multiple times)"))
}

func TestCreateCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", "not-a-uuid",
		"--label", "CI", "--scope", validScope)
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
}

func TestCreateCommand_InvalidScopeFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", testSubAccountID,
		"--label", "CI", "--scope", "not-a-real-scope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --scope value "not-a-real-scope": not an API key scope`)
	assert.False(t, *called, "auth resolver must not be reached for an invalid scope")
}

// When --idempotency-key is provided it is passed through verbatim.
func TestCreateCommand_IdempotencyKeyPassThrough(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
		requests.CreateAPIKeyRequest{Label: "CI", Scopes: []string{validScope}},
		testutil.IdempotencyKeyArg("my-key")).
		Return(mocks.NewAPIKey(), nil)
	called := testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", testSubAccountID,
		"--label", "CI", "--scope", validScope, "--idempotency-key", "my-key")
	require.NoError(t, err)
	assert.True(t, *called)
	mockClient.AssertExpectations(t)
}

// When --idempotency-key is omitted no key option is passed, so the SDK
// generates one.
func TestCreateCommand_IdempotencyKeyLeftToTheSDK(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
		requests.CreateAPIKeyRequest{Label: "CI", Scopes: []string{validScope}}).
		Return(mocks.NewAPIKey(), nil)
	called := testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", testSubAccountID,
		"--label", "CI", "--scope", validScope)
	require.NoError(t, err)
	assert.True(t, *called)
	mockClient.AssertExpectations(t)
}

// The created APIKey flows through HandleCreateAPIKey so JSON preserves the
// one-time secret_key verbatim.
func TestCreateCommand_JSONPreservesSecretKey(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
		mock.AnythingOfType("requests.CreateAPIKeyRequest")).
		Return(newTestAPIKeyWithSecret(), nil)
	testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewCreateCommand(), "json", testSubAccountID,
		"--label", "CI", "--scope", validScope)
	require.NoError(t, err)
	assert.Contains(t, out, `"secret_key"`)
	assert.Contains(t, out, "sk_one_time_secret_value")
	// JSON output must not carry the human-readable replay note.
	assert.NotContains(t, out, "replay window")
	mockClient.AssertExpectations(t)
}

// Table and plain output include the replay note when secret_key is present.
func TestCreateCommand_ReplayNotePrintedForHumanFormats(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
				mock.AnythingOfType("requests.CreateAPIKeyRequest")).
				Return(newTestAPIKeyWithSecret(), nil)
			testutil.InjectClient(t, mockClient)

			out, err := testutil.RunCommandCombined(t, NewCreateCommand(), format, testSubAccountID,
				"--label", "CI", "--scope", validScope)
			require.NoError(t, err)
			assert.Contains(t, out, "5-minute replay window")
			mockClient.AssertExpectations(t)
		})
	}
}

// TestCreateCommand_PrintsTheSameAsTopLevelCreate checks that the sub-account
// create prints one secret notice, the one the top-level create prints, so
// the two commands say the same thing.
func TestCreateCommand_PrintsTheSameAsTopLevelCreate(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
				mock.AnythingOfType("requests.CreateAPIKeyRequest")).
				Return(newTestAPIKeyWithSecret(), nil)
			mockClient.On("CreateAPIKey", mock.AnythingOfType("requests.CreateAPIKeyRequest")).
				Return(newTestAPIKeyWithSecret(), nil)
			testutil.InjectClient(t, mockClient)

			nested, _, err := testutil.RunCommand(t, NewCreateCommand(), format, "", testSubAccountID,
				"--label", "CI", "--scope", validScope)
			require.NoError(t, err)
			top, _, err := testutil.RunCommand(t, topapikeys.NewCreateCommand(), format, "",
				"--label", "CI", "--scope", validScope)
			require.NoError(t, err)

			assert.Equal(t, top, nested)
			assert.Equal(t, 1, strings.Count(nested, "replay window"))
		})
	}
}

// The note is suppressed when the secret is absent, even for human formats.
func TestCreateCommand_NoReplayNoteWhenSecretMissing(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
		mock.AnythingOfType("requests.CreateAPIKeyRequest")).
		Return(mocks.NewAPIKey(), nil) // no SecretKey
	testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", testSubAccountID,
		"--label", "CI", "--scope", validScope)
	require.NoError(t, err)
	assert.NotContains(t, out, "replay window")
	mockClient.AssertExpectations(t)
}

// The note is not printed for CSV output.
func TestCreateCommand_NoReplayNoteForCSV(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
		mock.AnythingOfType("requests.CreateAPIKeyRequest")).
		Return(newTestAPIKeyWithSecret(), nil)
	testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewCreateCommand(), "csv", testSubAccountID,
		"--label", "CI", "--scope", validScope)
	require.NoError(t, err)
	assert.NotContains(t, out, "replay window")
	mockClient.AssertExpectations(t)
}

// Raw 409/422 SDK API errors are returned verbatim so the root error handler
// prints the API body in JSON mode and keeps the exit code at 0.
func TestCreateCommand_RawAPIErrorJSONPassThrough(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		raw        string
	}{
		{"conflict", 409, `{"error":"idempotency_conflict","message":"idempotency key is already in use"}`},
		{"unprocessable", 422, `{"error":"validation_failed","message":"request body is invalid"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			apiErr := &api.APIError{
				StatusCode: tc.statusCode,
				Message:    "api request failed",
				Code:       "api_error",
				Raw:        []byte(tc.raw),
			}
			mockClient := &mocks.MockClient{}
			mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
				mock.AnythingOfType("requests.CreateAPIKeyRequest")).
				Return(nil, apiErr)
			testutil.InjectClient(t, mockClient)

			_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "json", testSubAccountID,
				"--label", "CI", "--scope", validScope)

			// The runner returns the SDK error verbatim with its raw body.
			returnedAPIErr, ok := err.(*api.APIError)
			require.True(t, ok, "runner must return the raw SDK *api.APIError")
			assert.Equal(t, tc.statusCode, returnedAPIErr.StatusCode)
			assert.JSONEq(t, tc.raw, string(returnedAPIErr.Raw))

			// The JSON handler prints the raw body and still reports the failure.
			var buf bytes.Buffer
			handler := testutil.ResponseHandler(t, "json", &buf, os.Stderr)
			assert.Equal(t, err, handler.HandleError(err))
			assert.JSONEq(t, tc.raw, buf.String())

			mockClient.AssertExpectations(t)
		})
	}
}

// update ----------------------------------------------------------------------

func TestUpdateCommand_RequiresExactlyTwoArgs(t *testing.T) {
	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", testSubAccountID)
	require.Error(t, err)
	assert.Equal(t, "the key-id argument is required", err.Error())
}

func TestUpdateCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", "not-a-uuid", testKeyID, "--label", "New")
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
}

func TestUpdateCommand_BadKeyIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", testSubAccountID, "not-a-uuid", "--label", "New")
	require.Error(t, err)
	assert.Equal(t, `invalid API key ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed API key ID")
}

func TestUpdateCommand_NoChangedFlagsFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", testSubAccountID, testKeyID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of --label, --scope, --ip-allow-list or --clear-ip-allow-list must be provided")
	assert.False(t, *called, "auth resolver must not be reached when no changed flags are provided")
}

func TestUpdateCommand_MockBacked(t *testing.T) {
	label := "New Label"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateSubAccountAPIKey", uuid.MustParse(testSubAccountID), uuid.MustParse(testKeyID),
		requests.UpdateAPIKeyRequest{Label: &label}).
		Return(mocks.NewAPIKey(), nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", testSubAccountID, testKeyID, "--label", label)
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Production servers")
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_SendsIPAllowList(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
		requests.CreateAPIKeyRequest{
			Label:       "CI",
			Scopes:      []string{validScope},
			IPAllowList: []string{"203.0.113.0/24", "2001:db8::1"},
		}).
		Return(newTestAPIKeyWithSecret(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", testSubAccountID, "--label", "CI", "--scope", validScope,
		"--ip-allow-list", "203.0.113.0/24", "--ip-allow-list", "2001:db8::1")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_OmittedIPAllowListIsNil(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
		mock.MatchedBy(func(req requests.CreateAPIKeyRequest) bool { return req.IPAllowList == nil })).
		Return(newTestAPIKeyWithSecret(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", testSubAccountID, "--label", "CI", "--scope", validScope)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateCommand_ReplacesIPAllowList(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateSubAccountAPIKey", uuid.MustParse(testSubAccountID), uuid.MustParse(testKeyID),
		requests.UpdateAPIKeyRequest{IPAllowList: &[]string{"203.0.113.0/24"}}).
		Return(mocks.NewAPIKey(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", testSubAccountID, testKeyID, "--ip-allow-list", "203.0.113.0/24")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateCommand_ClearsIPAllowList(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateSubAccountAPIKey", uuid.MustParse(testSubAccountID), uuid.MustParse(testKeyID),
		requests.UpdateAPIKeyRequest{IPAllowList: &[]string{}}).
		Return(mocks.NewAPIKey(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", testSubAccountID, testKeyID, "--clear-ip-allow-list")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateCommand_IPAllowListAndClearFailBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", testSubAccountID, testKeyID,
		"--ip-allow-list", "203.0.113.0/24", "--clear-ip-allow-list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--ip-allow-list and --clear-ip-allow-list cannot be used together")
	assert.False(t, *called, "auth resolver must not be reached when validation fails")
}

func TestEmptyIPAllowListFailsBeforeAuth(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *cobra.Command
		args    []string
		message string
	}{
		{"create", NewCreateCommand(), []string{testSubAccountID, "--label", "CI", "--scope", validScope, "--ip-allow-list", ""},
			"--ip-allow-list cannot be empty"},
		{"update", NewUpdateCommand(), []string{testSubAccountID, testKeyID, "--ip-allow-list", "203.0.113.0/24,,2001:db8::1"},
			"--ip-allow-list cannot be empty; use --clear-ip-allow-list to clear it"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, err := testutil.RunCommandCombined(t, test.cmd, "plain", test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *called, "auth resolver must not be reached when validation fails")
		})
	}
}

// delete ----------------------------------------------------------------------

func TestDeleteCommand_RequiresExactlyTwoArgs(t *testing.T) {
	_, err := testutil.RunCommandCombined(t, NewDeleteCommand(), "plain", testSubAccountID)
	require.Error(t, err)
	assert.Equal(t, "the key-id argument is required", err.Error())
}

func TestDeleteCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewDeleteCommand(), "plain", "not-a-uuid", testKeyID, "--force")
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
}

func TestDeleteCommand_BadKeyIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewDeleteCommand(), "plain", testSubAccountID, "not-a-uuid", "--force")
	require.Error(t, err)
	assert.Equal(t, `invalid API key ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed API key ID")
}

func TestDeleteCommand_ForceMockBacked(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSubAccountAPIKey", uuid.MustParse(testSubAccountID), uuid.MustParse(testKeyID)).
		Return(&common.SuccessResponse{}, nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewDeleteCommand(), "plain", testSubAccountID, testKeyID, "--force")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Equal(t, "API key '"+testKeyID+"' deleted successfully\n", out)
	mockClient.AssertExpectations(t)
}

func TestCommands_CSVIncludesIPAllowList(t *testing.T) {
	key := mocks.NewAPIKey()
	key.IPAllowList = []string{"203.0.113.0/24"}
	tests := []struct {
		name  string
		setup func(*mocks.MockClient)
		cmd   func() *cobra.Command
		args  []string
	}{
		{"list", func(m *mocks.MockClient) {
			m.On("ListSubAccountAPIKeys", uuid.MustParse(testSubAccountID), common.PaginationParams{}).
				Return(&responses.PaginatedAPIKeysResponse{Data: []responses.APIKey{*key}}, nil)
		}, NewListCommand, []string{testSubAccountID}},
		{"get", func(m *mocks.MockClient) {
			m.On("GetSubAccountAPIKey", uuid.MustParse(testSubAccountID), uuid.MustParse(testKeyID)).Return(key, nil)
		}, NewGetCommand, []string{testSubAccountID, testKeyID}},
		{"create", func(m *mocks.MockClient) {
			m.On("CreateSubAccountAPIKey", uuid.MustParse(testSubAccountID),
				mock.AnythingOfType("requests.CreateAPIKeyRequest")).Return(key, nil)
		}, NewCreateCommand, []string{testSubAccountID, "--label", "CI", "--scope", validScope}},
		{"update", func(m *mocks.MockClient) {
			m.On("UpdateSubAccountAPIKey", uuid.MustParse(testSubAccountID), uuid.MustParse(testKeyID),
				mock.AnythingOfType("requests.UpdateAPIKeyRequest")).Return(key, nil)
		}, NewUpdateCommand, []string{testSubAccountID, testKeyID, "--label", "CI"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			tt.setup(mockClient)
			testutil.InjectClient(t, mockClient)

			out, err := testutil.RunCommandCombined(t, tt.cmd(), "csv", tt.args...)
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

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete API key 22222222-2222-2222-2222-222222222222 of sub-account 11111111-1111-1111-1111-111111111111? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}
