//go:build unix

package messages

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// TestCancelSeveralMessagesStopsOnCtrlC checks that Ctrl+C during the cancel
// of several messages lets the request in flight finish, sends no other,
// shows the message it cancelled and exits 130.
func TestCancelSeveralMessagesStopsOnCtrlC(t *testing.T) {
	// The test's own handler keeps a SIGINT the command does not handle
	// from ending the test binary.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGINT)
	t.Cleanup(func() { signal.Stop(guard) })
	const first, second, third = "11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetAccountID").Return(testAccountID)
	mockClient.On("CancelMessage", first).
		Run(func(mock.Arguments) {
			require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))
			time.Sleep(50 * time.Millisecond) // the signal arrives while the request is in flight
		}).
		Return(&common.SuccessResponse{}, nil).Once()
	// A request after the interruption is answered, so it fails the
	// count below rather than the mock.
	mockClient.On("CancelMessage", mock.Anything).Return(&common.SuccessResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewCancelCommand(), "plain", "", first, second, third, "--force")

	require.Error(t, err)
	assert.Equal(t, "interrupted: 2 of 3 messages were not cancelled", err.Error())
	assert.Equal(t, 130, clierrors.GetExitCode(err))
	assert.True(t, strings.HasPrefix(stdout, "1 of 3 messages cancelled\n"), stdout)
	assert.Contains(t, stdout, first)
	assert.NotContains(t, stdout, second)
	mockClient.AssertNumberOfCalls(t, "CancelMessage", 1)
}
