package apikeys

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	topapikeys "github.com/AhaSend/ahasend-cli/cmd/groups/apikeys"
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/printer"
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

// execCommand runs a leaf command with a plain response handler installed in
// context and captures its output. It mirrors how the root command wires the
// handler before a runner executes.
func execCommand(cmd *cobra.Command, args ...string) (string, error) {
	var buf bytes.Buffer
	handler := printer.GetResponseHandler("plain", false, &buf)
	ctx := context.WithValue(context.Background(), printer.ResponseHandlerKey, handler)
	cmd.SetContext(ctx)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// execCommandJSON mirrors execCommand but installs a JSON response handler so
// tests can assert the SDK pass-through contract for machine-readable output.
func execCommandJSON(cmd *cobra.Command, args ...string) (string, error) {
	var buf bytes.Buffer
	handler := printer.GetResponseHandler("json", false, &buf)
	ctx := context.WithValue(context.Background(), printer.ResponseHandlerKey, handler)
	cmd.SetContext(ctx)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// trackingResolver installs a test auth resolver that records whether it was
// invoked and returns the supplied client. The returned *bool lets tests assert
// validation-before-auth ordering.
func trackingResolver(t *testing.T, c client.AhaSendClient) *bool {
	t.Helper()
	called := false
	restore := auth.SetAuthenticatedClientResolverForTesting(func(*cobra.Command) (client.AhaSendClient, error) {
		called = true
		return c, nil
	})
	t.Cleanup(restore)
	return &called
}

func newTestAPIKey() *responses.APIKey {
	return &responses.APIKey{
		Object:    "api_key",
		ID:        uuid.MustParse(testKeyID),
		AccountID: uuid.MustParse(testSubAccountID),
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		Label:     "Sub Key",
		PublicKey: "ak_subaccount_example",
	}
}

func newTestAPIKeyList() *responses.PaginatedAPIKeysResponse {
	return &responses.PaginatedAPIKeysResponse{
		Object: "list",
		Data:   []responses.APIKey{*newTestAPIKey()},
	}
}

// Group structure -----------------------------------------------------------

func TestAPIKeysCommand_Structure(t *testing.T) {
	cmd := NewCommand()
	assert.Equal(t, "api-keys", cmd.Name())
	assert.Contains(t, cmd.Aliases, "apikeys")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	subcommands := make([]string, 0)
	for _, c := range cmd.Commands() {
		subcommands = append(subcommands, c.Name())
	}
	assert.ElementsMatch(t, []string{"list", "get", "create", "update", "delete"}, subcommands)
}

func TestAPIKeysCommand_Help(t *testing.T) {
	out, err := execCommand(NewCommand(), "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "Manage API keys that belong to a specific sub-account")
	for _, sub := range []string{"list", "create", "get", "update", "delete"} {
		assert.Contains(t, out, sub)
	}
}

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

func TestListCommand_Structure(t *testing.T) {
	cmd := NewListCommand()
	assert.Equal(t, "list", cmd.Name())
	assert.NotNil(t, cmd.Args)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
}

func TestListCommand_Flags(t *testing.T) {
	flags := NewListCommand().Flags()

	limitFlag := flags.Lookup("limit")
	require.NotNil(t, limitFlag)
	assert.Equal(t, "int32", limitFlag.Value.Type())
	assert.Equal(t, "0", limitFlag.DefValue)

	cursorFlag := flags.Lookup("cursor")
	require.NotNil(t, cursorFlag)
	assert.Equal(t, "string", cursorFlag.Value.Type())
	assert.Empty(t, cursorFlag.DefValue)
}

func TestListCommand_RequiresExactlyOneArg(t *testing.T) {
	_, err := execCommand(NewListCommand())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestListCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewListCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
	mockClient.AssertNotCalled(t, "ListSubAccountAPIKeys", mock.Anything, mock.Anything, mock.Anything)
}

func TestListCommand_LimitOutOfBoundsFailsBeforeAuth(t *testing.T) {
	for _, limit := range []string{"--limit=-1", "--limit=101"} {
		t.Run(limit, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			called := trackingResolver(t, mockClient)

			_, err := execCommand(NewListCommand(), testSubAccountID, limit)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid page limit")
			assert.False(t, *called, "auth resolver must not be reached when validation fails")
			mockClient.AssertNotCalled(t, "ListSubAccountAPIKeys", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestListCommand_LimitZeroAccepted_NilPagination(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccountAPIKeys", testSubAccountID, (*int32)(nil), (*string)(nil)).
		Return(newTestAPIKeyList(), nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommand(NewListCommand(), testSubAccountID, "--limit=0")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Sub Key")
	mockClient.AssertExpectations(t)
}

func TestListCommand_LimitMaxAccepted(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccountAPIKeys", testSubAccountID,
		mock.MatchedBy(func(l *int32) bool { return l != nil && *l == 100 }),
		(*string)(nil)).
		Return(&responses.PaginatedAPIKeysResponse{Object: "list"}, nil)
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewListCommand(), testSubAccountID, "--limit=100")
	require.NoError(t, err)
	assert.True(t, *called)
	mockClient.AssertExpectations(t)
}

// A non-nil list response flows through the shared API-key renderer so JSON
// output stays a verbatim SDK PaginatedAPIKeysResponse.
func TestListCommand_JSONVerbatimPassThrough(t *testing.T) {
	nextCursor := "next-page"
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccountAPIKeys", testSubAccountID, (*int32)(nil), (*string)(nil)).
		Return(&responses.PaginatedAPIKeysResponse{
			Object: "list",
			Data:   []responses.APIKey{*newTestAPIKey()},
			Pagination: common.PaginationInfo{
				HasMore:    true,
				NextCursor: &nextCursor,
			},
		}, nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommandJSON(NewListCommand(), testSubAccountID)
	require.NoError(t, err)
	assert.True(t, *called)

	assert.Contains(t, out, `"object": "list"`)
	assert.Contains(t, out, "next-page")
	assert.Contains(t, out, "ak_subaccount_example")
	assert.NotContains(t, out, `"empty"`)
	mockClient.AssertExpectations(t)
}

// get ------------------------------------------------------------------------

func TestGetCommand_Structure(t *testing.T) {
	cmd := NewGetCommand()
	assert.Equal(t, "get", cmd.Name())
	assert.NotNil(t, cmd.Args)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
}

func TestGetCommand_RequiresExactlyTwoArgs(t *testing.T) {
	_, err := execCommand(NewGetCommand(), testSubAccountID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 2 arg")
}

func TestGetCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewGetCommand(), "not-a-uuid", testKeyID)
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
	mockClient.AssertNotCalled(t, "GetSubAccountAPIKey", mock.Anything, mock.Anything)
}

func TestGetCommand_BadKeyIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewGetCommand(), testSubAccountID, "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, "invalid API key ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed API key ID")
	mockClient.AssertNotCalled(t, "GetSubAccountAPIKey", mock.Anything, mock.Anything)
}

func TestGetCommand_MockBacked(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetSubAccountAPIKey", testSubAccountID, testKeyID).Return(newTestAPIKey(), nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommand(NewGetCommand(), testSubAccountID, testKeyID)
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Sub Key")
	mockClient.AssertExpectations(t)
}

func TestGetCommand_JSONVerbatimPassThrough(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetSubAccountAPIKey", testSubAccountID, testKeyID).Return(newTestAPIKey(), nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommandJSON(NewGetCommand(), testSubAccountID, testKeyID)
	require.NoError(t, err)
	assert.True(t, *called)

	assert.Contains(t, out, `"object": "api_key"`)
	assert.Contains(t, out, "ak_subaccount_example")
	assert.Contains(t, out, testKeyID)
	mockClient.AssertExpectations(t)
}

// execCommandFormat mirrors execCommand but installs a handler for an arbitrary
// output format so tests can exercise table-vs-plain-vs-json behavior.
func execCommandFormat(format string, cmd *cobra.Command, args ...string) (string, error) {
	var buf bytes.Buffer
	handler := printer.GetResponseHandler(format, false, &buf)
	ctx := context.WithValue(context.Background(), printer.ResponseHandlerKey, handler)
	cmd.SetContext(ctx)
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func newTestAPIKeyWithSecret() *responses.APIKey {
	key := newTestAPIKey()
	secret := "sk_one_time_secret_value"
	key.SecretKey = &secret
	return key
}

const validScope = "messages:send:all"

// create ----------------------------------------------------------------------

func TestCreateCommand_Structure(t *testing.T) {
	cmd := NewCreateCommand()
	assert.Equal(t, "create", cmd.Name())
	assert.NotNil(t, cmd.Args)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)

	flags := cmd.Flags()
	require.NotNil(t, flags.Lookup("label"))
	require.NotNil(t, flags.Lookup("scope"))
	require.NotNil(t, flags.Lookup("idempotency-key"))
}

// The nested create help promises the idempotency flag and the 5-minute replay
// window, while the top-level apikeys create help promises neither.
func TestCreateCommand_HelpMentionsIdempotencyAndReplayWindow(t *testing.T) {
	out, err := execCommand(NewCreateCommand(), "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "--idempotency-key")
	assert.Contains(t, out, "5-minute replay window")

	topOut, err := execCommand(topapikeys.NewCreateCommand(), "--help")
	require.NoError(t, err)
	lower := strings.ToLower(topOut)
	assert.NotContains(t, lower, "idempotency")
	assert.NotContains(t, lower, "replay")
}

func TestCreateCommand_RequiresLabelAndScope(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewCreateCommand(), testSubAccountID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag(s)")
	assert.Contains(t, err.Error(), "label")
	assert.Contains(t, err.Error(), "scope")
	assert.False(t, *called, "auth resolver must not be reached when required flags are missing")
}

func TestCreateCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewCreateCommand(), "not-a-uuid",
		"--label", "CI", "--scope", validScope)
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
	mockClient.AssertNotCalled(t, "CreateSubAccountAPIKey", mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateCommand_InvalidScopeFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewCreateCommand(), testSubAccountID,
		"--label", "CI", "--scope", "not-a-real-scope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scope: not-a-real-scope")
	assert.False(t, *called, "auth resolver must not be reached for an invalid scope")
	mockClient.AssertNotCalled(t, "CreateSubAccountAPIKey", mock.Anything, mock.Anything, mock.Anything)
}

// When --idempotency-key is provided it is passed through verbatim.
func TestCreateCommand_IdempotencyKeyPassThrough(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", testSubAccountID,
		requests.CreateAPIKeyRequest{Label: "CI", Scopes: []string{validScope}},
		"my-key").
		Return(newTestAPIKey(), nil)
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewCreateCommand(), testSubAccountID,
		"--label", "CI", "--scope", validScope, "--idempotency-key", "my-key")
	require.NoError(t, err)
	assert.True(t, *called)
	mockClient.AssertExpectations(t)
}

// When --idempotency-key is omitted a non-empty UUID is generated and passed.
func TestCreateCommand_IdempotencyKeyGenerated(t *testing.T) {
	var capturedKey string
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", testSubAccountID,
		requests.CreateAPIKeyRequest{Label: "CI", Scopes: []string{validScope}},
		mock.MatchedBy(func(key string) bool {
			capturedKey = key
			_, err := uuid.Parse(key)
			return key != "" && err == nil
		})).
		Return(newTestAPIKey(), nil)
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewCreateCommand(), testSubAccountID,
		"--label", "CI", "--scope", validScope)
	require.NoError(t, err)
	assert.True(t, *called)
	assert.NotEmpty(t, capturedKey)
	mockClient.AssertExpectations(t)
}

