package campaigns

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	testCampaignID = "44444444-4444-4444-8444-444444444444"
	testListID     = "55555555-5555-4555-8555-555555555555"
	testListID2    = "66666666-6666-4666-8666-666666666666"
	testTemplateID = "77777777-7777-4777-8777-777777777777"
	testSegmentID  = "88888888-8888-4888-8888-888888888888"
	testSegmentID2 = "99999999-9999-4999-8999-999999999999"
)

// testAudienceCampaign returns a campaign whose audience has a list, a
// segment, an excluded list and an excluded segment.
func testAudienceCampaign() *responses.Campaign {
	campaign := testCampaign()
	campaign.SegmentIDs = []uuid.UUID{uuid.MustParse(testSegmentID)}
	campaign.ExcludeListIDs = []uuid.UUID{uuid.MustParse(testListID2)}
	campaign.ExcludeSegmentIDs = []uuid.UUID{uuid.MustParse(testSegmentID2)}
	return campaign
}

// audienceRows are the plain lines of testAudienceCampaign's audience.
const audienceRows = "Lists: " + testListID + "\nSegments: " + testSegmentID + "\nExcluded Lists: " + testListID2 +
	"\nExcluded Segments: " + testSegmentID2 + "\n"

// audienceCSV are the CSV cells of testAudienceCampaign's audience, from
// list_ids on.
const audienceCSV = testListID + "," + testSegmentID + "," + testListID2 + "," + testSegmentID2 + ","

func testCampaign() *responses.Campaign {
	subject := "October news"
	return &responses.Campaign{
		CampaignListItem: responses.CampaignListItem{
			Object:  "campaign",
			ID:      uuid.MustParse(testCampaignID),
			Name:    "October",
			Subject: &subject,
			Status:  common.CampaignStatusDraft,
		},
		ListIDs: []uuid.UUID{uuid.MustParse(testListID)},
	}
}

func testDraftWithIssues() *responses.Campaign {
	campaign := testCampaign()
	ready := false
	campaign.Ready = &ready
	campaign.Issues = []responses.CampaignIssue{
		{Code: responses.CampaignIssueSenderMissing, Message: "Add a sender"},
		{Code: responses.CampaignIssueSubjectMissing, Message: "Add a subject"},
	}
	return campaign
}

// notReadyError returns the 400 the API answers a send or test of a campaign
// with issues.
func notReadyError() *api.APIError {
	return &api.APIError{
		StatusCode: 400,
		Message:    "campaign is not ready to send",
		Raw: []byte(`{"message":"campaign is not ready to send","issues":[` +
			`{"code":"sender_missing","message":"Add a sender"},` +
			`{"code":"lists_missing","message":"Choose at least one list"}]}`),
	}
}

const notReadySendText = "failed to send campaign: campaign is not ready to send\n" +
	"sender_missing: Add a sender\n" +
	"lists_missing: Choose at least one list"

// uncheckedError returns the 400 the API answers a send or test of a
// campaign whose text could not be checked, and that has no list.
func uncheckedError() *api.APIError {
	return &api.APIError{
		StatusCode: 400,
		Message:    "campaign is not ready to send",
		Raw: []byte(`{"message":"campaign is not ready to send","issues":[` +
			`{"code":"lists_missing","message":"Choose at least one list"},` +
			`{"code":"text_unchecked","message":"The text could not be checked. Try again in a moment."}]}`),
	}
}

// uncheckedIssues are the issue lines of uncheckedError.
const uncheckedIssues = "lists_missing: Choose at least one list\n" +
	"text_unchecked: The text could not be checked. Try again in a moment.\n"

// list ------------------------------------------------------------------------

func TestListSendsStatusAndPagination(t *testing.T) {
	status, after := common.CampaignStatusDraft, "next"
	limit := int32(5)
	mockClient := &mocks.MockClient{}
	mockClient.On("ListCampaigns", requests.ListCampaignsParams{Status: &status, Limit: &limit, After: &after}).
		Return(&responses.PaginatedCampaignsResponse{Object: "list",
			Data: []responses.CampaignListItem{testCampaign().CampaignListItem}}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewListCommand(), "--status", "draft", "--limit", "5", "--after", after)
	require.NoError(t, err)
	assert.Contains(t, output, "Name: October")
	mockClient.AssertExpectations(t)
}

func TestListWithoutFlagsSendsEmptyParams(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListCampaigns", requests.ListCampaignsParams{}).
		Return(&responses.PaginatedCampaignsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewListCommand())
	require.NoError(t, err)
	assert.Equal(t, "No campaigns found\n", output)
	mockClient.AssertExpectations(t)
}

