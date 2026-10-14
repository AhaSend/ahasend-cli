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
	"github.com/stretchr/testify/require"
)

const testTemplateID = "33333333-3333-4333-8333-333333333333"

func testTemplate() *responses.Template {
	return &responses.Template{
		Object:    "template",
		ID:        uuid.MustParse(testTemplateID),
		Name:      "Welcome",
		Subject:   "Hello",
		Editor:    "advanced",
		Variables: []responses.TemplateVariable{{Name: "first_name", Required: true}},
	}
}

// list ------------------------------------------------------------------------

func TestListForwardsPagination(t *testing.T) {
	limit := int32(5)
	before := "previous"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetTemplates", requests.GetTemplatesParams{Limit: &limit, Before: &before}).
		Return(&responses.PaginatedTemplatesResponse{Object: "list", Data: []responses.Template{*testTemplate()}}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewListCommand(), "--limit", "5", "--before", before)
	require.NoError(t, err)
	assert.Contains(t, output, "Name: Welcome")
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

func TestGetSendsTemplateID(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetTemplate", uuid.MustParse(testTemplateID)).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewGetCommand(), testTemplateID)
	require.NoError(t, err)
	assert.Contains(t, output, "Variables: first_name (required)\n")
	mockClient.AssertExpectations(t)
}

func TestGetRejectsBadTemplateIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewGetCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid template ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *authCalled)
}

// create ----------------------------------------------------------------------

func TestCreateWithMJMLFileSendsMJMLAndNoEditor(t *testing.T) {
	mjmlPath := testutil.WriteTempFile(t, "welcome.mjml", "<mjml></mjml>")
	mjml := "<mjml></mjml>"
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateTemplate", requests.CreateTemplateRequest{
		Name:    "Welcome",
		Content: &requests.TemplateContentInput{MJML: &mjml},
	}).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Welcome", "--mjml-file", mjmlPath)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateSendsEveryFieldAndIdempotencyKey(t *testing.T) {
	textPath := testutil.WriteTempFile(t, "welcome.txt", "Hello {{ first_name }}")
	editor, subject, preheader, text := "simple", "Hello", "Read me", "Hello {{ first_name }}"
	fromName := "Doe, Jane"
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateTemplate", requests.CreateTemplateRequest{
		Name:      "Welcome",
		Editor:    &editor,
		Subject:   &subject,
		Preheader: &preheader,
		From:      &common.SenderAddress{Email: "hello@example.com", Name: &fromName},
		ReplyTo:   &common.SenderAddress{Email: "support@example.com"},
		Content:   &requests.TemplateContentInput{Text: &text},
		Publish:   true,
	}, testutil.IdempotencyKeyArg("template-key")).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Welcome", "--editor", editor,
		"--subject", subject, "--preheader", preheader, "--text-file", textPath,
		"--from", "hello@example.com", "--from-name", fromName, "--reply-to", "support@example.com",
		"--publish", "--idempotency-key", "template-key")
	require.NoError(t, err)
	assert.Contains(t, output, "Template created successfully")
	mockClient.AssertExpectations(t)
}

func TestCreateValidationHappensBeforeAuthentication(t *testing.T) {
	mjmlPath := testutil.WriteTempFile(t, "welcome.mjml", "<mjml></mjml>")
	htmlPath := testutil.WriteTempFile(t, "welcome.html", "<p></p>")
	latin1Path := testutil.WriteTempFile(t, "latin1.html", "<p>caf\xe9</p>")
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--subject", "Hello"}, "--name is required"},
		{[]string{"--name", "Welcome", "--mjml-file", mjmlPath, "--html-file", htmlPath}, "--mjml-file and --html-file cannot be used together"},
		{[]string{"--name", "Welcome", "--editor", "simple", "--from-name", "Jane"}, "--from-name needs --from"},
		{[]string{"--name", "Welcome", "--editor", "simple", "--from", ""}, "--from cannot be empty"},
		{[]string{"--name", "Welcome", "--editor", "simple", "--reply-to", ""}, "--reply-to cannot be empty"},
		{[]string{"--name", "Welcome", "--editor", "simple", "--reply-to", "a@example.com"}, "--reply-to needs --from"},
		{[]string{"--name", "Welcome", "--text-file", testutil.WriteTempFile(t, "empty.txt", "")}, "--text-file is empty"},
		{[]string{"--name", "Welcome", "--mjml-file", filepath.Join(t.TempDir(), "missing.mjml")}, "failed to read --mjml-file"},
		{[]string{"--name", "Welcome"}, "--editor is needed when there is neither --mjml-file nor --html-file"},
		{[]string{"--name", "Welcome", "--text-file", testutil.WriteTempFile(t, "notice.txt", "Hi")}, "--editor is needed when there is neither --mjml-file nor --html-file"},
		{[]string{"--name", "Welcome", "--editor", ""}, "--editor cannot be empty"},
		{[]string{"--name", "Welcome", "--html-file", latin1Path}, latin1Path + " is not UTF-8 text; save it as UTF-8"},
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
func expectUpdate(mockClient *mocks.MockClient, captured *requests.UpdateTemplateRequest) {
	testutil.CaptureRequest(mockClient, "UpdateTemplate", captured, testTemplate(), uuid.MustParse(testTemplateID))
}

func TestUpdateRegenerateTextSendsNullText(t *testing.T) {
	var captured requests.UpdateTemplateRequest
	mockClient := &mocks.MockClient{}
	expectUpdate(mockClient, &captured)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testTemplateID, "--regenerate-text")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"content": map[string]any{"text": nil}}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestUpdateClearFromSendsNullFrom(t *testing.T) {
	var captured requests.UpdateTemplateRequest
	mockClient := &mocks.MockClient{}
	expectUpdate(mockClient, &captured)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testTemplateID, "--clear-from")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"from": nil}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestUpdateClearReplyToSendsNullReplyTo(t *testing.T) {
	var captured requests.UpdateTemplateRequest
	mockClient := &mocks.MockClient{}
	expectUpdate(mockClient, &captured)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testTemplateID, "--clear-reply-to")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"reply_to": nil}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestUpdateEmptySubjectSendsEmptyString(t *testing.T) {
	var captured requests.UpdateTemplateRequest
	mockClient := &mocks.MockClient{}
	expectUpdate(mockClient, &captured)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testTemplateID, "--subject", "")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"subject": ""}, testutil.WireJSON(t, captured))
	mockClient.AssertExpectations(t)
}