// The created APIKey flows through HandleCreateAPIKey so JSON preserves the
// one-time secret_key verbatim.
func TestCreateCommand_JSONPreservesSecretKey(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", testSubAccountID,
		mock.AnythingOfType("requests.CreateAPIKeyRequest"),
		mock.AnythingOfType("string")).
		Return(newTestAPIKeyWithSecret(), nil)
	trackingResolver(t, mockClient)

	out, err := execCommandJSON(NewCreateCommand(), testSubAccountID,
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
			mockClient.On("CreateSubAccountAPIKey", testSubAccountID,
				mock.AnythingOfType("requests.CreateAPIKeyRequest"),
				mock.AnythingOfType("string")).
				Return(newTestAPIKeyWithSecret(), nil)
			trackingResolver(t, mockClient)

			out, err := execCommandFormat(format, NewCreateCommand(), testSubAccountID,
				"--label", "CI", "--scope", validScope)
			require.NoError(t, err)
			assert.Contains(t, out, "5-minute replay window")
			mockClient.AssertExpectations(t)
		})
	}
}

// The note is suppressed when the secret is absent, even for human formats.
func TestCreateCommand_NoReplayNoteWhenSecretMissing(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", testSubAccountID,
		mock.AnythingOfType("requests.CreateAPIKeyRequest"),
		mock.AnythingOfType("string")).
		Return(newTestAPIKey(), nil) // no SecretKey
	trackingResolver(t, mockClient)

	out, err := execCommandFormat("plain", NewCreateCommand(), testSubAccountID,
		"--label", "CI", "--scope", validScope)
	require.NoError(t, err)
	assert.NotContains(t, out, "replay window")
	mockClient.AssertExpectations(t)
}

