package subaccounts

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

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

// trackingResolver installs a test auth resolver that records whether it was
// invoked and returns the supplied client. The returned restore function and
// the *bool let tests assert validation-before-auth ordering.
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

func newTestSubAccount() *responses.SubAccount {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	parentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	return &responses.SubAccount{
		Object:          "sub_account",
		ID:              id,
		ParentAccountID: parentID,
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Name:            "Acme Subsidiary",
		Website:         "https://acme.example",
		Status:          "active",
		MonthlyCredit:   500,
		DomainCount:     3,
		MemberCount:     7,
	}
}

func newTestSubAccountUsage() *responses.SubAccountUsageResponse {
	parentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	subID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	subName := "Acme Subsidiary"
	return &responses.SubAccountUsageResponse{
		BillingPeriod: responses.SubAccountUsageBillingPeriod{
			Start: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		},
		Currency:         "usd",
		AllocationMethod: "proportional",
		Parent: responses.SubAccountUsageBreakdown{
			AccountID:      &parentID,
			ReceptionCount: 1000,
			AllocatedCost:  10,
		},
		SubAccounts: []responses.SubAccountUsageBreakdown{
			{
				AccountID:      &subID,
				Name:           &subName,
				ReceptionCount: 3000,
				AllocatedCost:  30,
			},
		},
		Total: responses.SubAccountUsageBreakdown{
			ReceptionCount: 4000,
			AllocatedCost:  40,
		},
	}
}

// Group structure -----------------------------------------------------------

