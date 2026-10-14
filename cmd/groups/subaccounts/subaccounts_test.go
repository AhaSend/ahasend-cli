package subaccounts

import (
	"bytes"
	"os"
	"strings"
	"testing"

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

// Group structure -----------------------------------------------------------

// list -----------------------------------------------------------------------

func TestListCommand_LimitZeroAccepted_NilPagination(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccounts", common.PaginationParams{}).
		Return(&responses.PaginatedSubAccountsResponse{
			Object: "list",
			Data:   []responses.SubAccount{*mocks.NewSubAccount()},
		}, nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewListCommand(), "plain", "--limit=0")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
}

// An empty (but non-nil) list response must still flow through the sub-account
// renderer so JSON output stays a verbatim SDK PaginatedSubAccountsResponse with
// its pagination metadata, rather than the generic empty CLI wrapper.
func TestListCommand_EmptyResponseJSONPassThrough(t *testing.T) {
	nextCursor := "next-page"
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSubAccounts", common.PaginationParams{}).
		Return(&responses.PaginatedSubAccountsResponse{
			Object: "list",
			Data:   []responses.SubAccount{},
			Pagination: common.PaginationInfo{
				HasMore:    true,
				NextCursor: &nextCursor,
			},
		}, nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewListCommand(), "json")
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
		mock.MatchedBy(func(p common.PaginationParams) bool { return p.Limit != nil && *p.Limit == 100 })).
		Return(&responses.PaginatedSubAccountsResponse{Object: "list"}, nil)
	called := testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewListCommand(), "plain", "--limit=100")
	require.NoError(t, err)
	assert.True(t, *called)
	mockClient.AssertExpectations(t)
}

// get ------------------------------------------------------------------------

func TestGetCommand_RequiresExactlyOneArg(t *testing.T) {
	_, err := testutil.RunCommandCombined(t, NewGetCommand(), "plain")
	require.Error(t, err)
	assert.Equal(t, "the sub-account-id argument is required", err.Error())
}

func TestGetCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewGetCommand(), "plain", "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
}

func TestGetCommand_MockBacked(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetSubAccount", uuid.MustParse(id)).Return(mocks.NewSubAccount(), nil)
	testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewGetCommand(), "plain", id)
	require.NoError(t, err)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
}

// usage ----------------------------------------------------------------------

func TestUsageCommand_RejectsArgs(t *testing.T) {
	mockClient := &mocks.MockClient{}
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewUsageCommand(), "plain", "unexpected")
	require.Error(t, err)
	assert.Equal(t, `unexpected argument "unexpected"`, err.Error())
}

func TestUsageCommand_MockBacked(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetSubAccountsUsage").Return(mocks.NewSubAccountUsage(0), nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewUsageCommand(), "plain")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.NotEmpty(t, out)
	mockClient.AssertCalled(t, "GetSubAccountsUsage")
	mockClient.AssertExpectations(t)
}

// create ----------------------------------------------------------------------

