package segments

import (
	"slices"
	"strings"
	"testing"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testSegmentID = "33333333-3333-4333-8333-333333333333"

// testDefinitionJSON is a definition file with a condition, a group and a
// count, the shape of the definition the help shows.
const testDefinitionJSON = `{
  "schema_version": 1,
  "match": "all",
  "conditions": [
    {"field": "attributes.plan", "op": "equal", "value": "pro"},
    {"match": "any", "conditions": [
      {"field": "attributes.country", "op": "equal", "value": "DE"},
      {"field": "campaign_count", "op": "opened_at_least",
       "value": {"count": 3, "since": "90 days ago"}}
    ]}
  ]
}`

// testDefinition is testDefinitionJSON decoded.
func testDefinition() common.SegmentDefinition {
	plan, country := "pro", "DE"
	return common.SegmentDefinition{
		SchemaVersion: 1,
		Match:         "all",
		Conditions: []common.SegmentRule{
			{Condition: &common.SegmentCondition{Field: "attributes.plan", Op: "equal", Value: &common.SegmentValue{String: &plan}}},
			{Group: &common.SegmentGroup{Match: "any", Conditions: []common.SegmentCondition{
				{Field: "attributes.country", Op: "equal", Value: &common.SegmentValue{String: &country}},
				{Field: "campaign_count", Op: "opened_at_least", Value: &common.SegmentValue{
					Count: &common.SegmentCampaignCount{Count: 3, Since: "90 days ago"},
				}},
			}}},
		},
	}
}

// definitionFile writes testDefinitionJSON to a file and returns its path.
func definitionFile(t *testing.T) string {
	return testutil.WriteTempFile(t, "engaged.json", testDefinitionJSON)
}

// segmentBody is an API body of a building segment.
const segmentBody = `{"object":"segment","id":"` + testSegmentID + `","name":"Engaged pro customers",` +
	`"description":"","state":"building","failure_reason":null,"member_count":0,"timezone":"UTC",` +
	`"last_evaluated_at":null,"created_at":"2026-09-10T08:30:00Z","updated_at":"2026-09-10T08:30:00Z",` +
	`"definition":{"schema_version":1,"match":"all","conditions":[{"field":"attributes.plan","op":"equal","value":"pro"}]}}`

// TestEveryCommandCallsItsMethodAndPrintsTheAnswer runs each segments
// command with flags that reach its request, checks the SDK call it makes,
// and prints the answer in every format: JSON is the API's body, and the
// other formats show the answer.
func TestEveryCommandCallsItsMethodAndPrintsTheAnswer(t *testing.T) {
	limit, after, before := int32(5), "next-page", "previous-page"
	name, description, timezone := "Engaged pro customers", "Pro plan, opened lately", "Europe/Berlin"
	segmentID := uuid.MustParse(testSegmentID)
	updated := testDefinition()
	previewBody := `{"object":"segment_preview","count":null,"contacts":[` +
		`{"contact_id":"11111111-1111-4111-8111-111111111111","email":"a@example.com"}]}`
	membersBody := `{"object":"list","data":[{"object":"segment_contact","segment_id":"` + testSegmentID +
		`","contact_id":"11111111-1111-4111-8111-111111111111","email":"a@example.com","entered_at":"2026-09-10T09:00:00Z"}],` +
		`"pagination":{"has_more":false}}`
	tests := []struct {
		name    string
		command func() *cobra.Command
		args    []string
		method  string
		calls   []any
		body    string
		result  func(*testing.T, string) any
		shows   string
	}{
		{"list", NewListCommand, []string{"--limit", "5", "--after", after}, "GetSegments",
			[]any{requests.GetSegmentsParams{Limit: &limit, After: &after}},
			`{"object":"list","data":[` + segmentBody + `],"pagination":{"has_more":true,"next_cursor":"n"}}`,
			func(t *testing.T, body string) any {
				return mocks.APIResponse[responses.PaginatedSegmentsResponse](t, body)
			},
			"Engaged pro customers"},
		{"get", NewGetCommand, []string{testSegmentID}, "GetSegment", []any{segmentID},
			strings.Replace(segmentBody, `"state"`, `"marketable_count":null,"state"`, 1),
			func(t *testing.T, body string) any { return mocks.APIResponse[responses.SegmentDetail](t, body) },
			"Engaged pro customers"},
		{"create", NewCreateCommand, []string{"--name", name, "--description", description, "--timezone", timezone,
			"--definition-file", "<definition>", "--idempotency-key", "create-engaged"}, "CreateSegment",
			[]any{requests.CreateSegmentRequest{Name: name, Description: &description, Definition: testDefinition(), Timezone: &timezone},
				testutil.IdempotencyKeyArg("create-engaged")},
			segmentBody,
			func(t *testing.T, body string) any { return mocks.APIResponse[responses.Segment](t, body) },
			"building"},
		{"update", NewUpdateCommand, []string{testSegmentID, "--name", name, "--definition-file", "<definition>"}, "UpdateSegment",
			[]any{segmentID, requests.UpdateSegmentRequest{Name: &name, Definition: &updated}},
			segmentBody,
			func(t *testing.T, body string) any { return mocks.APIResponse[responses.Segment](t, body) },
			"Engaged pro customers"},
		{"delete", NewDeleteCommand, []string{testSegmentID, "--force"}, "DeleteSegment", []any{segmentID},
			`{"message":"Segment deleted"}`,
			func(t *testing.T, body string) any { return mocks.APIResponse[common.SuccessResponse](t, body) },
			"Segment deleted"},
		{"preview", NewPreviewCommand, []string{"--definition-file", "<definition>", "--timezone", timezone}, "PreviewSegment",
			[]any{requests.PreviewSegmentRequest{Definition: testDefinition(), Timezone: &timezone}},
			previewBody,
			func(t *testing.T, body string) any { return mocks.APIResponse[responses.SegmentPreview](t, body) },
			"a@example.com"},
		{"members", NewMembersCommand, []string{testSegmentID, "--before", before}, "GetSegmentContacts",
			[]any{segmentID, requests.GetSegmentContactsParams{Before: &before}},
			membersBody,
			func(t *testing.T, body string) any {
				return mocks.APIResponse[responses.PaginatedSegmentContactsResponse](t, body)
			},
			"a@example.com"},
	}
	for _, test := range tests {
		for _, format := range []string{"table", "plain", "csv", "json"} {
			t.Run(test.name+" "+format, func(t *testing.T) {
				args := make([]string, len(test.args))
				for i, arg := range test.args {
					if arg == "<definition>" {
						arg = definitionFile(t)
					}
					args[i] = arg
				}
				mockClient := &mocks.MockClient{}
				mockClient.On(test.method, test.calls...).Return(test.result(t, test.body), nil).Once()
				testutil.InjectClient(t, mockClient)

				stdout, stderr, err := testutil.RunCommand(t, test.command(), format, "", args...)

				require.NoError(t, err, stderr)
				if format == "json" {
					assert.Equal(t, testutil.IndentedJSON(t, test.body), stdout)
				} else {
					assert.Contains(t, stdout, test.shows)
				}
				mockClient.AssertExpectations(t)
			})
		}
	}
}

func TestListValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--after", "a", "--before", "b"}, "--after and --before cannot be used together"},
		{[]string{"--limit", "101"}, "--limit must be between 0 and 100"},
	}
	for _, command := range []func() *cobra.Command{NewListCommand, NewMembersCommand} {
		for _, test := range tests {
			t.Run(strings.Join(test.args, " "), func(t *testing.T) {
				authCalled := testutil.RefuseAuth(t)
				cmd := command()
				args := test.args
				if cmd.Name() == "members" {
					args = append([]string{testSegmentID}, args...)
				}
				_, err := testutil.RunPlain(t, cmd, args...)
				require.Error(t, err)
				assert.Equal(t, test.message, err.Error())
				assert.False(t, *authCalled)
			})
		}
	}
}