func TestSubAccountsCommand_Structure(t *testing.T) {
	cmd := NewCommand()
	assert.Equal(t, "subaccounts", cmd.Name())
	assert.Equal(t, "Manage your AhaSend sub-accounts", cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	subcommands := make([]string, 0)
	for _, c := range cmd.Commands() {
		subcommands = append(subcommands, c.Name())
	}
	assert.ElementsMatch(t, []string{"list", "get", "usage", "create", "update", "delete", "suspend", "unsuspend", "api-keys"}, subcommands)
}

func TestSubAccountsCommand_Help(t *testing.T) {
	out, err := execCommand(NewCommand(), "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "Manage sub-accounts under your AhaSend parent account")
	assert.Contains(t, out, "list")
	assert.Contains(t, out, "get")
	assert.Contains(t, out, "usage")
}

// list -----------------------------------------------------------------------

func TestListCommand_Structure(t *testing.T) {
	cmd := NewListCommand()
	assert.Equal(t, "list", cmd.Name())
	assert.Equal(t, "List all sub-accounts", cmd.Short)
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

func TestListCommand_LimitOutOfBoundsFailsBeforeAuth(t *testing.T) {
	for _, limit := range []string{"--limit=-1", "--limit=101"} {
		t.Run(limit, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			called := trackingResolver(t, mockClient)

			_, err := execCommand(NewListCommand(), limit)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid page limit")
			assert.False(t, *called, "auth resolver must not be reached when validation fails")
			mockClient.AssertNotCalled(t, "ListSubAccounts", mock.Anything, mock.Anything)
		})
	}
}

func TestListCommand_LimitZeroAccepted_NilPagination(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccounts", (*int32)(nil), (*string)(nil)).
		Return(&responses.PaginatedSubAccountsResponse{
			Object: "list",
			Data:   []responses.SubAccount{*newTestSubAccount()},
		}, nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommand(NewListCommand(), "--limit=0")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
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

// An empty (but non-nil) list response must still flow through the sub-account
// renderer so JSON output stays a verbatim SDK PaginatedSubAccountsResponse with
// its pagination metadata, rather than the generic empty CLI wrapper.
func TestListCommand_EmptyResponseJSONPassThrough(t *testing.T) {
	nextCursor := "next-page"
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccounts", (*int32)(nil), (*string)(nil)).
		Return(&responses.PaginatedSubAccountsResponse{
			Object: "list",
			Data:   []responses.SubAccount{},
			Pagination: common.PaginationInfo{
				HasMore:    true,
				NextCursor: &nextCursor,
			},
		}, nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommandJSON(NewListCommand())
	require.NoError(t, err)
	assert.True(t, *called)

	// SDK struct fields are present; the empty CLI wrapper fields are not.
	assert.Contains(t, out, `"object": "list"`)
	assert.Contains(t, out, "next-page")
	assert.NotContains(t, out, `"empty"`)
	mockClient.AssertExpectations(t)
}

func TestListCommand_LimitMaxAccepted(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccounts",
		mock.MatchedBy(func(l *int32) bool { return l != nil && *l == 100 }),
		(*string)(nil)).
		Return(&responses.PaginatedSubAccountsResponse{Object: "list"}, nil)
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewListCommand(), "--limit=100")
	require.NoError(t, err)
	assert.True(t, *called)
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

func TestGetCommand_RequiresExactlyOneArg(t *testing.T) {
	_, err := execCommand(NewGetCommand())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestGetCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewGetCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
	mockClient.AssertNotCalled(t, "GetSubAccount", mock.Anything)
}

func TestGetCommand_MockBacked(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetSubAccount", id).Return(newTestSubAccount(), nil)
	trackingResolver(t, mockClient)

	out, err := execCommand(NewGetCommand(), id)
	require.NoError(t, err)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
}

// usage ----------------------------------------------------------------------

func TestUsageCommand_Structure(t *testing.T) {
	cmd := NewUsageCommand()
	assert.Equal(t, "usage", cmd.Name())
	assert.NotNil(t, cmd.Args)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
}

func TestUsageCommand_RejectsArgs(t *testing.T) {
	mockClient := &mocks.MockClient{}
	trackingResolver(t, mockClient)

	_, err := execCommand(NewUsageCommand(), "unexpected")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestUsageCommand_MockBacked(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetSubAccountsUsage").Return(newTestSubAccountUsage(), nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommand(NewUsageCommand())
	require.NoError(t, err)
	assert.True(t, *called)
	assert.NotEmpty(t, out)
	mockClient.AssertCalled(t, "GetSubAccountsUsage")
	mockClient.AssertExpectations(t)
}

// create ----------------------------------------------------------------------

func TestCreateCommand_Structure(t *testing.T) {
	cmd := NewCreateCommand()
	assert.Equal(t, "create", cmd.Name())
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)

	flags := cmd.Flags()
	require.NotNil(t, flags.Lookup("name"))
	require.NotNil(t, flags.Lookup("website"))

	creditFlag := flags.Lookup("monthly-credit")
	require.NotNil(t, creditFlag)
	assert.Equal(t, "int64", creditFlag.Value.Type())

	require.NotNil(t, flags.Lookup("idempotency-key"))
}

func TestCreateCommand_RequiresNameAndWebsite(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"missing name", []string{"--website", "https://acme.example"}, "--name is required"},
		{"blank name", []string{"--name", "   ", "--website", "https://acme.example"}, "--name is required"},
		{"missing website", []string{"--name", "Acme"}, "--website is required"},
		{"invalid website host", []string{"--name", "Acme", "--website", "acme"}, "invalid website"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			called := trackingResolver(t, mockClient)

			_, err := execCommand(NewCreateCommand(), tc.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.False(t, *called, "auth resolver must not be reached when validation fails")
			mockClient.AssertNotCalled(t, "CreateSubAccount", mock.Anything, mock.Anything)
		})
	}
}

func TestCreateCommand_GeneratesIdempotencyKeyWhenOmitted(t *testing.T) {
	var capturedKey string
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccount",
		mock.AnythingOfType("requests.CreateSubAccountRequest"),
		mock.MatchedBy(func(key string) bool {
			capturedKey = key
			return key != ""
		})).
		Return(newTestSubAccount(), nil)
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewCreateCommand(), "--name", "Acme", "--website", "https://acme.example")
	require.NoError(t, err)
	assert.True(t, *called)

	// A non-empty, valid UUID idempotency key is generated and passed through.
	_, parseErr := uuid.Parse(capturedKey)
	assert.NoError(t, parseErr, "generated idempotency key must be a UUID")
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_PassesUserProvidedIdempotencyKey(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccount",
		mock.AnythingOfType("requests.CreateSubAccountRequest"),
		"my-key").
		Return(newTestSubAccount(), nil)
	trackingResolver(t, mockClient)

	_, err := execCommand(NewCreateCommand(),
		"--name", "Acme", "--website", "https://acme.example", "--idempotency-key", "my-key")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_MonthlyCreditBounds(t *testing.T) {
	t.Run("below lower bound fails before auth", func(t *testing.T) {
		mockClient := &mocks.MockClient{}
		called := trackingResolver(t, mockClient)

		_, err := execCommand(NewCreateCommand(),
			"--name", "Acme", "--website", "https://acme.example", "--monthly-credit", "-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid monthly credit")
		assert.False(t, *called)
		mockClient.AssertNotCalled(t, "CreateSubAccount", mock.Anything, mock.Anything)
	})

	t.Run("above upper bound fails before auth", func(t *testing.T) {
		mockClient := &mocks.MockClient{}
		called := trackingResolver(t, mockClient)

		_, err := execCommand(NewCreateCommand(),
			"--name", "Acme", "--website", "https://acme.example", "--monthly-credit", "1000000001")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid monthly credit")
		assert.False(t, *called)
		mockClient.AssertNotCalled(t, "CreateSubAccount", mock.Anything, mock.Anything)
	})

	t.Run("explicit zero is accepted and sent", func(t *testing.T) {
		mockClient := &mocks.MockClient{}
		mockClient.On("CreateSubAccount",
			mock.MatchedBy(func(req requests.CreateSubAccountRequest) bool {
				return req.MonthlyCredit != nil && *req.MonthlyCredit == 0
			}),
			mock.AnythingOfType("string")).
			Return(newTestSubAccount(), nil)
		called := trackingResolver(t, mockClient)

		_, err := execCommand(NewCreateCommand(),
			"--name", "Acme", "--website", "https://acme.example", "--monthly-credit", "0")
		require.NoError(t, err)
		assert.True(t, *called)
		mockClient.AssertExpectations(t)
	})
}

func TestCreateCommand_OmittedMonthlyCreditIsNil(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccount",
		mock.MatchedBy(func(req requests.CreateSubAccountRequest) bool {
			return req.MonthlyCredit == nil
		}),
		mock.AnythingOfType("string")).
		Return(newTestSubAccount(), nil)
	trackingResolver(t, mockClient)

	_, err := execCommand(NewCreateCommand(), "--name", "Acme", "--website", "https://acme.example")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_MockBacked(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccount",
		mock.AnythingOfType("requests.CreateSubAccountRequest"),
		mock.AnythingOfType("string")).
		Return(newTestSubAccount(), nil)
	trackingResolver(t, mockClient)

	out, err := execCommand(NewCreateCommand(), "--name", "Acme", "--website", "https://acme.example")
	require.NoError(t, err)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
}

