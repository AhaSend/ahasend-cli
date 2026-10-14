package domains

import (
	"testing"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDeleteCommand_Args(t *testing.T) {
	// Test that delete command requires exactly 1 argument
	deleteCmd := NewDeleteCommand()
	assert.NotNil(t, deleteCmd.Args)
}

func TestDeleteDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", "example.com")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.Contains(t, stderr, "Are you sure you want to delete domain example.com? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedDeletesWithoutFetchingTheDomain(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteDomain", "example.com").Return(&common.SuccessResponse{Message: "domain deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "example.com")
	require.NoError(t, err)
	assert.Equal(t, "domain deleted\n", stdout)
	mockClient.AssertExpectations(t)
	mockClient.AssertNotCalled(t, "GetDomain", mock.Anything)
}
