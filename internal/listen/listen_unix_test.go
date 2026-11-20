//go:build unix

package listen

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catchSignals registers the test for signals, so sendSignal can wait until
// one has arrived, and returns the channel the test receives them on. While
// it is registered, a signal that Listen does not handle does not end the
// test binary.
func catchSignals(t *testing.T, signals ...os.Signal) chan os.Signal {
	t.Helper()
	caught := make(chan os.Signal, 4)
	signal.Notify(caught, signals...)
	t.Cleanup(func() { signal.Stop(caught) })
	return caught
}

// sendSignal sends sig to the test binary and waits until caught has it.
func sendSignal(t *testing.T, caught chan os.Signal, sig syscall.Signal) {
	t.Helper()
	require.NoError(t, syscall.Kill(syscall.Getpid(), sig))
	select {
	case <-caught:
	case <-time.After(5 * time.Second):
		t.Fatalf("%v never arrived", sig)
	}
}

// sendSignalToListenAlone stops the test's own registration for signals,
// then sends sig to the test binary: only Listen still handles it, so a
// Listen that has stopped handling signals lets sig end the test binary.
func sendSignalToListenAlone(t *testing.T, caught chan os.Signal, sig syscall.Signal) {
	t.Helper()
	signal.Stop(caught)
	require.NoError(t, syscall.Kill(syscall.Getpid(), sig))
}

// waitUntil waits until ch receives or is closed; it fails the test, naming
// what, when that takes more than a few seconds.
func waitUntil(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not happen", what)
	}
}

// listenResult returns the error Listen sends on done; it fails the test
// when Listen has not returned after a few seconds.
func listenResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Listen did not return")
		return nil
	}
}

// assertNotReturned fails the test when Listen returns on done within a
// short while.
func assertNotReturned(t *testing.T, done <-chan error, why string) {
	t.Helper()
	select {
	case <-done:
		t.Fatalf("Listen returned: %s", why)
	case <-time.After(100 * time.Millisecond):
	}
}

// stalledHandshake serves a stream that never answers the handshake, and
// returns its websocket URL and a channel that receives when the request
// arrives.
func stalledHandshake(t *testing.T) (string, chan struct{}) {
	t.Helper()
	requested := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		select {
		case requested <- struct{}{}:
		default:
		}
		<-release
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	return "ws" + strings.TrimPrefix(server.URL, "http"), requested
}

// openStream serves a stream that sends the connected frame and then waits
// for the client to close the connection, and returns its websocket URL.
func openStream(t *testing.T) string {
	t.Helper()
	return streamServerFunc(t, func(conn *websocket.Conn) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(connectedMessage))
		_, _, _ = conn.ReadMessage()
	})
}

// slowDelete is a DeleteTemporary that closes started when it is called and
// returns once the test calls release, or the test ends.
func slowDelete(t *testing.T) (del func() error, started chan struct{}, release func()) {
	t.Helper()
	started = make(chan struct{})
	released := make(chan struct{})
	release = sync.OnceFunc(func() { close(released) })
	t.Cleanup(release)
	del = func() error {
		close(started)
		<-released
		return nil
	}
	return del, started, release
}

// TestListen_CtrlCEndsTheWaitForQueuedEventsThenDeletes checks that Ctrl+C
// still works while the command waits for the events queued when the stream
// ended, and that the temporary webhook is deleted after the wait.
func TestListen_CtrlCEndsTheWaitForQueuedEventsThenDeletes(t *testing.T) {
	endpointURL, received := stoppedEndpoint(t)
	server := streamServerFunc(t, func(conn *websocket.Conn) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(openedEventNumber(1)))
		<-received
		_ = conn.WriteMessage(websocket.TextMessage, []byte(openedEventNumber(2)))
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_, _, _ = conn.ReadMessage()
	})
	del, calls := recordDeletes(nil)
	l := temporaryWebhook(del)
	l.WsURL = server
	l.ForwardTo = endpointURL
	cmd, _, stderr := listenCommand(t)
	apiClient := streamClient(t)
	done := make(chan error, 1)

	go func() { done <- Listen(cmd, apiClient, l) }()
	waitForOutput(t, stderr, "Waiting for the 2 events not yet forwarded (Ctrl+C to skip)\n")
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))

	require.NoError(t, listenResult(t, done))
	assert.Equal(t, int32(1), calls.Load())
	assert.True(t, strings.HasSuffix(stderr.String(), "\n\nStopping webhook listener...\nDeleted temporary webhook wh-1\n"), stderr.String())
}

func TestListen_CtrlCDuringTheHandshakeDeletesTheTemporaryWebhook(t *testing.T) {
	wsURL, requested := stalledHandshake(t)
	caught := catchSignals(t, syscall.SIGINT)
	del, calls := recordDeletes(nil)
	l := temporaryWebhook(del)
	l.WsURL = wsURL
	cmd, _, _ := listenCommand(t)
	apiClient := streamClient(t)
	done := make(chan error, 1)

	go func() { done <- Listen(cmd, apiClient, l) }()
	waitUntil(t, requested, "the handshake request")
	sendSignal(t, caught, syscall.SIGINT)

	require.NoError(t, listenResult(t, done))
	assert.Equal(t, int32(1), calls.Load())
}

