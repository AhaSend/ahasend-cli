package integration

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/AhaSend/ahasend-cli/cmd"
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
)

// Fixed UUIDs shared across the sub-account integration scenarios.
const (
	intParentID     = "00000000-0000-0000-0000-0000000000aa"
	intSubAccountID = "11111111-1111-1111-1111-111111111111"
	intKeyID        = "44444444-4444-4444-4444-444444444444"
	intScope        = "messages:send:all"
)

// SubAccountsIntegrationTestSuite exercises the sub-account provisioning surface
// end-to-end through real Cobra execution (cmd.NewRootCmdForTesting) against a
// MockClient wired in via the auth resolver seam. The suite intentionally drives
// commands rather than calling mock methods directly so the command wiring,
// validation-before-auth ordering, and SDK JSON pass-through are all covered.
type SubAccountsIntegrationTestSuite struct {
	suite.Suite
	mockClient *mocks.MockClient
}

func (suite *SubAccountsIntegrationTestSuite) SetupTest() {
	suite.mockClient = &mocks.MockClient{}
}

func (suite *SubAccountsIntegrationTestSuite) TearDownTest() {
	suite.mockClient = nil
}

// installResolver swaps in a test auth resolver returning the suite mock client
// and records whether it was reached. The returned *bool lets tests assert the
// validation-before-auth contract. The resolver is restored on test cleanup.
func (suite *SubAccountsIntegrationTestSuite) installResolver(c client.AhaSendClient) *bool {
	called := false
	restore := auth.SetAuthenticatedClientResolverForTesting(func(*cobra.Command) (client.AhaSendClient, error) {
		called = true
		return c, nil
	})
	suite.T().Cleanup(restore)
	return &called
}

