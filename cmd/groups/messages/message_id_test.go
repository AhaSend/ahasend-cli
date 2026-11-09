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

// sentMessageID is a message's id as messages send prints it.
const sentMessageID = "8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f"

// messageIDHeader is the Message-ID header of the message sentMessageID.
const messageIDHeader = "<8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f@example.com>"

func TestGetAndCancelAcceptTheMessageIDThatSendPrints(t *testing.T) {
	assertGetAndCancelSend(t, sentMessageID)
}

func TestGetAndCancelAcceptTheMessageIDHeader(t *testing.T) {
	assertGetAndCancelSend(t, messageIDHeader)
}

// assertGetAndCancelSend checks that messages get and messages cancel send id
// to the API as it is.
func assertGetAndCancelSend(t *testing.T, id string) {
	t.Helper()
	message := newTestMessage(sentMessageID, nil)
	mockClient := &mocks.MockClient{}
	mockClient.On("GetMessage", id).Return(&message, nil)
	mockClient.On("CancelMessage", id).Return(&common.SuccessResponse{Message: "Canceled"}, nil)
	mockClient.On("GetAccountID").Return(testAccountID).Maybe()

	_, err := testutil.RunWithClient(t, mockClient, NewGetCommand(), "plain", id)
	require.NoError(t, err)
	_, err = testutil.RunWithClient(t, mockClient, NewCancelCommand(), "plain", id, "--force")
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
				assert.Equal(t, fmt.Sprintf("invalid message ID value %q: not a UUID", id), err.Error())
				assert.Equal(t, 4, clierrors.GetExitCode(err))
				assert.False(t, *called)
			})
		}
	}
}