func TestListSendsBeforeCursor(t *testing.T) {
	before := "previous"
	mockClient := &mocks.MockClient{}
	mockClient.On("ListCampaigns", requests.ListCampaignsParams{Before: &before}).
		Return(&responses.PaginatedCampaignsResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewListCommand(), "--before", before)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--after", "a", "--before", "b"}, "--after and --before cannot be used together"},
		{[]string{"--limit", "101"}, "--limit must be between 0 and 100"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewListCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

// get -------------------------------------------------------------------------

func TestGetShowsDraftIssues(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetCampaign", uuid.MustParse(testCampaignID)).Return(testDraftWithIssues(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewGetCommand(), testCampaignID)
	require.NoError(t, err)
	assert.Contains(t, output, "Ready: No\n")
	assert.Contains(t, output, "Issues: sender_missing: Add a sender, subject_missing: Add a subject\n")
	mockClient.AssertExpectations(t)
}

func TestGetShowsTheAudience(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetCampaign", uuid.MustParse(testCampaignID)).Return(testAudienceCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	plain, _, err := testutil.RunCommand(t, NewGetCommand(), "plain", "", testCampaignID)
	require.NoError(t, err)
	assert.Contains(t, plain, "\n"+audienceRows)
	csv, _, err := testutil.RunCommand(t, NewGetCommand(), "csv", "", testCampaignID)
	require.NoError(t, err)
	assert.Contains(t, csv, "list_ids,segment_ids,exclude_list_ids,exclude_segment_ids,editor,")
	assert.Contains(t, csv, ","+audienceCSV)
}

// testMarkup is the AhaML of a Simple Editor design, with the trailing
// newline the API ends it with.
const testMarkup = "<Email version=\"1\">\n  <Paragraph>October news</Paragraph>\n</Email>\n"

func TestGetMarkupWritesTheMarkupAlone(t *testing.T) {
	campaign := testCampaign()
	markup := testMarkup
	campaign.Content = &responses.TemplateContent{Markup: &markup, HTML: "<p>October news</p>"}
	mockClient := &mocks.MockClient{}
	mockClient.On("GetCampaign", uuid.MustParse(testCampaignID)).Return(campaign, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewGetCommand(), "plain", "", testCampaignID, "--markup")
	require.NoError(t, err)
	assert.Equal(t, testMarkup, stdout)
	assert.Empty(t, stderr)
	mockClient.AssertExpectations(t)
}

func TestGetMarkupFailsWithoutAhaML(t *testing.T) {
	withError := testCampaign()
	withError.Content = &responses.TemplateContent{HTML: "<p>Hi</p>",
		MarkupError: &responses.MarkupError{Code: "renderer_unavailable", Message: "try again later"}}
	html := testCampaign()
	html.Content = &responses.TemplateContent{HTML: "<p>Hi</p>"}
	tests := map[string]struct {
		campaign *responses.Campaign
		message  string
	}{
		"markup error": {withError, "the design could not be read as AhaML: renderer_unavailable: try again later"},
		"HTML design":  {html, "campaign " + testCampaignID + " has no AhaML: only a Simple Editor design has one"},
		"no content":   {testCampaign(), "campaign " + testCampaignID + " has no content"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetCampaign", uuid.MustParse(testCampaignID)).Return(test.campaign, nil)
			testutil.InjectClient(t, mockClient)

			stdout, _, err := testutil.RunCommand(t, NewGetCommand(), "plain", "", testCampaignID, "--markup")
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.Equal(t, 1, clierrors.GetExitCode(err))
			assert.Empty(t, stdout)
		})
	}
}

func TestGetRejectsBadCampaignIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewGetCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid campaign ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *authCalled)
}

// create ----------------------------------------------------------------------

func TestCreateWithNameAloneSendsOnlyTheName(t *testing.T) {
	var captured requests.CreateCampaignRequest
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateCampaign", mock.AnythingOfType("requests.CreateCampaignRequest")).
		Run(func(args mock.Arguments) {
			captured = args.Get(0).(requests.CreateCampaignRequest)
		}).Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "x")
	require.NoError(t, err)
	assert.Contains(t, output, "Campaign created successfully")
	assert.Equal(t, map[string]any{"name": "x"}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestCreateWithTwoListIDsSendsBothAndIdempotencyKey(t *testing.T) {
	mjml := "<mjml></mjml>"
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateCampaign", requests.CreateCampaignRequest{
		Name:    "October",
		ListIDs: []uuid.UUID{uuid.MustParse(testListID), uuid.MustParse(testListID2)},
		Content: &requests.TemplateContentInput{MJML: &mjml},
	}, testutil.IdempotencyKeyArg("create-key")).Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "October", "--list-id", testListID,
		"--list-id", testListID2, "--mjml-file", testutil.WriteTempFile(t, "body.mjml", mjml), "--idempotency-key", "create-key")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// TestCreateSendsSegmentsAndExclusions checks that the audience flags reach
// the request, each in its set.
func TestCreateSendsSegmentsAndExclusions(t *testing.T) {
	var captured requests.CreateCampaignRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "CreateCampaign", &captured, testCampaign())
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Win back", "--segment-id", testSegmentID,
		"--exclude-list-id", testListID, "--exclude-segment-id", testSegmentID2)

	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"name":                "Win back",
		"segment_ids":         []any{testSegmentID},
		"exclude_list_ids":    []any{testListID},
		"exclude_segment_ids": []any{testSegmentID2},
	}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

// TestAudienceLimitsCountIncludedAndExcludedTogether checks the limits of
// 50 lists and 10 segments, included and excluded together, before
// authentication, on create and update.
func TestAudienceLimitsCountIncludedAndExcludedTogether(t *testing.T) {
	ids := func(prefix string, from, count int) string {
		values := make([]string, count)
		for i := range values {
			values[i] = fmt.Sprintf("%s-0000-4000-8000-%012d", prefix, from+i)
		}
		return strings.Join(values, ",")
	}
	lists := []string{"--list-id", ids("00000000", 0, 41), "--exclude-list-id", ids("00000000", 41, 10)}
	segments := []string{"--segment-id", ids("11111111", 0, 10), "--exclude-segment-id", ids("11111111", 10, 1)}
	tests := []struct {
		args    []string
		message string
	}{
		{lists, "--list-id and --exclude-list-id take at most 50 lists in all, got 51"},
		{segments, "--segment-id and --exclude-segment-id take at most 10 segments in all, got 11"},
	}
	for _, test := range tests {
		for name, command := range map[string]func() *cobra.Command{"create": NewCreateCommand, "update": NewUpdateCommand} {
			t.Run(name+" "+test.args[0], func(t *testing.T) {
				args := append([]string{"--name", "Fall"}, test.args...)
				if name == "update" {
					args = append([]string{testCampaignID}, test.args...)
				}
				authCalled := testutil.RefuseAuth(t)

				_, err := testutil.RunPlain(t, command(), args...)

				require.Error(t, err)
				assert.Equal(t, test.message, err.Error())
				assert.Equal(t, 4, clierrors.GetExitCode(err))
				assert.False(t, *authCalled)
			})
		}
	}
}