func TestCreateCommand_RequiresNameAndWebsite(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"missing name", []string{"--website", "acme.example.com"}, "--name is required"},
		{"blank name", []string{"--name", "   ", "--website", "acme.example.com"}, "--name cannot be empty"},
		{"missing website", []string{"--name", "Acme"}, "--website is required"},
		{"blank website", []string{"--name", "Acme", "--website", " \t"}, "--website cannot be empty"},
		{"invalid website host", []string{"--name", "Acme", "--website", "acme"}, `invalid --website value "acme"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", tc.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.False(t, *called, "auth resolver must not be reached when validation fails")
		})
	}
}

func TestNormalizeWebsite(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{"bare domain", "acme.example.com", "acme.example.com", ""},
		{"surrounding whitespace", "  acme.example.com  ", "acme.example.com", ""},
		{"uppercase", "Acme.Example.COM", "acme.example.com", ""},
		{"trailing dot", "acme.example.com.", "acme.example.com", ""},
		{"https URL", "https://acme.example.com", "acme.example.com", ""},
		{"http URL with root path", "http://acme.example.com/", "acme.example.com", ""},
		{"uppercase scheme", "HTTPS://ACME.COM/Path", "acme.com", ""},
		// The API drops the path, query, fragment and port, with or without
		// a scheme, and a leading "//"
		{"URL with path", "https://acme.com/about", "acme.com", ""},
		{"URL with query and fragment", "https://acme.com/team?utm=1#frag", "acme.com", ""},
		{"URL with port", "https://acme.example.com:8443", "acme.example.com", ""},
		{"bare domain with root path", "acme.com/", "acme.com", ""},
		{"bare domain with path", "acme.example.com/about", "acme.example.com", ""},
		{"bare domain with port", "acme.example.com:8080", "acme.example.com", ""},
		{"bare domain with port and path", "acme.com:8443/team", "acme.com", ""},
		{"bare domain with query", "acme.com?utm=1", "acme.com", ""},
		{"protocol-relative", "//acme.com", "acme.com", ""},
		// An international domain is sent in Punycode, as the API stores it
		{"international domain", "bücher.de", "xn--bcher-kva.de", ""},
		{"international URL", "https://MÜNCHEN.de/über", "xn--mnchen-3ya.de", ""},
		{"single label", "acme", "", "must be a fully qualified domain"},
		{"single label URL", "https://acme", "", "must be a fully qualified domain"},
		{"numeric TLD", "10.0.0.1", "", "valid top-level domain"},
		{"hyphen in the TLD", "example.xn--p1ai", "", "not a domain name"},
		{"leading hyphen", "-acme.example.com", "", "not a domain name"},
		{"underscore", "ac_me.example.com", "", "not a domain name"},
		{"too long", strings.Repeat(strings.Repeat("a", 60)+".", 4) + strings.Repeat("b", 11) + ".com", "",
			"longer than 255 characters"},
		{"at the length limit", strings.Repeat(strings.Repeat("a", 60)+".", 4) + strings.Repeat("b", 7) + ".com",
			strings.Repeat(strings.Repeat("a", 60)+".", 4) + strings.Repeat("b", 7) + ".com", ""},
		{"URL with credentials", "https://user:pass@acme.example.com", "", "must not include credentials"},
		{"bare domain with credentials", "acme.com@evil.com", "", "must not include credentials"},
		{"non-http scheme", "ftp://acme.example.com", "", "only http and https"},
		{"javascript", "javascript:alert(1)", "", "not a domain or an http or https URL"},
		{"URL without host", "https://", "", "not a domain name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeWebsite(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCreateCommand_SendsWebsiteAsDomain(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccount",
		mock.MatchedBy(func(req requests.CreateSubAccountRequest) bool {
			return req.Website == "acme.example.com"
		})).
		Return(mocks.NewSubAccount(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", "--name", "Acme", "--website", "https://acme.example.com/")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_SendsAnInternationalWebsiteInPunycode(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccount",
		mock.MatchedBy(func(req requests.CreateSubAccountRequest) bool {
			return req.Website == "xn--bcher-kva.de"
		})).
		Return(mocks.NewSubAccount(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", "--name", "Acme", "--website", "https://bücher.de/about")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_InvalidWebsiteFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", "--name", "Acme", "--website", "ftp://acme.example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --website value")
	assert.False(t, *called)
}

func TestCreateCommand_LeavesTheIdempotencyKeyToTheSDKWhenOmitted(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccount", mock.AnythingOfType("requests.CreateSubAccountRequest")).
		Return(mocks.NewSubAccount(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", "--name", "Acme", "--website", "acme.example.com")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_PassesUserProvidedIdempotencyKey(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccount",
		mock.AnythingOfType("requests.CreateSubAccountRequest"),
		testutil.IdempotencyKeyArg("my-key")).
		Return(mocks.NewSubAccount(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain",
		"--name", "Acme", "--website", "acme.example.com", "--idempotency-key", "my-key")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_MonthlyCreditBounds(t *testing.T) {
	t.Run("below lower bound fails before auth", func(t *testing.T) {
		called := testutil.RefuseAuth(t)

		_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain",
			"--name", "Acme", "--website", "acme.example.com", "--monthly-credit", "-1")
		require.Error(t, err)
		assert.Equal(t, "--monthly-credit must be between 0 and 1000000000", err.Error())
		assert.False(t, *called)
	})

	t.Run("above upper bound fails before auth", func(t *testing.T) {
		called := testutil.RefuseAuth(t)

		_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain",
			"--name", "Acme", "--website", "acme.example.com", "--monthly-credit", "1000000001")
		require.Error(t, err)
		assert.Equal(t, "--monthly-credit must be between 0 and 1000000000", err.Error())
		assert.False(t, *called)
	})

	t.Run("explicit zero is accepted and sent", func(t *testing.T) {
		mockClient := &mocks.MockClient{}
		mockClient.On("CreateSubAccount",
			mock.MatchedBy(func(req requests.CreateSubAccountRequest) bool {
				return req.MonthlyCredit != nil && *req.MonthlyCredit == 0
			})).
			Return(mocks.NewSubAccount(), nil)
		called := testutil.InjectClient(t, mockClient)

		_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain",
			"--name", "Acme", "--website", "acme.example.com", "--monthly-credit", "0")
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
		})).
		Return(mocks.NewSubAccount(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", "--name", "Acme", "--website", "acme.example.com")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_MockBacked(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccount",
		mock.AnythingOfType("requests.CreateSubAccountRequest")).
		Return(mocks.NewSubAccount(), nil)
	testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewCreateCommand(), "plain", "--name", "Acme", "--website", "acme.example.com")
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
				mock.AnythingOfType("requests.CreateSubAccountRequest")).
				Return(nil, apiErr)
			testutil.InjectClient(t, mockClient)

			_, err := testutil.RunCommandCombined(t, NewCreateCommand(), "json",
				"--name", "Acme", "--website", "acme.example.com")

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

func TestUpdateCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", "not-a-uuid", "--name", "New")
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
}

func TestUpdateCommand_NoChangedFlagsFailsBeforeAuth(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", id)
	require.Error(t, err)
	assert.Equal(t, "at least one of --name, --website or --monthly-credit must be provided", err.Error())
	assert.False(t, *called, "auth resolver must not be reached when no flags change")
}

func TestUpdateCommand_MonthlyCreditBounds(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"

	t.Run("below lower bound fails before auth", func(t *testing.T) {
		called := testutil.RefuseAuth(t)

		_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", id, "--monthly-credit", "-1")
		require.Error(t, err)
		assert.Equal(t, "--monthly-credit must be between 0 and 1000000000", err.Error())
		assert.False(t, *called)
	})

	t.Run("above upper bound fails before auth", func(t *testing.T) {
		called := testutil.RefuseAuth(t)

		_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", id, "--monthly-credit", "1000000001")
		require.Error(t, err)
		assert.Equal(t, "--monthly-credit must be between 0 and 1000000000", err.Error())
		assert.False(t, *called)
	})

	t.Run("explicit zero is accepted and sent", func(t *testing.T) {
		mockClient := &mocks.MockClient{}
		mockClient.On("UpdateSubAccount", uuid.MustParse(id),
			mock.MatchedBy(func(req requests.UpdateSubAccountRequest) bool {
				return req.MonthlyCredit != nil && *req.MonthlyCredit == 0
			})).
			Return(mocks.NewSubAccount(), nil)
		called := testutil.InjectClient(t, mockClient)

		_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", id, "--monthly-credit", "0")
		require.NoError(t, err)
		assert.True(t, *called)
		mockClient.AssertExpectations(t)
	})
}

func TestUpdateCommand_PartialUpdateOnlySetsChangedFields(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateSubAccount", uuid.MustParse(id),
		mock.MatchedBy(func(req requests.UpdateSubAccountRequest) bool {
			return req.Name != nil && *req.Name == "New Name" &&
				req.Website == nil && req.MonthlyCredit == nil
		})).
		Return(mocks.NewSubAccount(), nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", id, "--name", "New Name")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
}

func TestUpdateCommand_SendsWebsiteAsDomain(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateSubAccount", uuid.MustParse(id),
		mock.MatchedBy(func(req requests.UpdateSubAccountRequest) bool {
			return req.Website != nil && *req.Website == "acme.example.com" &&
				req.Name == nil && req.MonthlyCredit == nil
		})).
		Return(mocks.NewSubAccount(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", id, "--website", "https://acme.example.com")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateCommand_InvalidWebsiteFailsBeforeAuth(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	tests := []struct {
		website string
		message string
	}{
		{"", "--website cannot be empty"},
		{"acme", `invalid --website value "acme": must be a fully qualified domain; use a domain such as example.com`},
		{"https://user@acme.example.com", `invalid --website value "https://user@acme.example.com": must not include credentials; use a domain such as example.com`},
	}
	for _, test := range tests {
		t.Run(test.website, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, err := testutil.RunCommandCombined(t, NewUpdateCommand(), "plain", id, "--website", test.website)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *called)
		})
	}
}

// delete ----------------------------------------------------------------------

func TestDeleteCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewDeleteCommand(), "plain", "not-a-uuid", "--force")
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
}

func TestDeleteCommand_ForceSuccess(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSubAccount", uuid.MustParse(id)).
		Return(&common.SuccessResponse{Message: "deleted"}, nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewDeleteCommand(), "plain", id, "--force")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Equal(t, "deleted\n", out, "table output prints the API's message")
	mockClient.AssertExpectations(t)
}

func TestDeleteCommand_ApiErrorPropagates(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSubAccount", uuid.MustParse(id)).Return(nil, assert.AnError)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewDeleteCommand(), "plain", id, "--force")
	require.Error(t, err)
	mockClient.AssertExpectations(t)
}

// suspend ---------------------------------------------------------------------

func TestSuspendCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewSuspendCommand(), "plain", "not-a-uuid", "--reason", "x", "--force")
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
}

func TestSuspendCommand_EmptyReasonFailsBeforeAuth(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	for _, reason := range []string{"", "   "} {
		t.Run("reason="+reason, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, err := testutil.RunCommandCombined(t, NewSuspendCommand(), "plain", id, "--reason", reason, "--force")
			require.Error(t, err)
			assert.Equal(t, "--reason cannot be empty", err.Error())
			assert.False(t, *called, "auth resolver must not be reached for empty reason")
		})
	}
}

func TestSuspendCommand_OverlongReasonFailsBeforeAuth(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	reason := strings.Repeat("a", 501)
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewSuspendCommand(), "plain", id, "--reason", reason, "--force")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--reason must be at most 500 characters")
	assert.False(t, *called, "auth resolver must not be reached for overlong reason")
}

func TestSuspendCommand_ForceSuccessSendsValidatedReason(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("SuspendSubAccount", uuid.MustParse(id),
		mock.MatchedBy(func(req requests.SuspendSubAccountRequest) bool {
			return req.Reason == "Payment overdue"
		})).
		Return(mocks.NewSubAccount(), nil)
	called := testutil.InjectClient(t, mockClient)

	// Leading/trailing whitespace is trimmed before the reason is sent.
	out, err := testutil.RunCommandCombined(t, NewSuspendCommand(), "plain", id, "--reason", "  Payment overdue  ", "--force")
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
}

func TestSuspendCommand_CountsReasonCharactersNotBytes(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	reason := strings.Repeat("é", 500)
	mockClient := &mocks.MockClient{}
	mockClient.On("SuspendSubAccount", uuid.MustParse(id), requests.SuspendSubAccountRequest{Reason: reason}).
		Return(mocks.NewSubAccount(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewSuspendCommand(), "plain", id, "--reason", reason, "--force")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)

	_, err = testutil.RunCommandCombined(t, NewSuspendCommand(), "plain", id, "--reason", reason+"é", "--force")
	require.Error(t, err)
	assert.Equal(t, "--reason must be at most 500 characters", err.Error())
}

// unsuspend -------------------------------------------------------------------

func TestUnsuspendCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUnsuspendCommand(), "plain", "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
}

func TestUnsuspendCommand_MockBacked(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("UnsuspendSubAccount", uuid.MustParse(id)).Return(mocks.NewSubAccount(), nil)
	called := testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewUnsuspendCommand(), "plain", id)
	require.NoError(t, err)
	assert.True(t, *called)
	assert.Contains(t, out, "Acme Subsidiary")
	mockClient.AssertExpectations(t)
}

// pause, resume and unpause aliases ---------------------------------------------

func TestSuspendAndUnsuspendAliasesResolve(t *testing.T) {
	cmd := NewCommand()

	pause, _, err := cmd.Find([]string{"pause"})
	require.NoError(t, err)
	assert.Equal(t, "suspend", pause.Name())

	resume, _, err := cmd.Find([]string{"resume"})
	require.NoError(t, err)
	assert.Equal(t, "unsuspend", resume.Name())

	unpause, _, err := cmd.Find([]string{"unpause"})
	require.NoError(t, err)
	assert.Equal(t, "unsuspend", unpause.Name())
}

// unpause-domain ----------------------------------------------------------------

const testUnpauseSubAccountID = "11111111-1111-1111-1111-111111111111"

// newTestUnpausedDomain is the domain the API returns once it is unpaused.
func newTestUnpausedDomain() *responses.Domain {
	return &responses.Domain{
		Object:      "domain",
		ID:          uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Domain:      "example.com",
		DNSValid:    true,
		SendingType: common.DomainSendingTypeMarketing,
	}
}

// TestUnpauseDomainCommand_TakesNoIdempotencyKey checks that unpause-domain
// has no --idempotency-key: the API takes no Idempotency-Key for an unpause.
func TestUnpauseDomainCommand_TakesNoIdempotencyKey(t *testing.T) {
	assert.Nil(t, NewUnpauseDomainCommand().Flags().Lookup("idempotency-key"))
}

func TestUnpauseDomainCommand_SendsRequest(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UnpauseSubAccountDomain", uuid.MustParse(testUnpauseSubAccountID), "example.com").
		Return(newTestUnpausedDomain(), nil)
	called := testutil.InjectClient(t, mockClient)

	_, err := testutil.RunCommandCombined(t, NewUnpauseDomainCommand(), "plain", testUnpauseSubAccountID, "example.com")
	require.NoError(t, err)
	assert.True(t, *called)
	mockClient.AssertExpectations(t)
}

func TestUnpauseDomainCommand_BadUUIDFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUnpauseDomainCommand(), "plain", "not-a-uuid", "example.com")
	require.Error(t, err)
	assert.Equal(t, `invalid sub-account ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *called, "auth resolver must not be reached for malformed UUID")
}