// The note is not printed for CSV output.
func TestCreateCommand_NoReplayNoteForCSV(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", testSubAccountID,
		mock.AnythingOfType("requests.CreateAPIKeyRequest"),
		mock.AnythingOfType("string")).
		Return(newTestAPIKeyWithSecret(), nil)
	trackingResolver(t, mockClient)

	out, err := execCommandFormat("csv", NewCreateCommand(), testSubAccountID,
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
			mockClient.On("CreateSubAccountAPIKey", testSubAccountID,
				mock.AnythingOfType("requests.CreateAPIKeyRequest"),
				mock.AnythingOfType("string")).
				Return(nil, apiErr)
			trackingResolver(t, mockClient)

			_, err := execCommandJSON(NewCreateCommand(), testSubAccountID,
				"--label", "CI", "--scope", validScope)

			// The runner returns the SDK error verbatim with its raw body.
			returnedAPIErr, ok := err.(*api.APIError)
			require.True(t, ok, "runner must return the raw SDK *api.APIError")
			assert.Equal(t, tc.statusCode, returnedAPIErr.StatusCode)
			assert.JSONEq(t, tc.raw, string(returnedAPIErr.Raw))

			// The JSON handler prints the raw body and returns nil (exit 0).
			var buf bytes.Buffer
			handler := printer.GetResponseHandler("json", false, &buf)
			assert.NoError(t, handler.HandleError(err))
			assert.JSONEq(t, tc.raw, buf.String())

			mockClient.AssertExpectations(t)
		})
	}
}