// TestAudienceLimitsCountAnIDGivenTwiceOnce checks that the CLI counts an ID
// given to both an ID flag and its exclude flag once. The API refuses such an
// ID for being in both sets.
func TestAudienceLimitsCountAnIDGivenTwiceOnce(t *testing.T) {
	ids := make([]string, 0, 10)
	for i := range 10 {
		ids = append(ids, fmt.Sprintf("11111111-0000-4000-8000-%012d", i))
	}
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateCampaign", mock.MatchedBy(func(request requests.CreateCampaignRequest) bool {
		return len(request.SegmentIDs) == 10 && len(request.ExcludeSegmentIDs) == 1
	})).Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "October", "--segment-id", strings.Join(ids, ","),
		"--exclude-segment-id", ids[0])

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// TestListIDsCountDistinctLists checks that --list-id counts a list given
// twice once, as the API does: 50 distinct lists and a repeat are sent.
func TestListIDsCountDistinctLists(t *testing.T) {
	ids := make([]string, 0, 51)
	for i := range 50 {
		ids = append(ids, fmt.Sprintf("00000000-0000-4000-8000-%012d", i))
	}
	ids = append(ids, ids[0])
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateCampaign", mock.MatchedBy(func(request requests.CreateCampaignRequest) bool {
		return len(request.ListIDs) == 51
	})).Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "October", "--list-id", strings.Join(ids, ","))

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateSendsEveryContentAndSenderField(t *testing.T) {
	html, text := "<p>Hi</p>", "Hi"
	editor, subject, preheader, replyTo := "html", "October news", "Read me", "support@example.com"
	fromName := "Doe, Jane"
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateCampaign", requests.CreateCampaignRequest{
		Name:      "October",
		Subject:   &subject,
		Preheader: &preheader,
		From:      &common.SenderAddress{Email: "news@example.com", Name: &fromName},
		ReplyTo:   &replyTo,
		Editor:    &editor,
		Content:   &requests.TemplateContentInput{HTML: &html, Text: &text},
	}).Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "October", "--subject", subject,
		"--preheader", preheader, "--editor", editor, "--html-file", testutil.WriteTempFile(t, "body.html", html),
		"--text-file", testutil.WriteTempFile(t, "body.txt", text), "--from", "news@example.com",
		"--from-name", fromName, "--reply-to", replyTo)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateWithMarkupFileSendsMarkupAndTheSimpleEditor(t *testing.T) {
	var captured requests.CreateCampaignRequest
	mockClient := &mocks.MockClient{}
	testutil.CaptureRequest(mockClient, "CreateCampaign", &captured, testCampaign())
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "October",
		"--markup-file", testutil.WriteTempFile(t, "october.ahaml", testMarkup))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"name": "October", "editor": "simple", "content": map[string]any{"markup": testMarkup}},
		testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestCreateWithCampaignTemplateIDSendsTemplateID(t *testing.T) {
	templateID := uuid.MustParse(testTemplateID)
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateCampaign", requests.CreateCampaignRequest{Name: "Launch", TemplateID: &templateID}).
		Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Launch", "--campaign-template-id", testTemplateID)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateValidationHappensBeforeAuthentication(t *testing.T) {
	htmlPath := testutil.WriteTempFile(t, "body.html", "<p></p>")
	mjmlPath := testutil.WriteTempFile(t, "body.mjml", "<mjml></mjml>")
	textPath := testutil.WriteTempFile(t, "body.txt", "Hi")
	markupPath := testutil.WriteTempFile(t, "body.ahaml", testMarkup)
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--subject", "Hello"}, "--name is required"},
		{[]string{"--name", "x", "--campaign-template-id", testTemplateID, "--html-file", htmlPath},
			"--campaign-template-id and --html-file cannot be used together"},
		{[]string{"--name", "x", "--campaign-template-id", testTemplateID, "--mjml-file", mjmlPath},
			"--campaign-template-id and --mjml-file cannot be used together"},
		{[]string{"--name", "x", "--campaign-template-id", testTemplateID, "--text-file", textPath},
			"--campaign-template-id and --text-file cannot be used together"},
		{[]string{"--name", "x", "--campaign-template-id", testTemplateID, "--editor", "html"},
			"--campaign-template-id and --editor cannot be used together"},
		{[]string{"--name", "x", "--campaign-template-id", testTemplateID, "--markup-file", markupPath},
			"--campaign-template-id and --markup-file cannot be used together"},
		{[]string{"--name", "x", "--markup-file", markupPath, "--mjml-file", mjmlPath}, "--markup-file and --mjml-file cannot be used together"},
		{[]string{"--name", "x", "--markup-file", markupPath, "--html-file", htmlPath}, "--markup-file and --html-file cannot be used together"},
		{[]string{"--name", "x", "--markup-file", markupPath, "--editor", "html"},
			"--editor html cannot be used with --markup-file, which holds a Simple Editor design"},
		{[]string{"--name", "x", "--markup-file", markupPath, "--editor", "advanced"},
			"--editor advanced cannot be used with --markup-file, which holds a Simple Editor design"},
		{[]string{"--name", "x", "--markup-file", testutil.WriteTempFile(t, "empty.ahaml", "")}, "--markup-file is empty"},
		{[]string{"--name", "x", "--campaign-template-id", "nope"}, `invalid campaign template ID value "nope": not a UUID`},
		{[]string{"--name", "x", "--list-id", "nope"}, `invalid list ID value "nope": not a UUID`},
		{[]string{"--name", "x", "--segment-id", "nope"}, `invalid segment ID value "nope": not a UUID`},
		{[]string{"--name", "x", "--exclude-list-id", "nope"}, `invalid list ID value "nope": not a UUID`},
		{[]string{"--name", "x", "--exclude-segment-id", ""}, "--exclude-segment-id cannot be empty"},
		{[]string{"--name", "x", "--mjml-file", mjmlPath, "--html-file", htmlPath}, "--mjml-file and --html-file cannot be used together"},
		{[]string{"--name", "x", "--from-name", "Jane"}, "--from-name needs --from"},
		{[]string{"--name", "x", "--from", ""}, "--from cannot be empty"},
		{[]string{"--name", "x", "--reply-to", ""}, "--reply-to cannot be empty"},
		{[]string{"--name", "x", "--list-id", ""}, "--list-id cannot be empty"},
		{[]string{"--name", "x", "--list-id", testListID + ","}, "--list-id cannot be empty"},
		{[]string{"--name", "x", "--text-file", testutil.WriteTempFile(t, "empty.txt", "")}, "--text-file is empty"},
		{[]string{"--name", "x", "--html-file", filepath.Join(t.TempDir(), "missing.html")}, "failed to read --html-file"},
		{[]string{"--name", "x", "--editor", ""}, "--editor cannot be empty"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewCreateCommand(), test.args...)
			require.Error(t, err)
			assert.True(t, strings.HasPrefix(err.Error(), test.message), err.Error())
			assert.False(t, *authCalled)
		})
	}
}

// update ----------------------------------------------------------------------

// expectUpdate records the update request the mock receives.
func expectUpdate(mockClient *mocks.MockClient, captured *requests.UpdateCampaignRequest) {
	testutil.CaptureRequest(mockClient, "UpdateCampaign", captured, testCampaign(), uuid.MustParse(testCampaignID))
}

