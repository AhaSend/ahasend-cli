package draft

import (
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTemplateID = "33333333-3333-4333-8333-333333333333"

func testTemplate() *responses.Template {
	return &responses.Template{
		Object: "template",
		ID:     uuid.MustParse(testTemplateID),
		Name:   "Welcome",
		Editor: "html",
	}
}

// discard ---------------------------------------------------------------------

func TestDiscardPrintsReturnedTemplate(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DiscardTemplateDraft", uuid.MustParse(testTemplateID)).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewDiscardCommand(), "plain", "", testTemplateID, "--force")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Template draft discarded successfully\n")
	assert.Contains(t, stdout, "Name: Welcome\n")
	assert.Contains(t, stdout, "ID: "+testTemplateID+"\n")
	mockClient.AssertExpectations(t)
}

func TestDiscardDeclinedMakesNoCall(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDiscardCommand(), "plain", "no\n", testTemplateID)
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to discard the draft of template "+testTemplateID)
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDiscardAcceptedPromptsOnStderrOnly(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DiscardTemplateDraft", uuid.MustParse(testTemplateID)).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDiscardCommand(), "json", "y\n", testTemplateID)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	assert.NotContains(t, stdout, "Are you sure")
	assert.Contains(t, stdout, `"name": "Welcome"`)
	mockClient.AssertExpectations(t)
}

func TestDiscardForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DiscardTemplateDraft", uuid.MustParse(testTemplateID)).Return(testTemplate(), nil)
	testutil.InjectClient(t, mockClient)

	_, stderr, err := testutil.RunCommand(t, NewDiscardCommand(), "plain", "", testTemplateID, "--force")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	mockClient.AssertExpectations(t)
}

func TestDiscardRejectsBadTemplateIDBeforePrompt(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewDiscardCommand(), "plain", "y\n", "not-a-uuid")
	require.Error(t, err)
	assert.NotContains(t, stderr, "Are you sure")
	assert.False(t, *authCalled)
}
