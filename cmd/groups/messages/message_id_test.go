package messages

import (
	"fmt"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// sentMessageID is a Message-ID as messages send returns it.
const sentMessageID = "<8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f@example.com>"

func TestGetAndCancelAcceptTheMessageIDThatSendReturns(t *testing.T) {
	message := newTestMessage("8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f", nil)
	mockClient := &mocks.MockClient{}
	mockClient.On("GetMessage", sentMessageID).Return(&message, nil)
	mockClient.On("CancelMessage", sentMessageID).Return(&common.SuccessResponse{Message: "Canceled"}, nil)
	mockClient.On("GetAccountID").Return(testAccountID).Maybe()

	_, err := testutil.RunWithClient(t, mockClient, NewGetCommand(), "plain", sentMessageID)
	require.NoError(t, err)
	_, err = testutil.RunWithClient(t, mockClient, NewCancelCommand(), "plain", sentMessageID, "--force")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestGetAndCancelRejectAMessageIDOfAnotherShapeBeforeAuthenticating(t *testing.T) {
	for _, id := range []string{
		"not-a-uuid",
		"8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f@example.com",
		"<8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f@example.com",
		"<8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f>",
		"<not-a-uuid@example.com>",
		"<8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f@not a domain>",
	} {
		for name, newCommand := range map[string]func() *cobra.Command{"get": NewGetCommand, "cancel": NewCancelCommand} {
			t.Run(name+" "+id, func(t *testing.T) {
				called := testutil.RefuseAuth(t)
				args := []string{id}
				if name == "cancel" {
					args = append(args, "--force")
				}

				_, _, err := testutil.RunCommand(t, newCommand(), "plain", "", args...)

				require.Error(t, err)
				assert.Equal(t, fmt.Sprintf("invalid message ID value %q: not a UUID or <uuid@domain>", id), err.Error())
				assert.Equal(t, 4, clierrors.GetExitCode(err))
				assert.False(t, *called)
			})
		}
	}
}