func TestUpdateClearReplyToSendsNull(t *testing.T) {
	var captured requests.UpdateCampaignRequest
	mockClient := &mocks.MockClient{}
	expectUpdate(mockClient, &captured)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignID, "--clear-reply-to")
	require.NoError(t, err)
	wire := testutil.WireJSON(t, captured)
	replyTo, present := wire["reply_to"]
	assert.True(t, present)
	assert.Nil(t, replyTo)
	assert.Len(t, wire, 1)
	mockClient.AssertExpectations(t)
}

func TestUpdateClearFromSendsNullFrom(t *testing.T) {
	var captured requests.UpdateCampaignRequest
	mockClient := &mocks.MockClient{}
	expectUpdate(mockClient, &captured)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignID, "--clear-from")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"from": nil}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

// TestUpdateClearExcludeSegmentsSendsAnEmptySetAlone checks that a clear
// flag sends its set as [] and leaves the other sets out, so the API keeps
// them.
func TestUpdateClearExcludeSegmentsSendsAnEmptySetAlone(t *testing.T) {
	var captured requests.UpdateCampaignRequest
	mockClient := &mocks.MockClient{}
	expectUpdate(mockClient, &captured)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignID, "--clear-exclude-segments")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"exclude_segment_ids": []any{}}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

// TestUpdateReplacesAndEmptiesTheAudienceSets checks that each ID flag
// replaces its set and each clear flag empties its own.
func TestUpdateReplacesAndEmptiesTheAudienceSets(t *testing.T) {
	tests := map[string]struct {
		args []string
		wire map[string]any
	}{
		"replace": {
			[]string{"--list-id", testListID, "--segment-id", testSegmentID, "--exclude-list-id", testListID2,
				"--exclude-segment-id", testSegmentID2},
			map[string]any{"list_ids": []any{testListID}, "segment_ids": []any{testSegmentID},
				"exclude_list_ids": []any{testListID2}, "exclude_segment_ids": []any{testSegmentID2}},
		},
		"empty": {
			[]string{"--clear-lists", "--clear-segments", "--clear-exclude-lists", "--clear-exclude-segments"},
			map[string]any{"list_ids": []any{}, "segment_ids": []any{}, "exclude_list_ids": []any{}, "exclude_segment_ids": []any{}},
		},
		"segments for lists": {
			[]string{"--segment-id", testSegmentID, "--clear-lists"},
			map[string]any{"list_ids": []any{}, "segment_ids": []any{testSegmentID}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var captured requests.UpdateCampaignRequest
			mockClient := &mocks.MockClient{}
			expectUpdate(mockClient, &captured)
			testutil.InjectClient(t, mockClient)

			_, err := testutil.RunPlain(t, NewUpdateCommand(), append([]string{testCampaignID}, test.args...)...)
			require.NoError(t, err)
			assert.Equal(t, test.wire, testutil.WireJSON(t, captured))
			mockClient.AssertExpectations(t)
		})
	}
}

func TestUpdateRegenerateTextSendsNullText(t *testing.T) {
	var captured requests.UpdateCampaignRequest
	mockClient := &mocks.MockClient{}
	expectUpdate(mockClient, &captured)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignID, "--html-file", testutil.WriteTempFile(t, "body.html", "<p>Hi</p>"),
		"--regenerate-text")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"content": map[string]any{"html": "<p>Hi</p>", "text": nil}}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestUpdateSendsChangedFieldsAndIdempotencyKey(t *testing.T) {
	name, subject, preheader, replyTo := "November", "", "Read me", "support@example.com"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateCampaign", uuid.MustParse(testCampaignID), requests.UpdateCampaignRequest{
		Name:      &name,
		Subject:   &subject,
		Preheader: &preheader,
		ReplyTo:   &replyTo,
		ListIDs:   []uuid.UUID{uuid.MustParse(testListID2), uuid.MustParse(testListID)},
	}, testutil.IdempotencyKeyArg("update-key")).Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignID, "--name", name, "--subject", subject,
		"--preheader", preheader, "--reply-to", replyTo, "--list-id", testListID2, "--list-id", testListID,
		"--idempotency-key", "update-key")
	require.NoError(t, err)
	assert.Contains(t, output, "Campaign updated successfully")
	mockClient.AssertExpectations(t)
}