// TestListen_CtrlCDuringTheHandshakeStopsTheCommand checks that Ctrl+C while
// the stream server has not answered the handshake ends the command with no
// error.
func TestListen_CtrlCDuringTheHandshakeStopsTheCommand(t *testing.T) {
	wsURL, requested := stalledHandshake(t)
	caught := catchSignals(t, syscall.SIGINT)
	cmd, stdout, stderr := listenCommand(t)
	apiClient := streamClient(t)
	done := make(chan error, 1)

	go func() { done <- Listen(cmd, apiClient, Listener{Kind: "webhook", Title: "Webhook", WsURL: wsURL}) }()
	waitUntil(t, requested, "the handshake request")
	sendSignal(t, caught, syscall.SIGINT)

	require.NoError(t, listenResult(t, done))
	assert.Equal(t, "\n\nStopping webhook listener...\n", stderr.String())
	assert.Empty(t, stdout.String())
}

// TestListen_CtrlCWhileListeningDeletesAfterClosingTheStream checks that the
// stream's connection is closed before the temporary webhook is deleted.
func TestListen_CtrlCWhileListeningDeletesAfterClosingTheStream(t *testing.T) {
	closed := make(chan bool, 1)
	wsURL := streamServerFunc(t, func(conn *websocket.Conn) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(connectedMessage))
		_, _, err := conn.ReadMessage()
		closed <- websocket.IsCloseError(err, websocket.CloseNormalClosure)
	})
	closedAtDelete := make(chan bool, 1)
	l := temporaryWebhook(func() error {
		select {
		case wasClosed := <-closed:
			closedAtDelete <- wasClosed
		case <-time.After(time.Second):
			closedAtDelete <- false
		}
		return nil
	})
	l.WsURL = wsURL
	caught := catchSignals(t, syscall.SIGINT)
	cmd, stdout, stderr := listenCommand(t)
	apiClient := streamClient(t)
	done := make(chan error, 1)

	go func() { done <- Listen(cmd, apiClient, l) }()
	waitForOutput(t, stdout, "Session established: sess-1\n")
	sendSignal(t, caught, syscall.SIGINT)

	require.NoError(t, listenResult(t, done))
	require.Len(t, closedAtDelete, 1, "the delete did not run")
	assert.True(t, <-closedAtDelete, "the stream was not closed when the delete ran")
	assert.True(t, strings.HasSuffix(stderr.String(), "Deleted temporary webhook wh-1\n"), stderr.String())
}

func TestListen_SIGTERMWhileListeningDeletesTheTemporaryWebhook(t *testing.T) {
	del, calls := recordDeletes(nil)
	l := temporaryWebhook(del)
	l.WsURL = openStream(t)
	caught := catchSignals(t, syscall.SIGTERM)
	cmd, stdout, _ := listenCommand(t)
	apiClient := streamClient(t)
	done := make(chan error, 1)

	go func() { done <- Listen(cmd, apiClient, l) }()
	waitForOutput(t, stdout, "Session established: sess-1\n")
	sendSignal(t, caught, syscall.SIGTERM)

	require.NoError(t, listenResult(t, done))
	assert.Equal(t, int32(1), calls.Load())
}

// TestListen_ASecondCtrlCDoesNotEndTheWaitForTheDelete checks that a second
// Ctrl+C, pressed while the delete runs, neither ends the wait for it nor
// prints a second Stopping line.
func TestListen_ASecondCtrlCDoesNotEndTheWaitForTheDelete(t *testing.T) {
	del, started, release := slowDelete(t)
	l := temporaryWebhook(del)
	l.WsURL = openStream(t)
	caught := catchSignals(t, syscall.SIGINT)
	cmd, stdout, stderr := listenCommand(t)
	apiClient := streamClient(t)
	done := make(chan error, 1)

	go func() { done <- Listen(cmd, apiClient, l) }()
	waitForOutput(t, stdout, "Session established: sess-1\n")
	sendSignal(t, caught, syscall.SIGINT)
	waitUntil(t, started, "the delete")
	sendSignalToListenAlone(t, caught, syscall.SIGINT)
	assertNotReturned(t, done, "the second Ctrl+C ended the wait for the delete")
	release()

	require.NoError(t, listenResult(t, done))
	assert.Equal(t, 1, strings.Count(stderr.String(), "Stopping webhook listener..."), stderr.String())
	assert.True(t, strings.HasSuffix(stderr.String(), "Deleted temporary webhook wh-1\n"), stderr.String())
}

// TestListen_CtrlCDuringTheDeleteAfterTheStreamClosedDoesNothing checks that
// Ctrl+C, pressed while the delete runs after the server closed the stream,
// neither ends the wait for the delete nor prints the Stopping line.
func TestListen_CtrlCDuringTheDeleteAfterTheStreamClosedDoesNothing(t *testing.T) {
	del, started, release := slowDelete(t)
	l := temporaryWebhook(del)
	l.WsURL = streamServer(t, websocket.CloseNormalClosure, connectedMessage)
	caught := catchSignals(t, syscall.SIGINT)
	cmd, _, stderr := listenCommand(t)
	apiClient := streamClient(t)
	done := make(chan error, 1)

	go func() { done <- Listen(cmd, apiClient, l) }()
	waitUntil(t, started, "the delete")
	sendSignalToListenAlone(t, caught, syscall.SIGINT)
	assertNotReturned(t, done, "Ctrl+C ended the wait for the delete")
	release()

	require.NoError(t, listenResult(t, done))
	assert.NotContains(t, stderr.String(), "Stopping")
	assert.True(t, strings.HasSuffix(stderr.String(), "Deleted temporary webhook wh-1\n"), stderr.String())
}
