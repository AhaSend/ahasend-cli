//go:build unix

package listen

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListen_CtrlCEndsTheWaitForQueuedEvents checks that Ctrl+C still works
// while the command waits for the events queued when the stream ended.
func TestListen_CtrlCEndsTheWaitForQueuedEvents(t *testing.T) {
	endpointURL, received := stoppedEndpoint(t)
	server := streamServerFunc(t, func(conn *websocket.Conn) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(openedEventNumber(1)))
		<-received
		_ = conn.WriteMessage(websocket.TextMessage, []byte(openedEventNumber(2)))
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_, _, _ = conn.ReadMessage()
	})
	cmd, _, stderr := listenCommand(t)
	done := make(chan error, 1)

	go func() {
		done <- Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", WsURL: server, ForwardTo: endpointURL})
	}()
	waitForOutput(t, stderr, "Waiting for the 2 events not yet forwarded (Ctrl+C to skip)\n")
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))

	select {
	case err := <-done:
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(stderr.String(), "\n\nStopping webhook listener...\n"), stderr.String())
	case <-time.After(5 * time.Second):
		t.Fatal("Ctrl+C did not end the wait")
	}
}

// TestListen_CtrlCDuringTheHandshakeStopsTheCommand checks that Ctrl+C while
// the stream server has not answered the handshake ends the command with no
// error, after the note, such as how to delete a temporary webhook, was
// shown.
func TestListen_CtrlCDuringTheHandshakeStopsTheCommand(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	// The test's own handler keeps a SIGINT that Listen does not handle
	// from ending the test binary.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGINT)
	t.Cleanup(func() { signal.Stop(guard) })
	cmd, stdout, stderr := listenCommand(t)
	done := make(chan error, 1)

	go func() {
		done <- Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook",
			WsURL: "ws" + strings.TrimPrefix(server.URL, "http"), Note: "Note: temporary webhook wh-1 stays"})
	}()
	waitForOutput(t, stderr, "Note: temporary webhook wh-1 stays\n")
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))

	select {
	case err := <-done:
		require.NoError(t, err)
		assert.Equal(t, "Note: temporary webhook wh-1 stays\n\n\nStopping webhook listener...\n", stderr.String())
		assert.Empty(t, stdout.String())
	case <-time.After(5 * time.Second):
		t.Fatal("Ctrl+C did not stop the handshake")
	}
}