func TestUpdateWithMarkupFileAloneSendsTheMarkup(t *testing.T) {
	var captured requests.UpdateCampaignRequest
	mockClient := &mocks.MockClient{}
	expectUpdate(mockClient, &captured)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignID,
		"--markup-file", testutil.WriteTempFile(t, "october.ahaml", testMarkup))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"content": map[string]any{"markup": testMarkup}}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestUpdateSendsContentAndSender(t *testing.T) {
	mjml := "<mjml></mjml>"
	text := "Hi"
	fromName := "Acme"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateCampaign", uuid.MustParse(testCampaignID), requests.UpdateCampaignRequest{
		Content: &requests.TemplateContentInput{MJML: &mjml, Text: &text},
		From:    &common.SenderAddress{Email: "news@example.com", Name: &fromName},
	}).Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignID, "--mjml-file", testutil.WriteTempFile(t, "body.mjml", mjml),
		"--text-file", testutil.WriteTempFile(t, "body.txt", text), "--from", "news@example.com", "--from-name", fromName)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateValidationHappensBeforeAuthentication(t *testing.T) {
	textPath := testutil.WriteTempFile(t, "body.txt", "Hi")
	atLeastOne := "at least one of --name, --subject, --preheader, --markup-file, --mjml-file, --html-file, --text-file, " +
		"--regenerate-text, --from, --clear-from, --reply-to, --clear-reply-to, --list-id, --clear-lists, --segment-id, " +
		"--clear-segments, --exclude-list-id, --clear-exclude-lists, --exclude-segment-id or --clear-exclude-segments " +
		"must be provided"
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{testCampaignID, "--list-id", ""}, "--list-id cannot be empty; use --clear-lists to clear it"},
		{[]string{testCampaignID, "--clear-lists", "--list-id", testListID}, "--clear-lists and --list-id cannot be used together"},
		{[]string{testCampaignID, "--clear-lists", "--list-id", "x"}, "--clear-lists and --list-id cannot be used together"},
		{[]string{testCampaignID, "--segment-id", testSegmentID, "--clear-segments=false"},
			"--segment-id and --clear-segments cannot be used together"},
		{[]string{testCampaignID, "--exclude-list-id", testListID, "--clear-exclude-lists"},
			"--exclude-list-id and --clear-exclude-lists cannot be used together"},
		{[]string{testCampaignID, "--clear-exclude-segments", "--exclude-segment-id", testSegmentID},
			"--clear-exclude-segments and --exclude-segment-id cannot be used together"},
		{[]string{testCampaignID, "--clear-segments=false", "--clear-exclude-lists=false"}, atLeastOne},
		{[]string{testCampaignID, "--segment-id", "nope"}, `invalid segment ID value "nope": not a UUID`},
		{[]string{testCampaignID, "--list-id", testListID, "--list-id", "nope"}, `invalid list ID value "nope": not a UUID`},
		{[]string{"not-a-uuid", "--name", "x"}, `invalid campaign ID value "not-a-uuid": not a UUID`},
		{[]string{testCampaignID}, atLeastOne},
		{[]string{testCampaignID, "--clear-from=false", "--regenerate-text=false"}, atLeastOne},
		{[]string{testCampaignID, "--clear-from", "--from", "a@example.com"}, "--clear-from and --from cannot be used together"},
		{[]string{testCampaignID, "--regenerate-text", "--text-file", textPath}, "--regenerate-text and --text-file cannot be used together"},
		{[]string{testCampaignID, "--text-file", testutil.WriteTempFile(t, "empty.txt", "")}, "--text-file is empty"},
		{[]string{testCampaignID, "--mjml-file", textPath, "--html-file", textPath}, "--mjml-file and --html-file cannot be used together"},
		{[]string{testCampaignID, "--markup-file", textPath, "--html-file", textPath}, "--markup-file and --html-file cannot be used together"},
		{[]string{testCampaignID, "--markup-file", testutil.WriteTempFile(t, "empty.ahaml", "")}, "--markup-file is empty"},
		{[]string{testCampaignID, "--from-name", "Jane"}, "--from-name needs --from"},
		{[]string{testCampaignID, "--from", ""}, "--from cannot be empty; use --clear-from to clear it"},
		{[]string{testCampaignID, "--name", ""}, "--name cannot be empty"},
		{[]string{testCampaignID, "--clear-reply-to=false"}, atLeastOne},
		{[]string{testCampaignID, "--reply-to", ""}, "--reply-to cannot be empty; use --clear-reply-to to clear it"},
		{[]string{testCampaignID, "--reply-to", "a@example.com", "--clear-reply-to"},
			"--reply-to and --clear-reply-to cannot be used together"},
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

// delete ----------------------------------------------------------------------

func TestDeleteDeclinedMakesNoCall(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", testCampaignID)
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete campaign "+testCampaignID)
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedPromptsOnStderrOnly(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteCampaign", uuid.MustParse(testCampaignID)).
		Return(&common.SuccessResponse{Message: "Campaign deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "json", "yes\n", testCampaignID)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	assert.JSONEq(t, `{"message":"Campaign deleted"}`, stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteCampaign", uuid.MustParse(testCampaignID)).Return(&common.SuccessResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "", testCampaignID, "--force")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, "Campaign '"+testCampaignID+"' deleted successfully\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteRejectsBadCampaignIDBeforePrompt(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "not-a-uuid")
	require.Error(t, err)
	assert.NotContains(t, stderr, "Are you sure")
	assert.False(t, *authCalled)
}

// duplicate -------------------------------------------------------------------

func TestDuplicateSendsNameAndIdempotencyKey(t *testing.T) {
	name := "November"
	mockClient := &mocks.MockClient{}
	mockClient.On("DuplicateCampaign", uuid.MustParse(testCampaignID), requests.DuplicateCampaignRequest{Name: &name},
		testutil.IdempotencyKeyArg("dup-key")).Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewDuplicateCommand(), testCampaignID, "--name", name, "--idempotency-key", "dup-key")
	require.NoError(t, err)
	assert.Contains(t, output, "Campaign duplicated successfully")
	mockClient.AssertExpectations(t)
}

func TestDuplicateWithoutNameSendsNoName(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DuplicateCampaign", uuid.MustParse(testCampaignID), requests.DuplicateCampaignRequest{}).
		Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewDuplicateCommand(), testCampaignID)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestDuplicateValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"not-a-uuid"}, `invalid campaign ID value "not-a-uuid": not a UUID`},
		{[]string{testCampaignID, "--name", ""}, "--name cannot be empty"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewDuplicateCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

// send ------------------------------------------------------------------------

func testCampaignSend(warnings ...string) *responses.CampaignSend {
	campaign := testCampaign()
	campaign.Status = common.CampaignStatusValidating
	return &responses.CampaignSend{Campaign: *campaign, Warnings: warnings}
}

func TestSendScheduleSendsSendAtAndIdempotencyKey(t *testing.T) {
	sendAt := time.Date(2099, 11, 1, 9, 0, 0, 0, time.UTC)
	var captured requests.SendCampaignRequest
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), mock.AnythingOfType("requests.SendCampaignRequest"),
		testutil.IdempotencyKeyArg("send-key")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(requests.SendCampaignRequest)
		}).Return(testCampaignSend(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewSendCommand(), testCampaignID, "--schedule", "2099-11-01T09:00:00Z", "--force",
		"--idempotency-key", "send-key")
	require.NoError(t, err)
	require.NotNil(t, captured.SendAt)
	assert.True(t, sendAt.Equal(*captured.SendAt))
	assert.Equal(t, map[string]any{"send_at": "2099-11-01T09:00:00Z"}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestSendWithoutScheduleSendsNow(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), requests.SendCampaignRequest{}).
		Return(testCampaignSend(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewSendCommand(), testCampaignID, "--force")
	require.NoError(t, err)
	assert.Contains(t, output, "Campaign send accepted")
	assert.Contains(t, output, "Status: validating\n")
	mockClient.AssertExpectations(t)
}

func TestSendOutputPrintsWarnings(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), requests.SendCampaignRequest{}).
		Return(testCampaignSend("The account cannot send right now"), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewSendCommand(), testCampaignID, "--force")
	require.NoError(t, err)
	assert.Contains(t, output, "Warnings: The account cannot send right now\n")
	mockClient.AssertExpectations(t)
}

