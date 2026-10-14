package versions

import (
	"encoding/json"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testTemplateID = "33333333-3333-4333-8333-333333333333"
	testVersionID  = "44444444-4444-4444-8444-444444444444"
)

func testTemplate() *responses.Template {
	return &responses.Template{Object: "template", ID: uuid.MustParse(testTemplateID), Name: "Welcome"}
}

func TestListSendsTemplateID(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetTemplateVersions", uuid.MustParse(testTemplateID)).Return(&responses.TemplateVersionsResponse{
		Object: "list",
		Data:   []responses.TemplateVersion{{Object: "template_version", ID: uuid.MustParse(testVersionID), Version: 3}},
	}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewListCommand(), testTemplateID)
	require.NoError(t, err)
	assert.Contains(t, output, "Version: 3\n")
	assert.NotContains(t, output, "Published By")
	mockClient.AssertExpectations(t)
}

func TestGetSendsTemplateAndVersionIDs(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetTemplateVersion", uuid.MustParse(testTemplateID), uuid.MustParse(testVersionID)).
		Return(&responses.TemplateVersionDetail{Object: "template_version", ID: uuid.MustParse(testVersionID), Version: 2}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewGetCommand(), testTemplateID, testVersionID)
	require.NoError(t, err)
	assert.Contains(t, output, "Version: 2\n")
	mockClient.AssertExpectations(t)
}

func TestRestorePublishSendsPublishTrue(t *testing.T) {
	var captured requests.RestoreTemplateVersionRequest
	mockClient := &mocks.MockClient{}
	mockClient.On("RestoreTemplateVersion", uuid.MustParse(testTemplateID), uuid.MustParse(testVersionID),
		mock.AnythingOfType("requests.RestoreTemplateVersionRequest"), testutil.IdempotencyKeyArg("restore-key")).
		Run(func(args mock.Arguments) {
			captured = args.Get(2).(requests.RestoreTemplateVersionRequest)
		}).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewRestoreCommand(), testTemplateID, testVersionID, "--publish", "--idempotency-key", "restore-key", "--force")
	require.NoError(t, err)
	assert.Contains(t, output, "Template version restored successfully")
	wire, err := json.Marshal(captured)
	require.NoError(t, err)
	assert.JSONEq(t, `{"publish":true}`, string(wire))
	mockClient.AssertExpectations(t)
}

func TestRestoreWithoutPublishSendsEmptyBody(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("RestoreTemplateVersion", uuid.MustParse(testTemplateID), uuid.MustParse(testVersionID),
		requests.RestoreTemplateVersionRequest{}).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewRestoreCommand(), testTemplateID, testVersionID, "--force")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestRestoreDeclinedMakesNoCall(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewRestoreCommand(), "plain", "n\n", testTemplateID, testVersionID)
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to restore version "+testVersionID+
		" into the draft of template "+testTemplateID+"? (y/N): ")
	assert.NotContains(t, stdout, "restored")
	assert.False(t, *authCalled)
}

func TestRestoreAcceptedPromptMentionsPublish(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("RestoreTemplateVersion", uuid.MustParse(testTemplateID), uuid.MustParse(testVersionID),
		requests.RestoreTemplateVersionRequest{Publish: true}).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, stderr, err := testutil.RunCommand(t, NewRestoreCommand(), "plain", "y\n", testTemplateID, testVersionID, "--publish")
	require.NoError(t, err)
	assert.Contains(t, stderr, "into the draft of template "+testTemplateID+" and publish it?")
	mockClient.AssertExpectations(t)
}

func TestRestoreForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("RestoreTemplateVersion", uuid.MustParse(testTemplateID), uuid.MustParse(testVersionID),
		requests.RestoreTemplateVersionRequest{}).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewRestoreCommand(), "plain", "", testTemplateID, testVersionID, "--force")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "Template version restored successfully")
	mockClient.AssertExpectations(t)
}

func TestIDsRejectedBeforeAuthentication(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *cobra.Command
		args    []string
		message string
	}{
		{"list template", NewListCommand(), []string{"bad"}, `invalid template ID value "bad": not a UUID`},
		{"get template", NewGetCommand(), []string{"bad", testVersionID}, `invalid template ID value "bad": not a UUID`},
		{"get version", NewGetCommand(), []string{testTemplateID, "bad"}, `invalid version ID value "bad": not a UUID`},
		{"restore template", NewRestoreCommand(), []string{"bad", testVersionID}, `invalid template ID value "bad": not a UUID`},
		{"restore version", NewRestoreCommand(), []string{testTemplateID, ""}, "version ID cannot be empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, test.cmd, test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}
