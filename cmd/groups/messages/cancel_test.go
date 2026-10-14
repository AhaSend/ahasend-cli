package messages

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCancelCommand(t *testing.T) {
	// Test command structure
	cancelCmd := NewCancelCommand()

	assert.Equal(t, "cancel", cancelCmd.Name())
	assert.Equal(t, "Cancel a scheduled message", cancelCmd.Short)
	assert.NotEmpty(t, cancelCmd.Long)
	assert.NotEmpty(t, cancelCmd.Example)

	// Test that it requires at least one argument
	err := cancelCmd.Args(cancelCmd, []string{})
	assert.Error(t, err, "should require at least one argument")
	assert.Equal(t, "the message-id argument is required", err.Error())

	// Test flags
	forceFlag := cancelCmd.Flag("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "f", forceFlag.Shorthand)
	assert.Equal(t, "Skip confirmation prompt", forceFlag.Usage)
}

func TestCancelCommandValidation(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "missing message ID",
			args:          []string{"cancel"},
			expectedError: "the message-id argument is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute command
			output, err := testutil.RunCommandCombined(t, NewCommand(), "table", tt.args...)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			_ = output // Output might be empty for validation errors
		})
	}
}

// TestCancelCommandIntegration would test with real client
// but we skip it for unit tests as it requires auth setup
func TestCancelCommandHelp(t *testing.T) {
	cancelCmd := NewCancelCommand()

	// Check that help can be generated without errors
	helpFunc := cancelCmd.HelpFunc()
	assert.NotNil(t, helpFunc)

	// Check examples are present
	assert.Contains(t, cancelCmd.Example, "ahasend messages cancel")
	assert.Contains(t, cancelCmd.Example, "550e8400-e29b-41d4-a716-446655440000")
}

func TestCancelDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewCancelCommand(), "plain", "n\n", "11111111-1111-1111-1111-111111111111")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to cancel message 11111111-1111-1111-1111-111111111111? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestCancelSeveralDeclinedNamesTheCount(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewCancelCommand(), "plain", "n\n", "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to cancel 2 messages? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestCancelOneMessagePrintsTheCancelledLine(t *testing.T) {
	const messageID = "11111111-1111-1111-1111-111111111111"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetAccountID").Return(testAccountID)
	mockClient.On("CancelMessage", messageID).Return(&common.SuccessResponse{}, nil)

	out, err := testutil.RunWithClient(t, mockClient, NewCancelCommand(), "plain", messageID, "--force")
	require.NoError(t, err)
	assert.Equal(t, "Message '"+messageID+"' cancelled successfully\n", out)
}

func TestCancelSeveralMessagesListsEveryOutcome(t *testing.T) {
	const cancelled, missing = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetAccountID").Return(testAccountID)
	mockClient.On("CancelMessage", cancelled).Return(&common.SuccessResponse{}, nil)
	mockClient.On("CancelMessage", missing).Return(nil, &api.APIError{StatusCode: 404, Message: "not found"})
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewCancelCommand(), "table", "", cancelled, missing, "--force")
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(stdout, "1 of 2 messages cancelled\nSucceeded: 1  Failed: 1\n"), stdout)
	assert.Regexp(t, `│ `+cancelled+` │ success │ - +│`, stdout)
	assert.Regexp(t, `│ `+missing+` │ failed  │ .*not found`, stdout)
}

// TestCancelSeveralMessagesSaysHowManyWereCancelled checks the heading of the
// results when every cancel succeeded and when none did.
func TestCancelSeveralMessagesSaysHowManyWereCancelled(t *testing.T) {
	const first, second = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	tests := map[string]struct {
		err     error
		heading string
	}{
		"all cancelled":  {nil, "Messages cancelled successfully\n"},
		"none cancelled": {&api.APIError{StatusCode: 404, Message: "not found"}, "No message cancelled\n"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetAccountID").Return(testAccountID)
			var response *common.SuccessResponse
			if test.err == nil {
				response = &common.SuccessResponse{}
			}
			mockClient.On("CancelMessage", mock.Anything).Return(response, test.err)
			testutil.InjectClient(t, mockClient)

			stdout, _, _ := testutil.RunCommand(t, NewCancelCommand(), "plain", "", first, second, "--force")
			assert.True(t, strings.HasPrefix(stdout, test.heading), stdout)
		})
	}
}