func TestSendInvalidScheduleRejectedBeforePromptAndAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "y\n", testCampaignID, "--schedule", "tomorrow")
	require.Error(t, err)
	assert.Equal(t, `invalid --schedule value "tomorrow": use RFC 3339 or a relative time such as 2h or 7d`, err.Error())
	assert.NotContains(t, stderr, "Are you sure")
	assert.False(t, *authCalled)
}

func TestSendScheduleAcceptsARelativeTime(t *testing.T) {
	var sent requests.SendCampaignRequest
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), mock.AnythingOfType("requests.SendCampaignRequest")).
		Run(func(arguments mock.Arguments) { sent = arguments.Get(1).(requests.SendCampaignRequest) }).
		Return(testCampaignSend(), nil)
	testutil.InjectClient(t, mockClient)

	before := time.Now()
	_, err := testutil.RunPlain(t, NewSendCommand(), testCampaignID, "--force", "--schedule", "2h")

	require.NoError(t, err)
	require.NotNil(t, sent.SendAt)
	assert.WithinDuration(t, before.Add(2*time.Hour), *sent.SendAt, time.Minute)
}

func TestSendRejectsBadCampaignIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewSendCommand(), "not-a-uuid", "--force")
	require.Error(t, err)
	assert.Equal(t, `invalid campaign ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *authCalled)
}

func TestSendDeclinedMakesNoCall(t *testing.T) {
	previous := time.Local
	time.Local = time.FixedZone("UTC+2", 2*60*60)
	t.Cleanup(func() { time.Local = previous })
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "n\n", testCampaignID,
		"--schedule", "2099-11-01T09:00:00+02:00")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "send campaign "+testCampaignID)
	assert.Contains(t, stderr, "at 2099-11-01 09:00:00 UTC+2?")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestSendPromptWithoutScheduleNamesCampaign(t *testing.T) {
	testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "\n", testCampaignID)
	require.Error(t, err)
	assert.Contains(t, stderr, "Are you sure you want to send campaign "+testCampaignID+" to its audience now? (y/N): ")
}

func TestSendAcceptedPromptsOnStderrOnly(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), requests.SendCampaignRequest{}).
		Return(testCampaignSend(), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewSendCommand(), "json", "y\n", testCampaignID)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	var decoded responses.CampaignSend
	require.NoError(t, json.Unmarshal([]byte(stdout), &decoded))
	assert.Equal(t, common.CampaignStatusValidating, decoded.Status)
	mockClient.AssertExpectations(t)
}

func TestSendForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), requests.SendCampaignRequest{}).
		Return(testCampaignSend(), nil)
	testutil.InjectClient(t, mockClient)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", testCampaignID, "--force")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	mockClient.AssertExpectations(t)
}

func TestSendNotReadyListsIssuesOutsideJSON(t *testing.T) {
	for _, format := range []string{"table", "plain", "csv"} {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), requests.SendCampaignRequest{}).
				Return(nil, notReadyError())
			testutil.InjectClient(t, mockClient)

			stdout, _, err := testutil.RunCommand(t, NewSendCommand(), format, "", testCampaignID, "--force")
			require.Error(t, err)
			assert.Equal(t, notReadySendText, err.Error())
			var cliErr *clierrors.CLIError
			require.ErrorAs(t, err, &cliErr)
			assert.Equal(t, clierrors.ErrCodeValidation, cliErr.Code)
			assert.Empty(t, stdout)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestSendOutputShowsTheAudience(t *testing.T) {
	send := &responses.CampaignSend{Campaign: *testAudienceCampaign()}
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), requests.SendCampaignRequest{}).Return(send, nil)
	testutil.InjectClient(t, mockClient)

	plain, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", testCampaignID, "--force")
	require.NoError(t, err)
	assert.Contains(t, plain, "\n"+audienceRows)
	csv, _, err := testutil.RunCommand(t, NewSendCommand(), "csv", "", testCampaignID, "--force")
	require.NoError(t, err)
	assert.Contains(t, csv, "list_ids,segment_ids,exclude_list_ids,exclude_segment_ids,editor,")
	assert.Contains(t, csv, ","+audienceCSV)
}

// TestTextUncheckedSaysToRunTheCommandAgain checks the line a refusal with
// the text_unchecked issue ends with: a send with --idempotency-key, whose
// refusal the API stores, needs a new key, and a send without one and a
// test do not.
func TestTextUncheckedSaysToRunTheCommandAgain(t *testing.T) {
	const again = "text_unchecked clears on its own: run the command again"
	tests := map[string]struct {
		command func() *cobra.Command
		method  string
		args    []string
		want    string
	}{
		"send": {NewSendCommand, "SendCampaign", []string{testCampaignID, "--force"},
			"failed to send campaign: campaign is not ready to send\n" + uncheckedIssues + again},
		"send with a key": {NewSendCommand, "SendCampaign", []string{testCampaignID, "--force", "--idempotency-key", "send-1"},
			"failed to send campaign: campaign is not ready to send\n" + uncheckedIssues + again + " with a new --idempotency-key"},
		"test": {NewTestCommand, "TestCampaign", []string{testCampaignID, "--to", "jane@example.com"},
			"failed to test campaign: campaign is not ready to send\n" + uncheckedIssues + again},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On(test.method, mock.Anything, mock.Anything).Return(nil, uncheckedError()).Maybe()
			mockClient.On(test.method, mock.Anything, mock.Anything, mock.Anything).Return(nil, uncheckedError()).Maybe()
			testutil.InjectClient(t, mockClient)

			stdout, _, err := testutil.RunCommand(t, test.command(), "table", "", test.args...)

			require.Error(t, err)
			assert.Equal(t, test.want, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.Empty(t, stdout)
		})
	}
}

func TestSendNotReadyKeepsSDKErrorInJSON(t *testing.T) {
	apiErr := notReadyError()
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), requests.SendCampaignRequest{}).Return(nil, apiErr)
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "json", "", testCampaignID, "--force")
	assert.Same(t, apiErr, err)
	mockClient.AssertExpectations(t)
}

func TestSendOtherRefusalKeepsSDKError(t *testing.T) {
	apiErr := &api.APIError{StatusCode: 400, Message: "send_at is in the past", Raw: []byte(`{"message":"send_at is in the past"}`)}
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), requests.SendCampaignRequest{}).Return(nil, apiErr)
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "table", "", testCampaignID, "--force")
	assert.Same(t, apiErr, err)
	mockClient.AssertExpectations(t)
}

// unschedule ------------------------------------------------------------------

func TestUnscheduleCallsAPIWithCampaignID(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UnscheduleCampaign", uuid.MustParse(testCampaignID), testutil.IdempotencyKeyArg("unschedule-key")).
		Return(testCampaign(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewUnscheduleCommand(), testCampaignID, "--idempotency-key", "unschedule-key")
	require.NoError(t, err)
	assert.Contains(t, output, "Campaign unscheduled successfully")
	mockClient.AssertExpectations(t)
}

func TestUnscheduleRejectsBadCampaignIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewUnscheduleCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid campaign ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *authCalled)
}

// test ------------------------------------------------------------------------

func testCampaignTestResult() *responses.CampaignTest {
	return &responses.CampaignTest{
		Object: "campaign_test",
		SentTo: []string{"jane@example.com"},
		Failed: []responses.CampaignTestFailure{{Email: "bad@example.com", Message: "mailbox unavailable"}},
	}
}

func TestTestWithoutToRejectedBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewTestCommand(), testCampaignID)
	require.Error(t, err)
	assert.Equal(t, "--to is required", err.Error())
	assert.False(t, *authCalled)
}

func TestTestRejectsBadCampaignIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewTestCommand(), "not-a-uuid", "--to", "a@example.com")
	require.Error(t, err)
	assert.Equal(t, `invalid campaign ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *authCalled)
}