// Raw 409/422 SDK API errors are returned verbatim so the root error handler
// faithfully prints the API body in JSON mode and keeps the exit code at 0.
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
			mockClient.On("CreateSubAccount",
				mock.AnythingOfType("requests.CreateSubAccountRequest"),
				mock.AnythingOfType("string")).
				Return(nil, apiErr)
			trackingResolver(t, mockClient)

			_, err := execCommandJSON(NewCreateCommand(),
				"--name", "Acme", "--website", "https://acme.example")

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
	require.NotNil(t, flags.Lookup("name"))
	require.NotNil(t, flags.Lookup("website"))
	require.NotNil(t, flags.Lookup("monthly-credit"))
}

func TestUpdateCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewUpdateCommand(), "not-a-uuid", "--name", "New")
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
	mockClient.AssertNotCalled(t, "UpdateSubAccount", mock.Anything, mock.Anything)
}

func TestUpdateCommand_NoChangedFlagsFailsBeforeAuth(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewUpdateCommand(), id)
	require.Error(t, err)
	assert.Equal(t, "at least one of --name, --website, or --monthly-credit must be provided", err.Error())
	assert.False(t, *called, "auth resolver must not be reached when no flags change")
	mockClient.AssertNotCalled(t, "UpdateSubAccount", mock.Anything, mock.Anything)
}

func TestUpdateCommand_MonthlyCreditBounds(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"

	t.Run("below lower bound fails before auth", func(t *testing.T) {
		mockClient := &mocks.MockClient{}
		called := trackingResolver(t, mockClient)

		_, err := execCommand(NewUpdateCommand(), id, "--monthly-credit", "-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid monthly credit")
		assert.False(t, *called)
		mockClient.AssertNotCalled(t, "UpdateSubAccount", mock.Anything, mock.Anything)
	})

	t.Run("above upper bound fails before auth", func(t *testing.T) {
		mockClient := &mocks.MockClient{}
		called := trackingResolver(t, mockClient)

		_, err := execCommand(NewUpdateCommand(), id, "--monthly-credit", "1000000001")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid monthly credit")
		assert.False(t, *called)
		mockClient.AssertNotCalled(t, "UpdateSubAccount", mock.Anything, mock.Anything)
	})

	t.Run("explicit zero is accepted and sent", func(t *testing.T) {
		mockClient := &mocks.MockClient{}
		mockClient.On("UpdateSubAccount", id,
			mock.MatchedBy(func(req requests.UpdateSubAccountRequest) bool {
				return req.MonthlyCredit != nil && *req.MonthlyCredit == 0
			})).
			Return(newTestSubAccount(), nil)
		called := trackingResolver(t, mockClient)

		_, err := execCommand(NewUpdateCommand(), id, "--monthly-credit", "0")
		require.NoError(t, err)
		assert.True(t, *called)
		mockClient.AssertExpectations(t)
	})
}

