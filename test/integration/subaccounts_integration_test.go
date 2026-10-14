package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
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
	commandSuite
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
		Website:         "acme.example.com",
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
	suite.mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(intSubAccountID),
		requests.CreateAPIKeyRequest{Label: "Production API", Scopes: []string{intScope}},
		mock.AnythingOfType("string")).
		Return(apiKey, nil)
	suite.mockClient.On("GetSubAccountAPIKey", uuid.MustParse(intSubAccountID), uuid.MustParse(intKeyID)).
		Return(fixedAPIKey(false), nil)
	suite.mockClient.On("DeleteSubAccountAPIKey", uuid.MustParse(intSubAccountID), uuid.MustParse(intKeyID)).
		Return(&common.SuccessResponse{}, nil)
	suite.mockClient.On("DeleteSubAccount", uuid.MustParse(intSubAccountID)).
		Return(&common.SuccessResponse{}, nil)

	called := testutil.InjectClient(suite.T(), suite.mockClient)

	// 1. Create the sub-account.
	out, _, code := execRoot("plain", "", "subaccounts", "create",
		"--name", "Acme Inc", "--website", "acme.example.com")
	suite.Zero(code)
	suite.Contains(out, "Acme Inc")

	// 2. Create an API key under the sub-account; the one-time secret and the
	//    replay-window note render for plain output.
	out, _, code = execRoot("plain", "", "subaccounts", "api-keys", "create",
		intSubAccountID, "--label", "Production API", "--scope", intScope)
	suite.Zero(code)
	suite.Contains(out, "sk_one_time_secret_value")
	suite.Contains(out, "5-minute replay window")

	// 3. Get the API key (secret never returned on read).
	out, _, code = execRoot("plain", "", "subaccounts", "api-keys", "get",
		intSubAccountID, intKeyID)
	suite.Zero(code)
	suite.Contains(out, "Production API")
	suite.NotContains(out, "sk_one_time_secret_value")

	// 4. Delete the API key.
	out, _, code = execRoot("plain", "", "subaccounts", "api-keys", "delete",
		intSubAccountID, intKeyID, "--force")
	suite.Zero(code)
	suite.Contains(out, "API key '"+intKeyID+"' deleted successfully")

	// 5. Delete the sub-account.
	out, _, code = execRoot("plain", "", "subaccounts", "delete",
		intSubAccountID, "--force")
	suite.Zero(code)
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
			message: `invalid sub-account ID value "not-a-uuid": not a UUID`,
			method:  "GetSubAccountAPIKey",
		},
		{
			name:    "get bad key ID",
			args:    []string{"subaccounts", "api-keys", "get", intSubAccountID, "not-a-uuid"},
			message: `invalid API key ID value "not-a-uuid": not a UUID`,
			method:  "GetSubAccountAPIKey",
		},
		{
			name:    "create bad sub-account ID",
			args:    []string{"subaccounts", "api-keys", "create", "not-a-uuid", "--label", "CI", "--scope", intScope},
			message: `invalid sub-account ID value "not-a-uuid": not a UUID`,
			method:  "CreateSubAccountAPIKey",
		},
		{
			name:    "delete bad key ID",
			args:    []string{"subaccounts", "api-keys", "delete", intSubAccountID, "not-a-uuid", "--force"},
			message: `invalid API key ID value "not-a-uuid": not a UUID`,
			method:  "DeleteSubAccountAPIKey",
		},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			mockClient := &mocks.MockClient{}
			called := testutil.InjectClient(suite.T(), mockClient)

			out, stderr, code := execRoot("plain", "", tc.args...)
			suite.Equal(4, code)
			suite.Empty(out)
			suite.Contains(stderr, tc.message)
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
	suite.mockClient.On("ListSubAccounts", common.PaginationParams{}).Return(fixture, nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("json", "", "subaccounts", "list")
	suite.Zero(code)

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
	suite.mockClient.On("GetSubAccount", uuid.MustParse(intSubAccountID)).Return(fixture, nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("json", "", "subaccounts", "get", intSubAccountID)
	suite.Zero(code)

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
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("json", "", "subaccounts", "create",
		"--name", "Acme Inc", "--website", "acme.example.com")
	suite.Zero(code)

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
	suite.mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(intSubAccountID),
		requests.CreateAPIKeyRequest{Label: "Production API", Scopes: []string{intScope}},
		mock.AnythingOfType("string")).
		Return(fixture, nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("json", "", "subaccounts", "api-keys", "create",
		intSubAccountID, "--label", "Production API", "--scope", intScope)
	suite.Zero(code)

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
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("json", "", "subaccounts", "usage")
	suite.Zero(code)

	var got responses.SubAccountUsageResponse
	suite.Require().NoError(json.Unmarshal([]byte(out), &got))
	suite.Equal(*fixture, got)
	suite.assertNoWrapperKeys(out)
	suite.mockClient.AssertExpectations(suite.T())
}

// Raw 409/422 API errors -------------------------------------------------------

// rawCreateAPIError installs a mock whose nested API-key create fails with the
// given raw SDK API error and runs the create command through the real Cobra
// root path for the requested output format. It returns the captured stdout,
// stderr and exit code, and the wrapping mock for assertions.
//
// The root's error wiring (cmd.handleError) prints JSON raw API errors
// verbatim and renders an "Error:" message in the other formats; every format
// exits nonzero.
func (suite *SubAccountsIntegrationTestSuite) rawCreateAPIError(format string, apiErr *api.APIError) (string, string, int, *mocks.MockClient) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateSubAccountAPIKey", uuid.MustParse(intSubAccountID),
		mock.AnythingOfType("requests.CreateAPIKeyRequest"),
		mock.AnythingOfType("string")).
		Return(nil, apiErr)
	testutil.InjectClient(suite.T(), mockClient)

	out, stderr, code := execRoot(format, "", "subaccounts", "api-keys", "create",
		intSubAccountID, "--label", "CI", "--scope", intScope)
	return out, stderr, code, mockClient
}

// TestRawAPIError_PassThroughBehavior verifies, end-to-end through real Cobra
// execution, that nested-create raw 409/422 SDK API errors take the expected
// branch of the root error path:
//   - JSON mode prints the raw body verbatim and adds no human-readable
//     "Error:" wrapper.
//   - table/plain modes render a human "Error:" message.
//   - Every mode exits nonzero.
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

			// JSON mode: raw body printed verbatim, no human error wrapper, and a
			// nonzero exit code.
			out, _, jsonExit, jsonMock := suite.rawCreateAPIError("json", newErr())
			suite.JSONEq(tc.raw, out)
			suite.NotContains(out, "Error:",
				"JSON raw API error must pass the body through, not wrap it")
			suite.NotZero(jsonExit,
				"JSON raw API error must exit nonzero")
			jsonMock.AssertExpectations(suite.T())

			// table/plain modes: human error rendered and a nonzero exit code.
			for _, format := range []string{"table", "plain"} {
				out, stderr, humanExit, humanMock := suite.rawCreateAPIError(format, newErr())
				suite.Empty(out)
				suite.Contains(stderr, "Error:",
					"%s raw API error must render a human error message", format)
				suite.NotZero(humanExit,
					"%s raw API error must exit nonzero", format)
				humanMock.AssertExpectations(suite.T())
			}
		})
	}
}

// Run the sub-accounts integration suite.
func TestSubAccountsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SubAccountsIntegrationTestSuite))
}