func TestTestSplitsOnCommasAndKeepsQuotedDisplayName(t *testing.T) {
	// The form the API's address parser accepts
	named := `"Doe, Jane" <j@example.com>`
	parsed, err := mail.ParseAddress(named)
	require.NoError(t, err)
	assert.Equal(t, "Doe, Jane", parsed.Name)
	mockClient := &mocks.MockClient{}
	mockClient.On("TestCampaign", uuid.MustParse(testCampaignID), requests.TestCampaignRequest{
		To: []string{named, "team@example.com", "me@example.com"},
	}, testutil.IdempotencyKeyArg("test-key")).Return(&responses.CampaignTest{Object: "campaign_test"}, nil)
	testutil.InjectClient(t, mockClient)

	// As the help writes it
	_, err = testutil.RunPlain(t, NewTestCommand(), testCampaignID, "--to", `"""Doe, Jane"" <j@example.com>"`,
		"--to", "team@example.com,me@example.com", "--idempotency-key", "test-key")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestTestRejectsAnAddressTheAPICannotParseBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	// The comma in an unquoted display name makes it no address
	_, err := testutil.RunPlain(t, NewTestCommand(), testCampaignID, "--to", `"Doe, Jane <j@example.com>"`)
	require.Error(t, err)
	assert.Equal(t, `invalid --to value "Doe, Jane <j@example.com>": not an email address; `+
		`a display name with a comma must be in double quotes, such as "Doe, Jane" <jane@example.com>`, err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *authCalled)
}

func TestTestMoreThanFiveAddressesRejectedBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewTestCommand(), testCampaignID,
		"--to", "a@example.com,b@example.com,c@example.com,d@example.com,e@example.com,f@example.com")
	require.Error(t, err)
	assert.Equal(t, "--to takes at most 5 addresses, got 6", err.Error())
	assert.False(t, *authCalled)
}

func TestTestAddressesThatDifferOnlyInCaseCountOnce(t *testing.T) {
	to := []string{"a@example.com", "Ann <A@example.com>", "b@example.com", "c@example.com", "d@example.com", "e@example.com"}
	mockClient := &mocks.MockClient{}
	mockClient.On("TestCampaign", uuid.MustParse(testCampaignID), requests.TestCampaignRequest{To: to}).
		Return(&responses.CampaignTest{Object: "campaign_test"}, nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewTestCommand(), testCampaignID, "--to", strings.Join(to, ","))
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestTestEmptyToRejectedBeforeAuthentication(t *testing.T) {
	for _, value := range []string{"", "a@example.com,,b@example.com"} {
		t.Run(value, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			_, err := testutil.RunPlain(t, NewTestCommand(), testCampaignID, "--to", value)
			require.Error(t, err)
			assert.Equal(t, "--to cannot be empty", err.Error())
			assert.False(t, *authCalled)
		})
	}
}