// execRoot runs a fresh root command for the given output format and captures
// stdout/stderr separately. Errors surfaced by command execution are handled by
// the root's JSON-aware error wiring, so callers assert on output content.
//
// The package-private globalExitCode is reset before execution so that, after
// execRoot returns, cmd.GlobalExitCodeForTesting() reflects only this run's
// outcome (handleError sets it only on error, so a successful command would
// otherwise leave a prior run's value in place). Callers asserting the
// exit-code contract read it via cmd.GlobalExitCodeForTesting().
func (suite *SubAccountsIntegrationTestSuite) execRoot(format string, args ...string) (string, string, error) {
	root := cmd.NewRootCmdForTesting()

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	full := append([]string{}, args...)
	full = append(full, "--output", format)
	root.SetArgs(full)

	cmd.ResetGlobalExitCodeForTesting()
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// Fixtures ---------------------------------------------------------------------

func fixedSubAccount() *responses.SubAccount {
	last := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	return &responses.SubAccount{
		Object:          "sub_account",
		ID:              uuid.MustParse(intSubAccountID),
		ParentAccountID: uuid.MustParse(intParentID),
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Name:            "Acme Inc",
		Website:         "https://acme.example",
		Status:          "active",
		MonthlyCredit:   5000,
		DomainCount:     2,
		MemberCount:     3,
		LastActivityAt:  &last,
	}
}

func fixedSubAccountsList() *responses.PaginatedSubAccountsResponse {
	return &responses.PaginatedSubAccountsResponse{
		Object: "list",
		Data:   []responses.SubAccount{*fixedSubAccount()},
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}
}

func fixedAPIKey(withSecret bool) *responses.APIKey {
	key := &responses.APIKey{
		Object:    "api_key",
		ID:        uuid.MustParse(intKeyID),
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		AccountID: uuid.MustParse(intSubAccountID),
		Label:     "Production API",
		PublicKey: "aha-pk-test",
		Scopes: []responses.APIKeyScope{
			{
				ID:        uuid.MustParse("55555555-5555-5555-5555-555555555555"),
				CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
				APIKeyID:  uuid.MustParse(intKeyID),
				Scope:     intScope,
			},
		},
	}
	if withSecret {
		secret := "sk_one_time_secret_value"
		key.SecretKey = &secret
	}
	return key
}

func fixedUsage() *responses.SubAccountUsageResponse {
	parent := uuid.MustParse(intParentID)
	sub := uuid.MustParse(intSubAccountID)
	name := "Acme Inc"
	return &responses.SubAccountUsageResponse{
		BillingPeriod: responses.SubAccountUsageBillingPeriod{
			Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		Currency:         "usd",
		AllocationMethod: "proportional",
		Parent: responses.SubAccountUsageBreakdown{
			AccountID:      &parent,
			ReceptionCount: 1000,
			AllocatedCost:  10,
		},
		SubAccounts: []responses.SubAccountUsageBreakdown{
			{
				AccountID:      &sub,
				Name:           &name,
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

// Workflow ---------------------------------------------------------------------

// TestProvisioningWorkflow drives the full provisioning lifecycle through real
// Cobra execution: create sub-account -> create sub-account API key -> get key
// -> delete key -> delete sub-account, all against a single MockClient.
func (suite *SubAccountsIntegrationTestSuite) TestProvisioningWorkflow() {
	t := suite.T()

	subAccount := fixedSubAccount()
	apiKey := fixedAPIKey(true)

	suite.mockClient.On("CreateSubAccount",
		mock.AnythingOfType("requests.CreateSubAccountRequest"),
		mock.AnythingOfType("string")).
		Return(subAccount, nil)
	suite.mockClient.On("CreateSubAccountAPIKey", intSubAccountID,
		requests.CreateAPIKeyRequest{Label: "Production API", Scopes: []string{intScope}},
		mock.AnythingOfType("string")).
		Return(apiKey, nil)
	suite.mockClient.On("GetSubAccountAPIKey", intSubAccountID, intKeyID).
		Return(fixedAPIKey(false), nil)
	suite.mockClient.On("DeleteSubAccountAPIKey", intSubAccountID, intKeyID).
		Return(&common.SuccessResponse{}, nil)
	suite.mockClient.On("DeleteSubAccount", intSubAccountID).
		Return(&common.SuccessResponse{}, nil)

	called := suite.installResolver(suite.mockClient)

	// 1. Create the sub-account.
	out, _, err := suite.execRoot("plain", "subaccounts", "create",
		"--name", "Acme Inc", "--website", "https://acme.example")
	suite.NoError(err)
	suite.Contains(out, "Acme Inc")

	// 2. Create an API key under the sub-account; the one-time secret and the
	//    replay-window note render for plain output.
	out, _, err = suite.execRoot("plain", "subaccounts", "api-keys", "create",
		intSubAccountID, "--label", "Production API", "--scope", intScope)
	suite.NoError(err)
	suite.Contains(out, "sk_one_time_secret_value")
	suite.Contains(out, "5-minute replay window")

	// 3. Get the API key (secret never returned on read).
	out, _, err = suite.execRoot("plain", "subaccounts", "api-keys", "get",
		intSubAccountID, intKeyID)
	suite.NoError(err)
	suite.Contains(out, "Production API")
	suite.NotContains(out, "sk_one_time_secret_value")

	// 4. Delete the API key.
	out, _, err = suite.execRoot("plain", "subaccounts", "api-keys", "delete",
		intSubAccountID, intKeyID, "--force")
	suite.NoError(err)
	suite.Contains(out, "Deleted")

	// 5. Delete the sub-account.
	out, _, err = suite.execRoot("plain", "subaccounts", "delete",
		intSubAccountID, "--force")
	suite.NoError(err)
	suite.Contains(out, "deleted successfully")

	suite.True(*called, "auth resolver must be reached for each authenticated step")
	suite.mockClient.AssertExpectations(t)
}

// Malformed-ID negative paths --------------------------------------------------

// TestMalformedIDsFailBeforeAuth verifies field-specific validation errors are
// raised before authentication for malformed positional UUIDs across the
// nested sub-account API-key commands.
func (suite *SubAccountsIntegrationTestSuite) TestMalformedIDsFailBeforeAuth() {
	cases := []struct {
		name    string
		args    []string
		message string
		method  string
	}{
		{
			name:    "get bad sub-account ID",
			args:    []string{"subaccounts", "api-keys", "get", "not-a-uuid", intKeyID},
			message: "invalid sub-account ID format: not-a-uuid",
			method:  "GetSubAccountAPIKey",
		},
		{
			name:    "get bad key ID",
			args:    []string{"subaccounts", "api-keys", "get", intSubAccountID, "not-a-uuid"},
			message: "invalid API key ID format: not-a-uuid",
			method:  "GetSubAccountAPIKey",
		},
		{
			name:    "create bad sub-account ID",
			args:    []string{"subaccounts", "api-keys", "create", "not-a-uuid", "--label", "CI", "--scope", intScope},
			message: "invalid sub-account ID format: not-a-uuid",
			method:  "CreateSubAccountAPIKey",
		},
		{
			name:    "delete bad key ID",
			args:    []string{"subaccounts", "api-keys", "delete", intSubAccountID, "not-a-uuid", "--force"},
			message: "invalid API key ID format: not-a-uuid",
			method:  "DeleteSubAccountAPIKey",
		},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			mockClient := &mocks.MockClient{}
			called := suite.installResolver(mockClient)

			out, _, err := suite.execRoot("plain", tc.args...)
			suite.NoError(err) // root handles the error internally; content is in output
			suite.Contains(out, tc.message)
			suite.False(*called, "auth resolver must not be reached for malformed IDs")
			mockClient.AssertNotCalled(suite.T(), tc.method, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// JSON round-tripping ----------------------------------------------------------

// assertNoWrapperKeys fails if the JSON object carries CLI-added envelope keys
// (the empty/success wrappers the human handlers emit) instead of a verbatim
// SDK response.
func (suite *SubAccountsIntegrationTestSuite) assertNoWrapperKeys(out string) {
	var raw map[string]json.RawMessage
	suite.Require().NoError(json.Unmarshal([]byte(out), &raw))
	for _, k := range []string{"empty", "success"} {
		_, present := raw[k]
		suite.False(present, "JSON output must not carry CLI wrapper key %q", k)
	}
}

// TestJSONRoundTrip_List confirms `subaccounts list` JSON round-trips into the
// SDK PaginatedSubAccountsResponse with no CLI wrapper keys.
func (suite *SubAccountsIntegrationTestSuite) TestJSONRoundTrip_List() {
	fixture := fixedSubAccountsList()
	suite.mockClient.On("ListSubAccounts", (*int32)(nil), (*string)(nil)).Return(fixture, nil)
	suite.installResolver(suite.mockClient)

	out, _, err := suite.execRoot("json", "subaccounts", "list")
	suite.NoError(err)

	var got responses.PaginatedSubAccountsResponse
	suite.Require().NoError(json.Unmarshal([]byte(out), &got))
	suite.Equal(*fixture, got)
	suite.assertNoWrapperKeys(out)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestJSONRoundTrip_Single confirms `subaccounts get` JSON round-trips into the
// SDK SubAccount struct verbatim.
func (suite *SubAccountsIntegrationTestSuite) TestJSONRoundTrip_Single() {
	fixture := fixedSubAccount()
	suite.mockClient.On("GetSubAccount", intSubAccountID).Return(fixture, nil)
	suite.installResolver(suite.mockClient)

	out, _, err := suite.execRoot("json", "subaccounts", "get", intSubAccountID)
	suite.NoError(err)

	var got responses.SubAccount
	suite.Require().NoError(json.Unmarshal([]byte(out), &got))
	suite.Equal(*fixture, got)
	suite.assertNoWrapperKeys(out)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestJSONRoundTrip_CreateSubAccount confirms `subaccounts create` JSON
// round-trips into the SDK SubAccount struct verbatim, with no CLI wrapper keys
// around the freshly provisioned record.
func (suite *SubAccountsIntegrationTestSuite) TestJSONRoundTrip_CreateSubAccount() {
	fixture := fixedSubAccount()
	suite.mockClient.On("CreateSubAccount",
		mock.AnythingOfType("requests.CreateSubAccountRequest"),
		mock.AnythingOfType("string")).
		Return(fixture, nil)
	suite.installResolver(suite.mockClient)

	out, _, err := suite.execRoot("json", "subaccounts", "create",
		"--name", "Acme Inc", "--website", "https://acme.example")
	suite.NoError(err)

	var got responses.SubAccount
	suite.Require().NoError(json.Unmarshal([]byte(out), &got))
	suite.Equal(*fixture, got)
	suite.assertNoWrapperKeys(out)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestJSONRoundTrip_Create confirms `subaccounts api-keys create` JSON round-trips
// into the SDK APIKey struct, preserving the one-time secret_key and emitting no
// human-readable replay note.
func (suite *SubAccountsIntegrationTestSuite) TestJSONRoundTrip_Create() {
	fixture := fixedAPIKey(true)
	suite.mockClient.On("CreateSubAccountAPIKey", intSubAccountID,
		requests.CreateAPIKeyRequest{Label: "Production API", Scopes: []string{intScope}},
		mock.AnythingOfType("string")).
		Return(fixture, nil)
	suite.installResolver(suite.mockClient)

	out, _, err := suite.execRoot("json", "subaccounts", "api-keys", "create",
		intSubAccountID, "--label", "Production API", "--scope", intScope)
	suite.NoError(err)

	var got responses.APIKey
	suite.Require().NoError(json.Unmarshal([]byte(out), &got))
	suite.Equal(*fixture, got)
	suite.NotContains(out, "replay window")
	suite.assertNoWrapperKeys(out)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestJSONRoundTrip_Usage confirms `subaccounts usage` JSON round-trips into the
// SDK SubAccountUsageResponse.
func (suite *SubAccountsIntegrationTestSuite) TestJSONRoundTrip_Usage() {
	fixture := fixedUsage()
	suite.mockClient.On("GetSubAccountsUsage").Return(fixture, nil)
	suite.installResolver(suite.mockClient)

	out, _, err := suite.execRoot("json", "subaccounts", "usage")
	suite.NoError(err)

	var got responses.SubAccountUsageResponse
	suite.Require().NoError(json.Unmarshal([]byte(out), &got))
	suite.Equal(*fixture, got)
	suite.assertNoWrapperKeys(out)
	suite.mockClient.AssertExpectations(suite.T())
}

// Raw 409/422 API errors -------------------------------------------------------

// rawCreateAPIError installs a mock whose nested API-key create fails with the
// given raw SDK API error and runs the create command through the real Cobra
// root path for the requested output format. It returns the captured stdout and
// the exit code recorded by the root error path, and the wrapping mock for
// assertions.
//
// The root's error wiring (cmd.handleError) discriminates JSON raw API errors
// (pass-through: raw body printed verbatim, globalExitCode left at 0) from
// human-format errors (rendered "Error:" message, nonzero globalExitCode) on a
// single branch. execRoot resets globalExitCode before running, so the value
// read here via cmd.GlobalExitCodeForTesting() is exactly the code this command
// produced. The same integer is also asserted at the cmd-package level in
// cmd/root_error_test.go.
func (suite *SubAccountsIntegrationTestSuite) rawCreateAPIError(format string, apiErr *api.APIError) (string, int, *mocks.MockClient) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", intSubAccountID,
		mock.AnythingOfType("requests.CreateAPIKeyRequest"),
		mock.AnythingOfType("string")).
		Return(nil, apiErr)
	suite.installResolver(mockClient)

	out, _, err := suite.execRoot(format, "subaccounts", "api-keys", "create",
		intSubAccountID, "--label", "CI", "--scope", intScope)
	suite.NoError(err) // root swallows the error after formatting it for output
	return out, cmd.GlobalExitCodeForTesting(), mockClient
}

// TestRawAPIError_PassThroughBehavior verifies, end-to-end through real Cobra
// execution, that nested-create raw 409/422 SDK API errors take the expected
// branch of the root error path:
//   - JSON mode prints the raw body verbatim, adds no human-readable "Error:"
//     wrapper, and leaves globalExitCode == 0 (the pass-through contract).
//   - table/plain modes render a human "Error:" message and leave a nonzero
//     globalExitCode instead.
func (suite *SubAccountsIntegrationTestSuite) TestRawAPIError_PassThroughBehavior() {
	cases := []struct {
		name       string
		statusCode int
		raw        string
	}{
		{"conflict", 409, `{"error":"idempotency_conflict","message":"idempotency key is already in use"}`},
		{"idempotency_unprocessable", 422, `{"error":"validation_failed","message":"request body is invalid"}`},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			newErr := func() *api.APIError {
				return &api.APIError{
					StatusCode: tc.statusCode,
					Message:    "api request failed",
					Code:       "api_error",
					Raw:        []byte(tc.raw),
				}
			}

			// JSON mode: raw body printed verbatim, no human error wrapper, and
			// the exit code left at 0 — exactly the pass-through branch.
			out, jsonExit, jsonMock := suite.rawCreateAPIError("json", newErr())
			suite.JSONEq(tc.raw, out)
			suite.NotContains(out, "Error:",
				"JSON raw API error must pass the body through, not wrap it")
			suite.Equal(0, jsonExit,
				"JSON raw API error must leave globalExitCode == 0 (pass-through)")
			jsonMock.AssertExpectations(suite.T())

			// table/plain modes: human error rendered and a nonzero exit code.
			for _, format := range []string{"table", "plain"} {
				out, humanExit, humanMock := suite.rawCreateAPIError(format, newErr())
				suite.Contains(out, "Error:",
					"%s raw API error must render a human error message", format)
				suite.NotZero(humanExit,
					"%s raw API error must leave a nonzero globalExitCode", format)
				humanMock.AssertExpectations(suite.T())
			}
		})
	}
}

// Run the sub-accounts integration suite.
func TestSubAccountsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SubAccountsIntegrationTestSuite))
}
