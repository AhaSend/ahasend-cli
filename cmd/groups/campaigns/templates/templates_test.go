package templates

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testCampaignTemplateID = "77777777-7777-4777-8777-777777777777"

func testCampaignTemplate() *responses.CampaignTemplate {
	return &responses.CampaignTemplate{
		Object:  "campaign_template",
		ID:      uuid.MustParse(testCampaignTemplateID),
		Name:    "Newsletter",
		Editor:  "html",
		Content: &responses.TemplateContent{HTML: "<p>News</p>"},
	}
}

// list ------------------------------------------------------------------------

func TestListForwardsPagination(t *testing.T) {
	limit := int32(5)
	after := "next"
	template := testCampaignTemplate()
	template.Content = nil
	mockClient := &mocks.MockClient{}
	mockClient.On("ListCampaignTemplates", requests.ListCampaignTemplatesParams{Limit: &limit, After: &after}).
		Return(&responses.PaginatedCampaignTemplatesResponse{Object: "list", Data: []responses.CampaignTemplate{*template}}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewListCommand(), "--limit", "5", "--after", after)
	require.NoError(t, err)
	assert.Contains(t, output, "Name: Newsletter")
	mockClient.AssertExpectations(t)
}

func TestListSendsBeforeCursor(t *testing.T) {
	before := "previous"
	mockClient := &mocks.MockClient{}
	mockClient.On("ListCampaignTemplates", requests.ListCampaignTemplatesParams{Before: &before}).
		Return(&responses.PaginatedCampaignTemplatesResponse{Object: "list"}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewListCommand(), "--before", before)
	require.NoError(t, err)
	assert.Equal(t, "No campaign templates found\n", output)
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

func TestGetShowsContent(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetCampaignTemplate", uuid.MustParse(testCampaignTemplateID)).Return(testCampaignTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewGetCommand(), testCampaignTemplateID)
	require.NoError(t, err)
	assert.Contains(t, output, "HTML:\n  <p>News</p>\n")
	mockClient.AssertExpectations(t)
}

func TestGetRejectsBadIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewGetCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid campaign template ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *authCalled)
}

// create ----------------------------------------------------------------------

func TestCreateSendsNameContentAndIdempotencyKey(t *testing.T) {
	html, text := "<p>News</p>", "News"
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateCampaignTemplate", requests.CreateCampaignTemplateRequest{
		Name:    "Newsletter",
		Content: &requests.TemplateContentInput{HTML: &html, Text: &text},
	}, testutil.IdempotencyKeyArg("create-key")).Return(testCampaignTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Newsletter",
		"--html-file", testutil.WriteTempFile(t, "news.html", html), "--text-file", testutil.WriteTempFile(t, "news.txt", text),
		"--idempotency-key", "create-key")
	require.NoError(t, err)
	assert.Contains(t, output, "Campaign template created successfully")
	mockClient.AssertExpectations(t)
}