// get -------------------------------------------------------------------------

func TestGetWithoutAMarketableCountPrintsUnknown(t *testing.T) {
	detail := &responses.SegmentDetail{Segment: responses.Segment{Name: "Engaged", State: common.SegmentStateBuilding}}
	mockClient := &mocks.MockClient{}
	mockClient.On("GetSegment", uuid.MustParse(testSegmentID)).Return(detail, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewGetCommand(), testSegmentID)

	require.NoError(t, err)
	assert.Contains(t, output, "\nMembers: 0\nMarketable: unknown\n")
	mockClient.AssertExpectations(t)
}

func TestIDsAreCheckedBeforeAuthentication(t *testing.T) {
	tests := map[string]struct {
		command func() *cobra.Command
		args    []string
	}{
		"get":     {NewGetCommand, []string{"not-a-uuid"}},
		"update":  {NewUpdateCommand, []string{"not-a-uuid", "--name", "Engaged"}},
		"delete":  {NewDeleteCommand, []string{"not-a-uuid"}},
		"members": {NewMembersCommand, []string{"not-a-uuid"}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			_, stderr, err := testutil.RunCommand(t, test.command(), "plain", "y\n", test.args...)

			require.Error(t, err)
			assert.Equal(t, `invalid segment ID value "not-a-uuid": not a UUID`, err.Error())
			assert.NotContains(t, stderr, "Are you sure")
			assert.False(t, *authCalled)
		})
	}
}

// create ----------------------------------------------------------------------

func TestCreateWithoutOptionalFlagsSendsNameAndDefinitionAlone(t *testing.T) {
	var captured requests.CreateSegmentRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "CreateSegment", &captured, &responses.Segment{Name: "Engaged"})
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Engaged", "--definition-file", definitionFile(t))

	require.NoError(t, err)
	wire := testutil.WireJSON(t, captured)
	assert.Equal(t, []string{"definition", "name"}, sortedKeys(wire))
	mockClient.AssertExpectations(t)
}