func TestUpdateSendsChangedFieldsAndIdempotencyKey(t *testing.T) {
	name := "Welcome v2"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateTemplate", uuid.MustParse(testTemplateID), requests.UpdateTemplateRequest{
		Name:    &name,
		ReplyTo: &common.SenderAddress{Email: "support@example.com"},
		Publish: true,
	}, testutil.IdempotencyKeyArg("update-key")).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testTemplateID, "--name", name,
		"--reply-to", "support@example.com", "--publish", "--idempotency-key", "update-key")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateSendsContentSenderAndPreheader(t *testing.T) {
	preheader := "Hello"
	html := "<p>Hi</p>"
	text := "Hi"
	fromName := "Acme"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateTemplate", uuid.MustParse(testTemplateID), requests.UpdateTemplateRequest{
		Preheader: &preheader,
		Content:   &requests.TemplateContentInput{HTML: &html, Text: &text},
		From:      &common.SenderAddress{Email: "hello@example.com", Name: &fromName},
	}).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testTemplateID, "--preheader", preheader,
		"--html-file", testutil.WriteTempFile(t, "body.html", html), "--text-file", testutil.WriteTempFile(t, "body.txt", text),
		"--from", "hello@example.com", "--from-name", fromName)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdatePublishAloneOnlyPublishes(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateTemplate", uuid.MustParse(testTemplateID), requests.UpdateTemplateRequest{Publish: true}).
		Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testTemplateID, "--publish")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateValidationHappensBeforeAuthentication(t *testing.T) {
	emptyPath := testutil.WriteTempFile(t, "empty.txt", "")
	textPath := testutil.WriteTempFile(t, "text.txt", "Hello")
	atLeastOne := "at least one of --name, --subject, --preheader, --mjml-file, --html-file, --text-file, " +
		"--regenerate-text, --from, --clear-from, --reply-to, --clear-reply-to or --publish must be provided"
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{testTemplateID, "--text-file", emptyPath}, "--text-file is empty"},
		{[]string{testTemplateID, "--regenerate-text", "--text-file", textPath}, "--regenerate-text and --text-file cannot be used together"},
		{[]string{testTemplateID, "--clear-from", "--from", "a@example.com"}, "--clear-from and --from cannot be used together"},
		{[]string{testTemplateID, "--clear-reply-to", "--reply-to", "a@example.com"}, "--clear-reply-to and --reply-to cannot be used together"},
		{[]string{testTemplateID}, atLeastOne},
		{[]string{testTemplateID, "--publish=false", "--clear-from=false", "--regenerate-text=false"}, atLeastOne},
		{[]string{"not-a-uuid", "--name", "x"}, `invalid template ID value "not-a-uuid": not a UUID`},
		{[]string{testTemplateID, "--mjml-file", textPath, "--html-file", textPath}, "--mjml-file and --html-file cannot be used together"},
		{[]string{testTemplateID, "--from-name", "Jane"}, "--from-name needs --from"},
		{[]string{testTemplateID, "--from", ""}, "--from cannot be empty; use --clear-from to clear it"},
		{[]string{testTemplateID, "--reply-to", ""}, "--reply-to cannot be empty; use --clear-reply-to to clear it"},
		{[]string{testTemplateID, "--name", ""}, "--name cannot be empty"},
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

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", testTemplateID)
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete template "+testTemplateID)
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedPromptsOnStderrOnly(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteTemplate", uuid.MustParse(testTemplateID)).
		Return(&common.SuccessResponse{Message: "Template deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "json", " Yes \n", testTemplateID)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	assert.JSONEq(t, `{"message":"Template deleted"}`, stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteTemplate", uuid.MustParse(testTemplateID)).Return(&common.SuccessResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "", testTemplateID, "--force")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, "Template '"+testTemplateID+"' deleted successfully\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteRejectsBadTemplateIDBeforePrompt(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "not-a-uuid")
	require.Error(t, err)
	assert.NotContains(t, stderr, "Are you sure")
	assert.False(t, *authCalled)
}

// publish ---------------------------------------------------------------------

func TestPublishSendsTemplateIDAndIdempotencyKey(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("PublishTemplate", uuid.MustParse(testTemplateID), testutil.IdempotencyKeyArg("publish-key")).
		Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewPublishCommand(), testTemplateID, "--idempotency-key", "publish-key")
	require.NoError(t, err)
	assert.Contains(t, output, "Template published successfully")
	mockClient.AssertExpectations(t)
}

func TestPublishRejectsBadIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewPublishCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid template ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *authCalled)
}

func TestPublishWithoutKeyPassesNoOption(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("PublishTemplate", uuid.MustParse(testTemplateID)).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewPublishCommand(), testTemplateID)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}