func TestCreateWithMJMLFileSendsMJML(t *testing.T) {
	mjml := "<mjml></mjml>"
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateCampaignTemplate", requests.CreateCampaignTemplateRequest{
		Name:    "Newsletter",
		Content: &requests.TemplateContentInput{MJML: &mjml},
	}).Return(testCampaignTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Newsletter", "--mjml-file", testutil.WriteTempFile(t, "news.mjml", mjml))
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateWithEditorAloneSendsEditor(t *testing.T) {
	var captured requests.CreateCampaignTemplateRequest
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateCampaignTemplate", mock.AnythingOfType("requests.CreateCampaignTemplateRequest")).
		Run(func(args mock.Arguments) {
			captured = args.Get(0).(requests.CreateCampaignTemplateRequest)
		}).Return(testCampaignTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Blank", "--editor", "simple")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"name": "Blank", "editor": "simple"}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestCreateValidationHappensBeforeAuthentication(t *testing.T) {
	htmlPath := testutil.WriteTempFile(t, "news.html", "<p></p>")
	mjmlPath := testutil.WriteTempFile(t, "news.mjml", "<mjml></mjml>")
	textPath := testutil.WriteTempFile(t, "news.txt", "News")
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--editor", "html"}, "--name is required"},
		{[]string{"--name", "x", "--mjml-file", mjmlPath, "--html-file", htmlPath}, "--mjml-file and --html-file cannot be used together"},
		{[]string{"--name", "x", "--text-file", testutil.WriteTempFile(t, "empty.txt", "")}, "--text-file is empty"},
		{[]string{"--name", "x", "--html-file", filepath.Join(t.TempDir(), "missing.html")}, "failed to read --html-file"},
		{[]string{"--name", "x"}, "--editor is needed when there is neither --mjml-file nor --html-file"},
		{[]string{"--name", "x", "--text-file", textPath}, "--editor is needed when there is neither --mjml-file nor --html-file"},
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

func TestUpdateRegenerateTextSendsNullText(t *testing.T) {
	var captured requests.UpdateCampaignTemplateRequest
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateCampaignTemplate", uuid.MustParse(testCampaignTemplateID),
		mock.AnythingOfType("requests.UpdateCampaignTemplateRequest")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(requests.UpdateCampaignTemplateRequest)
		}).Return(testCampaignTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignTemplateID, "--regenerate-text")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"content": map[string]any{"text": nil}}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestUpdateSendsNameContentAndIdempotencyKey(t *testing.T) {
	name, mjml := "Newsletter v2", "<mjml></mjml>"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateCampaignTemplate", uuid.MustParse(testCampaignTemplateID), requests.UpdateCampaignTemplateRequest{
		Name:    &name,
		Content: &requests.TemplateContentInput{MJML: &mjml},
	}, testutil.IdempotencyKeyArg("update-key")).Return(testCampaignTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignTemplateID, "--name", name,
		"--mjml-file", testutil.WriteTempFile(t, "news.mjml", mjml), "--idempotency-key", "update-key")
	require.NoError(t, err)
	assert.Contains(t, output, "Campaign template updated successfully")
	mockClient.AssertExpectations(t)
}

func TestUpdateSendsHTMLAndText(t *testing.T) {
	html, text := "<p>News</p>", "News"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateCampaignTemplate", uuid.MustParse(testCampaignTemplateID), requests.UpdateCampaignTemplateRequest{
		Content: &requests.TemplateContentInput{HTML: &html, Text: &text},
	}).Return(testCampaignTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testCampaignTemplateID,
		"--html-file", testutil.WriteTempFile(t, "news.html", html), "--text-file", testutil.WriteTempFile(t, "news.txt", text))
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateValidationHappensBeforeAuthentication(t *testing.T) {
	textPath := testutil.WriteTempFile(t, "news.txt", "News")
	atLeastOne := "at least one of --name, --mjml-file, --html-file, --text-file or --regenerate-text must be provided"
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{testCampaignTemplateID}, atLeastOne},
		{[]string{testCampaignTemplateID, "--regenerate-text=false"}, atLeastOne},
		{[]string{testCampaignTemplateID, "--regenerate-text", "--text-file", textPath}, "--regenerate-text and --text-file cannot be used together"},
		{[]string{testCampaignTemplateID, "--text-file", testutil.WriteTempFile(t, "empty.txt", "")}, "--text-file is empty"},
		{[]string{testCampaignTemplateID, "--mjml-file", textPath, "--html-file", textPath}, "--mjml-file and --html-file cannot be used together"},
		{[]string{"not-a-uuid", "--name", "x"}, `invalid campaign template ID value "not-a-uuid": not a UUID`},
		{[]string{testCampaignTemplateID, "--name", ""}, "--name cannot be empty"},
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

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", testCampaignTemplateID)
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete campaign template "+testCampaignTemplateID)
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedPromptsOnStderrOnly(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteCampaignTemplate", uuid.MustParse(testCampaignTemplateID)).
		Return(&common.SuccessResponse{Message: "Campaign template deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "json", "Y\n", testCampaignTemplateID)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	assert.JSONEq(t, `{"message":"Campaign template deleted"}`, stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteCampaignTemplate", uuid.MustParse(testCampaignTemplateID)).Return(&common.SuccessResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "", testCampaignTemplateID, "--force")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, "Campaign template '"+testCampaignTemplateID+"' deleted successfully\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteRejectsBadIDBeforePrompt(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "not-a-uuid")
	require.Error(t, err)
	assert.NotContains(t, stderr, "Are you sure")
	assert.False(t, *authCalled)
}