func TestUnpauseDomainCommand_EmptyDomainFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUnpauseDomainCommand(), "plain", testUnpauseSubAccountID, "")
	require.Error(t, err)
	assert.Equal(t, "domain cannot be empty", err.Error())
	assert.False(t, *called, "auth resolver must not be reached for an empty domain")
}

func TestUnpauseDomainCommand_DotDomainFailsBeforeAuth(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewUnpauseDomainCommand(), "plain", testUnpauseSubAccountID, "..")
	require.Error(t, err)
	assert.Equal(t, `invalid domain value "..": not a domain`, err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *called)
}

func TestUnpauseDomainCommand_TableOutput(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UnpauseSubAccountDomain", uuid.MustParse(testUnpauseSubAccountID), "example.com").
		Return(newTestUnpausedDomain(), nil)
	testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewUnpauseDomainCommand(), "table", testUnpauseSubAccountID, "example.com")
	require.NoError(t, err)
	assert.Contains(t, out, "Domain unpaused successfully")
	assert.Contains(t, out, "Sending Type")
	assert.Contains(t, out, "marketing")
	assert.NotContains(t, out, "Pause Reason")
	mockClient.AssertExpectations(t)
}

func TestUnpauseDomainCommand_JSONIsTheDomain(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UnpauseSubAccountDomain", uuid.MustParse(testUnpauseSubAccountID), "example.com").
		Return(newTestUnpausedDomain(), nil)
	testutil.InjectClient(t, mockClient)

	out, err := testutil.RunCommandCombined(t, NewUnpauseDomainCommand(), "json", testUnpauseSubAccountID, "example.com")
	require.NoError(t, err)
	assert.Contains(t, out, `"sending_type": "marketing"`)
	assert.Contains(t, out, `"paused": false`)
	assert.NotContains(t, out, `"pause_reason"`)
	mockClient.AssertExpectations(t)
}

func TestDeleteDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", "11111111-1111-1111-1111-111111111111")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete sub-account 11111111-1111-1111-1111-111111111111? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestSuspendDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewSuspendCommand(), "plain", "n\n", "11111111-1111-1111-1111-111111111111", "--reason", "overdue")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to suspend sub-account 11111111-1111-1111-1111-111111111111? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestOverlongNameIsAValidationError(t *testing.T) {
	longName := strings.Repeat("a", 256)
	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"create", NewCreateCommand(), []string{"--name", longName, "--website", "example.com"}},
		{"update", NewUpdateCommand(), []string{"11111111-1111-1111-1111-111111111111", "--name", longName}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			_, _, err := testutil.RunCommand(t, tt.cmd, "plain", "", tt.args...)
			require.Error(t, err)
			assert.Equal(t, "--name must be at most 255 characters", err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *authCalled)
		})
	}
}