// update ----------------------------------------------------------------------

func TestUpdateCommand_Structure(t *testing.T) {
	cmd := NewUpdateCommand()
	assert.Equal(t, "update", cmd.Name())
	assert.NotNil(t, cmd.Args)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)

	flags := cmd.Flags()
	require.NotNil(t, flags.Lookup("label"))
	require.NotNil(t, flags.Lookup("scope"))
}

func TestUpdateCommand_RequiresExactlyTwoArgs(t *testing.T) {
	_, err := execCommand(NewUpdateCommand(), testSubAccountID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 2 arg")
}

func TestUpdateCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewUpdateCommand(), "not-a-uuid", testKeyID, "--label", "New")
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
	mockClient.AssertNotCalled(t, "UpdateSubAccountAPIKey", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateCommand_BadKeyIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewUpdateCommand(), testSubAccountID, "not-a-uuid", "--label", "New")
	require.Error(t, err)
	assert.Equal(t, "invalid API key ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed API key ID")
	mockClient.AssertNotCalled(t, "UpdateSubAccountAPIKey", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateCommand_NoChangedFlagsFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewUpdateCommand(), testSubAccountID, testKeyID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of --label or --scope must be provided")
	assert.False(t, *called, "auth resolver must not be reached when no changed flags are provided")
	mockClient.AssertNotCalled(t, "UpdateSubAccountAPIKey", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateCommand_MockBacked(t *testing.T) {
	label := "New Label"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateSubAccountAPIKey", testSubAccountID, testKeyID,
		requests.UpdateAPIKeyRequest{Label: &label}).
		Return(newTestAPIKey(), nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommand(NewUpdateCommand(), testSubAccountID, testKeyID, "--label", label)
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Sub Key")
	mockClient.AssertExpectations(t)
}

// delete ----------------------------------------------------------------------

func TestDeleteCommand_Structure(t *testing.T) {
	cmd := NewDeleteCommand()
	assert.Equal(t, "delete", cmd.Name())
	assert.NotNil(t, cmd.Args)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
	require.NotNil(t, cmd.Flags().Lookup("force"))
}

func TestDeleteCommand_RequiresExactlyTwoArgs(t *testing.T) {
	_, err := execCommand(NewDeleteCommand(), testSubAccountID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 2 arg")
}

func TestDeleteCommand_BadSubAccountIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewDeleteCommand(), "not-a-uuid", testKeyID, "--force")
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed sub-account ID")
	mockClient.AssertNotCalled(t, "DeleteSubAccountAPIKey", mock.Anything, mock.Anything)
}

func TestDeleteCommand_BadKeyIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewDeleteCommand(), testSubAccountID, "not-a-uuid", "--force")
	require.Error(t, err)
	assert.Equal(t, "invalid API key ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed API key ID")
	mockClient.AssertNotCalled(t, "DeleteSubAccountAPIKey", mock.Anything, mock.Anything)
}

func TestDeleteCommand_ForceMockBacked(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSubAccountAPIKey", testSubAccountID, testKeyID).
		Return(&common.SuccessResponse{}, nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommand(NewDeleteCommand(), testSubAccountID, testKeyID, "--force")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Deleted")
	mockClient.AssertExpectations(t)
}