func TestUpdateCommand_PartialUpdateOnlySetsChangedFields(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateSubAccount", id,
		mock.MatchedBy(func(req requests.UpdateSubAccountRequest) bool {
			return req.Name != nil && *req.Name == "New Name" &&
				req.Website == nil && req.MonthlyCredit == nil
		})).
		Return(newTestSubAccount(), nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommand(NewUpdateCommand(), id, "--name", "New Name")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Acme Subsidiary")
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

func TestDeleteCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewDeleteCommand(), "not-a-uuid", "--force")
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
	mockClient.AssertNotCalled(t, "DeleteSubAccount", mock.Anything)
}

func TestDeleteCommand_ForceSuccess(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSubAccount", id).
		Return(&common.SuccessResponse{Message: "deleted"}, nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommand(NewDeleteCommand(), id, "--force")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "deleted successfully")
	mockClient.AssertExpectations(t)
}

func TestDeleteCommand_ApiErrorPropagates(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSubAccount", id).Return(nil, assert.AnError)
	trackingResolver(t, mockClient)

	_, err := execCommand(NewDeleteCommand(), id, "--force")
	require.Error(t, err)
	mockClient.AssertExpectations(t)
}

// suspend ---------------------------------------------------------------------

func TestSuspendCommand_Structure(t *testing.T) {
	cmd := NewSuspendCommand()
	assert.Equal(t, "suspend", cmd.Name())
	assert.NotNil(t, cmd.Args)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
	require.NotNil(t, cmd.Flags().Lookup("reason"))
	require.NotNil(t, cmd.Flags().Lookup("force"))
}

func TestSuspendCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewSuspendCommand(), "not-a-uuid", "--reason", "x", "--force")
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
	mockClient.AssertNotCalled(t, "SuspendSubAccount", mock.Anything, mock.Anything)
}

func TestSuspendCommand_EmptyReasonFailsBeforeAuth(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	for _, reason := range []string{"", "   "} {
		t.Run("reason="+reason, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			called := trackingResolver(t, mockClient)

			_, err := execCommand(NewSuspendCommand(), id, "--reason", reason, "--force")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--reason is required")
			assert.False(t, *called, "auth resolver must not be reached for empty reason")
			mockClient.AssertNotCalled(t, "SuspendSubAccount", mock.Anything, mock.Anything)
		})
	}
}

func TestSuspendCommand_OverlongReasonFailsBeforeAuth(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	reason := strings.Repeat("a", 501)
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewSuspendCommand(), id, "--reason", reason, "--force")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--reason too long")
	assert.False(t, *called, "auth resolver must not be reached for overlong reason")
	mockClient.AssertNotCalled(t, "SuspendSubAccount", mock.Anything, mock.Anything)
}

func TestSuspendCommand_ForceSuccessSendsValidatedReason(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("SuspendSubAccount", id,
		mock.MatchedBy(func(req requests.SuspendSubAccountRequest) bool {
			return req.Reason == "Payment overdue"
		})).
		Return(newTestSubAccount(), nil)
	called := trackingResolver(t, mockClient)

	// Leading/trailing whitespace is trimmed before the reason is sent.
	out, err := execCommand(NewSuspendCommand(), id, "--reason", "  Payment overdue  ", "--force")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
}

// unsuspend -------------------------------------------------------------------

func TestUnsuspendCommand_Structure(t *testing.T) {
	cmd := NewUnsuspendCommand()
	assert.Equal(t, "unsuspend", cmd.Name())
	assert.NotNil(t, cmd.Args)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
}

func TestUnsuspendCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	mockClient := &mocks.MockClient{}
	called := trackingResolver(t, mockClient)

	_, err := execCommand(NewUnsuspendCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, "invalid sub-account ID format: not-a-uuid", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
	mockClient.AssertNotCalled(t, "UnsuspendSubAccount", mock.Anything)
}

func TestUnsuspendCommand_MockBacked(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("UnsuspendSubAccount", id).Return(newTestSubAccount(), nil)
	called := trackingResolver(t, mockClient)

	out, err := execCommand(NewUnsuspendCommand(), id)
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
}