func TestTestOutputWithFailures(t *testing.T) {
	tests := map[string][]string{
		"table": {"Sent: 1  Failed: 1", "jane@example.com", "bad@example.com", "mailbox unavailable"},
		"csv":   {"email,outcome,reason\n", "jane@example.com,sent,\n", "bad@example.com,failed,mailbox unavailable\n"},
	}
	for format, expected := range tests {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("TestCampaign", uuid.MustParse(testCampaignID), mock.Anything).Return(testCampaignTestResult(), nil)
			testutil.InjectClient(t, mockClient)

			stdout, _, err := testutil.RunCommand(t, NewTestCommand(), format, "", testCampaignID, "--to", "jane@example.com")
			var itemsFailed *clierrors.ItemsFailedError
			require.ErrorAs(t, err, &itemsFailed)
			assert.Equal(t, "1 of 2 addresses failed", err.Error())
			for _, text := range expected {
				assert.Contains(t, stdout, text)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestTestSummaryReflectsOutcome(t *testing.T) {
	tests := []struct {
		name    string
		result  *responses.CampaignTest
		summary string
		failed  bool
	}{
		{"all sent", &responses.CampaignTest{Object: "campaign_test", SentTo: []string{"jane@example.com"}},
			"Campaign test sent\n", false},
		{"some failed", testCampaignTestResult(), "Campaign test sent to 1 of 2 addresses\n", true},
		{"all failed", &responses.CampaignTest{Object: "campaign_test",
			Failed: []responses.CampaignTestFailure{{Email: "bad@example.com", Message: "mailbox unavailable"}}},
			"Campaign test failed for every address\n", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("TestCampaign", uuid.MustParse(testCampaignID), mock.Anything).Return(test.result, nil)
			testutil.InjectClient(t, mockClient)

			stdout, _, err := testutil.RunCommand(t, NewTestCommand(), "plain", "", testCampaignID, "--to", "jane@example.com")
			assert.True(t, strings.HasPrefix(stdout, test.summary), stdout)
			if test.failed {
				var itemsFailed *clierrors.ItemsFailedError
				require.ErrorAs(t, err, &itemsFailed)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTestJSONPrintsOnlyTheResponseWhenAnAddressFailed(t *testing.T) {
	raw := `{"object":"campaign_test","sent_to":["jane@example.com"],"failed":[{"email":"bad@example.com","message":"mailbox unavailable"}],"result_unknown":false}`
	var result responses.CampaignTest
	require.NoError(t, json.Unmarshal([]byte(raw), &result))
	mockClient := &mocks.MockClient{}
	mockClient.On("TestCampaign", uuid.MustParse(testCampaignID), mock.Anything).Return(&result, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewTestCommand(), "json", "", testCampaignID, "--to", "jane@example.com")
	var itemsFailed *clierrors.ItemsFailedError
	require.ErrorAs(t, err, &itemsFailed)
	assert.JSONEq(t, raw, stdout)
}

func TestTestOutputWhenResultUnknown(t *testing.T) {
	tests := map[string]string{
		"table": "Result unknown",
		"csv":   "email,outcome,reason\n,unknown,\n",
	}
	for format, expected := range tests {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("TestCampaign", uuid.MustParse(testCampaignID), mock.Anything).
				Return(&responses.CampaignTest{Object: "campaign_test", ResultUnknown: true}, nil)
			testutil.InjectClient(t, mockClient)

			stdout, _, err := testutil.RunCommand(t, NewTestCommand(), format, "", testCampaignID, "--to", "jane@example.com")
			require.NoError(t, err)
			assert.Contains(t, stdout, expected)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestTestJSONPrintsOnlyTheResponseWhenResultUnknown(t *testing.T) {
	raw := `{"object":"campaign_test","sent_to":[],"failed":[],"result_unknown":true}`
	var result responses.CampaignTest
	require.NoError(t, json.Unmarshal([]byte(raw), &result))
	mockClient := &mocks.MockClient{}
	mockClient.On("TestCampaign", uuid.MustParse(testCampaignID), mock.Anything).Return(&result, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewTestCommand(), "json", "", testCampaignID, "--to", "jane@example.com")
	require.NoError(t, err)
	assert.JSONEq(t, raw, stdout)
	assert.Empty(t, stderr)
	mockClient.AssertExpectations(t)
}

func TestTestNotReadyListsIssuesInTable(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("TestCampaign", uuid.MustParse(testCampaignID), mock.Anything).Return(nil, notReadyError())
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewTestCommand(), "table", "", testCampaignID, "--to", "jane@example.com")
	require.Error(t, err)
	assert.Equal(t, "failed to test campaign: campaign is not ready to send\n"+
		"sender_missing: Add a sender\nlists_missing: Choose at least one list", err.Error())
	mockClient.AssertExpectations(t)
}

// stats -----------------------------------------------------------------------

func TestStatsLeavesNullRatesOutOfPlain(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetCampaignStats", uuid.MustParse(testCampaignID)).Return(&responses.CampaignStats{
		Object:     "campaign_stats",
		CampaignID: uuid.MustParse(testCampaignID),
		Recipients: 10,
	}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewStatsCommand(), testCampaignID)
	require.NoError(t, err)
	assert.Contains(t, output, "Recipients: 10\n")
	assert.NotContains(t, output, "Open Rate")
	mockClient.AssertExpectations(t)
}

func TestStatsRejectsBadCampaignIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewStatsCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid campaign ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *authCalled)
}

// recipients ------------------------------------------------------------------

func testRecipientsPage() *responses.PaginatedCampaignRecipientsResponse {
	listName := "Newsletter"
	return &responses.PaginatedCampaignRecipientsResponse{
		Object: "list",
		Data: []responses.CampaignRecipient{{
			Object: "campaign_recipient", ContactID: uuid.MustParse(testListID),
			Email: "jane@example.com", ListName: &listName, Status: "Delivered", Attempts: 1,
		}},
	}
}

func TestRecipientsSendsStatusAndPagination(t *testing.T) {
	status, before := "Delivered", "previous"
	limit := int32(10)
	mockClient := &mocks.MockClient{}
	mockClient.On("ListCampaignRecipients", uuid.MustParse(testCampaignID), requests.ListCampaignRecipientsParams{
		Status: &status, Limit: &limit, Before: &before,
	}).Return(testRecipientsPage(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewRecipientsCommand(), testCampaignID, "--status", status, "--limit", "10", "--before", before)
	require.NoError(t, err)
	assert.Contains(t, output, "Email: jane@example.com\n")
	mockClient.AssertExpectations(t)
}

func TestRecipientsSendsBounceClassificationAndAfter(t *testing.T) {
	classification, after := "invalid_recipient", "next"
	mockClient := &mocks.MockClient{}
	mockClient.On("ListCampaignRecipients", uuid.MustParse(testCampaignID), requests.ListCampaignRecipientsParams{
		BounceClassification: &classification,
		After:                &after,
	}).Return(testRecipientsPage(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewRecipientsCommand(), testCampaignID, "--bounce-classification", classification, "--after", after)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestRecipientsValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{testCampaignID, "--status", "Delivered", "--bounce-classification", "x"},
			"--status and --bounce-classification cannot be used together"},
		{[]string{testCampaignID}, "one of --status or --bounce-classification must be provided"},
		{[]string{testCampaignID, "--status", "Delivered", "--after", "a", "--before", "b"},
			"--after and --before cannot be used together"},
		{[]string{testCampaignID, "--status", "Delivered", "--limit", "101"},
			"--limit must be between 0 and 100"},
		{[]string{"not-a-uuid", "--status", "Delivered"}, `invalid campaign ID value "not-a-uuid": not a UUID`},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewRecipientsCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

func TestNotReadyIssuesAreEscaped(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("SendCampaign", uuid.MustParse(testCampaignID), requests.SendCampaignRequest{}).
		Return(nil, &api.APIError{
			StatusCode: 400,
			Message:    "campaign is not ready to send",
			Raw: []byte(`{"message":"not \u001b[2J ready","issues":[` +
				`{"code":"sender\u0007","message":"Add a sender\nlists_missing: fake"}]}`),
		})
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", testCampaignID, "--force")
	require.Error(t, err)
	assert.Equal(t, "failed to send campaign: not \\u001b[2J ready\n"+
		"sender\\u0007: Add a sender\\u000alists_missing: fake", err.Error())
}