func TestCreateValidationHappensBeforeAuthentication(t *testing.T) {
	path := definitionFile(t)
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--definition-file", path}, "--name is required"},
		{[]string{"--name", " ", "--definition-file", path}, "--name cannot be empty"},
		{[]string{"--name", "Engaged"}, "--definition-file is required"},
		{[]string{"--name", "Engaged", "--definition-file", ""}, "--definition-file cannot be empty"},
		{[]string{"--name", "Engaged", "--definition-file", testutil.WriteTempFile(t, "bad.json", `{"match": 1}`)},
			"--definition-file: match must be a string"},
		{[]string{"--name", "Engaged", "--definition-file", path, "--idempotency-key", " key"},
			"--idempotency-key cannot start or end with a space, which the HTTP header would drop"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewCreateCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *authCalled)
		})
	}
}

func TestCreateMissingDefinitionFileIsAValidationError(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Engaged", "--definition-file", "/nonexistent/engaged.json")

	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "failed to read --definition-file"), err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *authCalled)
}

// update ----------------------------------------------------------------------

func TestUpdateSendsAnEmptyDescriptionAndAnEmptyTimezone(t *testing.T) {
	var captured requests.UpdateSegmentRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "UpdateSegment", &captured, &responses.Segment{Name: "Engaged"}, uuid.MustParse(testSegmentID))
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testSegmentID, "--description", "", "--timezone", "")

	require.NoError(t, err)
	assert.Equal(t, map[string]any{"description": "", "timezone": ""}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestUpdateValidationHappensBeforeAuthentication(t *testing.T) {
	atLeastOne := "at least one of --name, --description, --definition-file or --timezone must be provided"
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{testSegmentID}, atLeastOne},
		{[]string{testSegmentID, "--name", ""}, "--name cannot be empty"},
		{[]string{testSegmentID, "--definition-file", testutil.WriteTempFile(t, "empty.json", "")},
			"--definition-file is not JSON: unexpected end of JSON input"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewUpdateCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *authCalled)
		})
	}
}

// delete ----------------------------------------------------------------------

func TestDeleteDeclinedMakesNoCall(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", testSegmentID)

	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete segment "+testSegmentID)
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedPromptsOnStderrOnly(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSegment", uuid.MustParse(testSegmentID)).Return(&common.SuccessResponse{Message: "Segment deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "yes\n", testSegmentID)

	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	assert.Equal(t, "Segment deleted\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteSegment", uuid.MustParse(testSegmentID)).Return(&common.SuccessResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "", testSegmentID, "--force")

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, "Segment '"+testSegmentID+"' deleted successfully\n", stdout)
	mockClient.AssertExpectations(t)
}

// preview ---------------------------------------------------------------------

// uncountedPreview is a preview whose count the API could not give in time,
// with two sample contacts.
func uncountedPreview() *responses.SegmentPreview {
	return &responses.SegmentPreview{
		Object: "segment_preview",
		Contacts: []responses.SegmentPreviewContact{
			{ContactID: uuid.MustParse("11111111-1111-4111-8111-111111111111"), Email: "a@example.com"},
			{ContactID: uuid.MustParse("22222222-2222-4222-8222-222222222222"), Email: "b@example.com"},
		},
	}
}

func TestPreviewWithANullCountSaysItWasNotCountedInTime(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("PreviewSegment", requests.PreviewSegmentRequest{Definition: testDefinition()}).Return(uncountedPreview(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewPreviewCommand(), "--definition-file", definitionFile(t))

	require.NoError(t, err)
	assert.Equal(t, "Count: not counted in time\n"+
		"\nEmail: a@example.com\nContact ID: 11111111-1111-4111-8111-111111111111\n"+
		"\nEmail: b@example.com\nContact ID: 22222222-2222-4222-8222-222222222222\n", output)
	mockClient.AssertExpectations(t)
}

func TestPreviewCSVHasOneRowPerSampleContactOrOneWithoutThem(t *testing.T) {
	count := 0
	tests := map[string]struct {
		preview *responses.SegmentPreview
		want    string
	}{
		"contacts": {uncountedPreview(), "count,email,contact_id\n" +
			",a@example.com,11111111-1111-4111-8111-111111111111\n,b@example.com,22222222-2222-4222-8222-222222222222\n"},
		"no contacts": {&responses.SegmentPreview{Count: &count}, "count,email,contact_id\n0,,\n"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("PreviewSegment", mock.Anything).Return(test.preview, nil)
			testutil.InjectClient(t, mockClient)

			stdout, _, err := testutil.RunCommand(t, NewPreviewCommand(), "csv", "", "--definition-file", definitionFile(t))

			require.NoError(t, err)
			assert.Equal(t, test.want, stdout)
		})
	}
}

func TestPreviewWithoutADefinitionFileIsAValidationError(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewPreviewCommand(), "--timezone", "UTC")

	require.Error(t, err)
	assert.Equal(t, "--definition-file is required", err.Error())
	assert.False(t, *authCalled)
}

// sortedKeys returns the keys of m in order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
