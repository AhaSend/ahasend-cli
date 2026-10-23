package listen

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/client"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// streamServer serves a websocket stream that sends messages and then closes
// the connection with code, and returns its websocket URL.
func streamServer(t *testing.T, code int, messages ...string) string {
	t.Helper()
	server := httptest.NewServer(streamHandler(code, messages...))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

// streamServerFunc serves a websocket stream that handle drives, and returns
// its websocket URL.
func streamServerFunc(t *testing.T, handle func(conn *websocket.Conn)) string {
	t.Helper()
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn)
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

// streamHandler upgrades a request to a websocket, sends messages and then
// closes the connection with code.
func streamHandler(code int, messages ...string) http.Handler {
	upgrader := websocket.Upgrader{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for _, message := range messages {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
				return
			}
		}
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, ""))
		// Wait for the client to close its side.
		_, _, _ = conn.ReadMessage()
	})
}

func streamClient(t *testing.T) client.AhaSendClient {
	t.Helper()
	apiClient, err := client.NewClient("test-key", "11111111-1111-4111-8111-111111111111", "http://127.0.0.1:1")
	require.NoError(t, err)
	return apiClient
}

const (
	connectedMessage = `{"type":"connected","session_id":"sess-1","timestamp":0}`
	openedEvent      = `{"type":"event","timestamp":0,"event":{"data":{"type":"message.opened","recipient":"a@example.com"}}}`
	clickedEvent     = `{"type":"event","timestamp":0,"event":{"data":{"type":"message.clicked","recipient":"b@example.com"}}}`
	// replayedEvent is a replay frame as the stream server sends it: without
	// a timestamp of its own, and with the event's in nanoseconds.
	replayedEvent = `{"type":"replay","replayed":true,"event":{"type":"webhook","stream_id":"wh-1","account_id":"acc-1",` +
		`"data":{"type":"message.opened","recipient":"c@example.com"},"metadata":null,"timestamp":1791547200123456789}}`
)

func TestListen_ShowsTheEventTimeOfAReplay(t *testing.T) {
	wsURL := streamServer(t, websocket.CloseNormalClosure, replayedEvent)
	cmd, stdout, _ := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", WsURL: wsURL, Waiting: "events", SlimOutput: true})

	require.NoError(t, err)
	stamped := time.Unix(0, 1791547200123456789).Format("15:04:05")
	assert.Contains(t, stdout.String(), "["+stamped+"] REPLAY: message.opened\n")
}

func TestListen_ShowsKeptEventsUntilTheServerCloses(t *testing.T) {
	wsURL := streamServer(t, websocket.CloseNormalClosure, connectedMessage, openedEvent, clickedEvent, replayedEvent)
	cmd, stdout, stderr := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{
		Kind:       "webhook",
		Title:      "Webhook",
		ID:         "wh-1",
		WsURL:      wsURL,
		Waiting:    "events",
		Keep:       func(eventType string) bool { return eventType == "message.opened" },
		SlimOutput: true,
	})

	require.NoError(t, err)
	out := stdout.String()
	assert.Contains(t, out, "Webhook connected\n\nWebhook ID: wh-1\nForwarding secret (signs forwarded events): aha-whsec-")
	assert.Contains(t, out, "Listening for events... (Press Ctrl+C to stop)\n")
	assert.Contains(t, out, "Session established: sess-1\n")
	assert.Contains(t, out, "] message.opened\n")
	assert.Contains(t, out, "] REPLAY: message.opened\n")
	assert.NotContains(t, out, "message.clicked")
	assert.Equal(t, "\nWebSocket connection closed\n", stderr.String())
}

// connectedWithWarnings is the connected frame of a webhook stream whose API
// key lacks the read scope of two event families the webhook turned on.
const connectedWithWarnings = `{"type":"connected","session_id":"sess-1","stream":"webhook:wh-1","timestamp":0,` +
	`"warnings":["The API key does not have the suppressions:read scope, so suppression events are not sent to this stream",` +
	`"The API key does not have the domains:read scope, so domain events are not sent to this stream"]}`

func TestListen_PrintsTheWarningsOfTheConnectedFrameOnStderr(t *testing.T) {
	wsURL := streamServer(t, websocket.CloseNormalClosure, connectedWithWarnings, openedEvent)
	cmd, stdout, stderr := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", WsURL: wsURL, Waiting: "events", SlimOutput: true})

	require.NoError(t, err)
	assert.Equal(t, "Warning: The API key does not have the suppressions:read scope, so suppression events are not sent to this stream\n"+
		"Warning: The API key does not have the domains:read scope, so domain events are not sent to this stream\n"+
		"\nWebSocket connection closed\n", stderr.String())
	assert.Contains(t, stdout.String(), "Session established: sess-1\n")
	assert.Contains(t, stdout.String(), "] message.opened\n")
	assert.NotContains(t, stdout.String(), "Warning")
}

func TestListen_PrintsTheWarningsOfEveryConnectedFrame(t *testing.T) {
	wsURL := streamServer(t, websocket.CloseNormalClosure, connectedWithWarnings, connectedWithWarnings)
	cmd, _, stderr := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", WsURL: wsURL, Waiting: "events"})

	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(stderr.String(), "Warning: The API key does not have the suppressions:read scope"))
}

func TestListen_PrintsNoWarningForAConnectedFrameWithoutWarnings(t *testing.T) {
	wsURL := streamServer(t, websocket.CloseNormalClosure, connectedMessage, `{"type":"connected","session_id":"sess-2","timestamp":0,"warnings":[]}`)
	cmd, stdout, stderr := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", WsURL: wsURL, Waiting: "events"})

	require.NoError(t, err)
	assert.Equal(t, "\nWebSocket connection closed\n", stderr.String())
	assert.NotContains(t, stdout.String(), "Warning")
}

func TestListen_EscapesTheWarningsOfTheConnectedFrame(t *testing.T) {
	wsURL := streamServer(t, websocket.CloseNormalClosure, `{"type":"connected","session_id":"sess-1","timestamp":0,"warnings":["no scope\u001b]0;x\u0007"]}`)
	cmd, _, stderr := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", WsURL: wsURL, Waiting: "events"})

	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "Warning: no scope")
	assert.NotContains(t, stderr.String(), "\x1b")
	assert.NotContains(t, stderr.String(), "\x07")
}

func TestListen_ShowsAnEventWithMetadata(t *testing.T) {
	event := `{"type":"event","timestamp":0,"event":{"type":"webhook","stream_id":"wh-1","account_id":"acc-1",` +
		`"data":{"type":"message.opened","recipient":"a@example.com"},"metadata":{"domain":"example.com"},"timestamp":1}}`
	wsURL := streamServer(t, websocket.CloseNormalClosure, event)
	cmd, stdout, stderr := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", WsURL: wsURL, Waiting: "events", SlimOutput: true})

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "] message.opened\n")
	assert.Equal(t, "\nWebSocket connection closed\n", stderr.String())
}

func TestListen_FullOutputPrintsTheEventData(t *testing.T) {
	wsURL := streamServer(t, websocket.CloseGoingAway, openedEvent)
	cmd, stdout, _ := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "route", Title: "Route", WsURL: wsURL, Waiting: "inbound emails"})

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "] message.opened\n  {\n    \"type\": \"message.opened\",\n    \"recipient\": \"a@example.com\"\n  }\n")
}

func TestListen_SlimDetailsFollowTheEventType(t *testing.T) {
	wsURL := streamServer(t, websocket.CloseNormalClosure, openedEvent)
	cmd, stdout, _ := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{
		Kind:       "route",
		Title:      "Route",
		WsURL:      wsURL,
		SlimOutput: true,
		SlimDetails: func(out io.Writer, data map[string]interface{}) {
			_, _ = io.WriteString(out, " to: "+data["recipient"].(string))
		},
	})

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "] message.opened to: a@example.com\n")
}

// TestListen_EscapesControlCharactersInTheEventType checks that an event
// type cannot send escape sequences to the terminal.
func TestListen_EscapesControlCharactersInTheEventType(t *testing.T) {
	event := `{"type":"event","timestamp":0,"event":{"data":{"type":"message.routing\u001b[2J\u0007"}}}`
	for _, slim := range []bool{true, false} {
		wsURL := streamServer(t, websocket.CloseNormalClosure, event)
		cmd, stdout, _ := confirmCommand(t, "")

		err := Listen(cmd, streamClient(t), Listener{Kind: "route", Title: "Route", WsURL: wsURL, SlimOutput: slim})

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), `] message.routing\u001b[2J\u0007`+"\n")
		assert.NotContains(t, stdout.String(), "\x1b[2J")
	}
}

// TestListen_FullOutputEscapesC1Characters checks that the event data, which
// JSON leaves C1 and DEL characters raw in, cannot send them to the terminal.
func TestListen_FullOutputEscapesC1Characters(t *testing.T) {
	event := "{\"type\":\"event\",\"timestamp\":0,\"event\":{\"data\":{\"type\":\"message.routing\",\"subject\":\"Hi\u009b2J\x7f\"}}}"
	wsURL := streamServer(t, websocket.CloseNormalClosure, event)
	cmd, stdout, _ := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "route", Title: "Route", WsURL: wsURL})

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"subject": "Hi\u009b2J\u007f"`)
	assert.NotContains(t, stdout.String(), "\u009b")
	assert.NotContains(t, stdout.String(), "\x7f")
}

// TestListen_SkipsMessagesItCannotRead checks that a text message that is
// not JSON and a binary message are skipped with a warning, and the events
// after them still show.
func TestListen_SkipsMessagesItCannotRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("not json"))
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte{0x01, 0x02})
		_ = conn.WriteMessage(websocket.TextMessage, []byte(openedEvent))
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_, _, _ = conn.ReadMessage()
	}))
	t.Cleanup(server.Close)
	cmd, stdout, stderr := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook",
		WsURL: "ws" + strings.TrimPrefix(server.URL, "http"), SlimOutput: true})

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "] message.opened\n")
	assert.Contains(t, stderr.String(), "Warning: skipped a webhook stream message: stream message cannot be read: a text message that is not JSON: ")
	assert.Contains(t, stderr.String(), "Warning: skipped a webhook stream message: stream message cannot be read: a binary message of 2 bytes\n")
	assert.True(t, strings.HasSuffix(stderr.String(), "\nWebSocket connection closed\n"), stderr.String())
}

// TestListen_EscapesTheWarningOfASkippedMessage checks that the warning of a
// message the stream sent that cannot be read carries none of the control
// characters the server put in it, such as in a metadata key. Go 1.27's
// decoder names the key in its error, so the warning shows it escaped; Go
// 1.26's does not name it.
func TestListen_EscapesTheWarningOfASkippedMessage(t *testing.T) {
	message := `{"type":"event","event":{"metadata":{"\u001b]0;x\u0007":5}}}`
	wsURL := streamServer(t, websocket.CloseNormalClosure, message)
	cmd, _, stderr := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", WsURL: wsURL})

	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "Warning: skipped a webhook stream message: stream message cannot be read: ")
	assert.NotContains(t, stderr.String(), "\x1b")
	assert.NotContains(t, stderr.String(), "\x07")
}

func TestListen_ForwardsSignedEvents(t *testing.T) {
	type forwarded struct {
		header http.Header
		body   string
	}
	received := make(chan forwarded, 2)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- forwarded{header: r.Header.Clone(), body: string(body)}
	}))
	t.Cleanup(endpoint.Close)
	wsURL := streamServer(t, websocket.CloseNormalClosure, openedEvent, clickedEvent)
	cmd, stdout, _ := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{
		Kind:      "webhook",
		Title:     "Webhook",
		WsURL:     wsURL,
		Keep:      func(eventType string) bool { return eventType == "message.opened" },
		ForwardTo: endpoint.URL,
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Forwarding to: "+endpoint.URL+"\n")

	var got forwarded
	select {
	case got = <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("the event was not forwarded")
	}
	assert.Equal(t, `{"type":"message.opened","recipient":"a@example.com"}`, got.body)
	assert.Equal(t, "application/json", got.header.Get("Content-Type"))

	secret := regexp.MustCompile(`Forwarding secret \(signs forwarded events\): (\S+)`).FindStringSubmatch(stdout.String())[1]
	unix, err := strconv.ParseInt(got.header.Get("webhook-timestamp"), 10, 64)
	require.NoError(t, err)
	signature, err := webhooks.NewSigner(secret).Sign(got.header.Get("webhook-id"), time.Unix(unix, 0), []byte(got.body))
	require.NoError(t, err)
	assert.Equal(t, signature, got.header.Get("webhook-signature"))

	select {
	case extra := <-received:
		t.Fatalf("forwarded an event the listener does not keep: %s", extra.body)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestListen_SkipVerifyStillVerifiesTheStream(t *testing.T) {
	server := httptest.NewTLSServer(streamHandler(websocket.CloseNormalClosure, connectedMessage))
	t.Cleanup(server.Close)
	cmd, _, _ := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{
		Kind:       "webhook",
		Title:      "Webhook",
		WsURL:      "wss" + strings.TrimPrefix(server.URL, "https"),
		SkipVerify: true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to verify certificate")
}

func TestListen_SkipVerifyForwardsToAnUnverifiedEndpoint(t *testing.T) {
	received := make(chan string, 1)
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- string(body)
	}))
	t.Cleanup(endpoint.Close)
	wsURL := streamServer(t, websocket.CloseNormalClosure, openedEvent)
	cmd, _, _ := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{
		Kind:       "webhook",
		Title:      "Webhook",
		WsURL:      wsURL,
		ForwardTo:  endpoint.URL,
		SkipVerify: true,
	})
	require.NoError(t, err)

	select {
	case body := <-received:
		assert.Equal(t, `{"type":"message.opened","recipient":"a@example.com"}`, body)
	case <-time.After(5 * time.Second):
		t.Fatal("the event was not forwarded")
	}
}

// TestEventForwarder_SkipVerifyKeepsTheDefaultTransport checks that skipping
// verification keeps the proxy and HTTP/2 settings of the default transport.
func TestEventForwarder_SkipVerifyKeepsTheDefaultTransport(t *testing.T) {
	forwarder := newEventForwarder("webhook", "https://localhost:3000", "c2VjcmV0", true)

	transport, ok := forwarder.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
	assert.NotNil(t, transport.Proxy)
	assert.True(t, transport.ForceAttemptHTTP2)
	assert.Equal(t, http.DefaultTransport.(*http.Transport).TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
}

// TestListen_AFailedStreamIsANetworkError checks that a stream that drops,
// or that the server closes with an error code, fails with exit code 7,
// while a normal close ends the command without an error.
func TestListen_AFailedStreamIsANetworkError(t *testing.T) {
	dropped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte(connectedMessage))
		_ = conn.NetConn().Close() // no close frame: the client sees 1006
	}))
	t.Cleanup(dropped.Close)
	tests := map[string]struct {
		wsURL string
		code  int
	}{
		"dropped connection": {"ws" + strings.TrimPrefix(dropped.URL, "http"), 7},
		"server error close": {streamServer(t, websocket.CloseInternalServerErr, connectedMessage), 7},
		"normal close":       {streamServer(t, websocket.CloseNormalClosure, connectedMessage), 0},
		"going away":         {streamServer(t, websocket.CloseGoingAway, connectedMessage), 0},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cmd, _, _ := confirmCommand(t, "")

			err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", WsURL: test.wsURL})

			if test.code == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, test.code, clierrors.GetExitCode(err), err.Error())
			assert.Contains(t, err.Error(), "websocket error: websocket connection failed")
		})
	}
}

// TestListen_ForwardsEventsInOrderAndByteForByte checks that events reach
// the endpoint in the order they arrived, however slow the endpoint is, with
// their data exactly as the server sent it: key order, large numbers and
// HTML characters kept.
func TestListen_ForwardsEventsInOrderAndByteForByte(t *testing.T) {
	var events []string
	var want []string
	for i := range 20 {
		data := fmt.Sprintf(`{"type":"message.opened","seq":%d,"id":12345678901234567891,"html":"<a href=\"x\">&</a>","z":1,"a":2}`, i)
		events = append(events, `{"type":"event","timestamp":0,"event":{"data":`+data+`}}`)
		want = append(want, data)
	}
	var mu sync.Mutex
	var bodies []string
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"seq":0,`) {
			time.Sleep(50 * time.Millisecond) // a slow first answer must not let later events pass it
		}
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
	}))
	t.Cleanup(endpoint.Close)
	cmd, _, _ := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook",
		WsURL: streamServer(t, websocket.CloseNormalClosure, events...), ForwardTo: endpoint.URL})

	require.NoError(t, err)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, want, bodies)
}

// syncBuffer is a buffer a command writes to while a test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// listenCommand returns a command whose output a test can read while Listen
// runs.
func listenCommand(t *testing.T) (*cobra.Command, *syncBuffer, *syncBuffer) {
	t.Helper()
	cmd := parsed(t, prompt.AddForceFlag)
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd, stdout, stderr
}

// waitForOutput waits until out holds text.
func waitForOutput(t *testing.T, out *syncBuffer, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(out.String(), text) {
		if time.Now().After(deadline) {
			t.Fatalf("the output never held %q: %s", text, out.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// stoppedEndpoint is a forwarding endpoint that answers no request until the
// test ends. It sends on received when its first request arrives.
func stoppedEndpoint(t *testing.T) (url string, received chan struct{}) {
	t.Helper()
	received = make(chan struct{}, 1)
	release := make(chan struct{})
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- struct{}{}:
		default:
		}
		<-release
	}))
	t.Cleanup(endpoint.Close)
	t.Cleanup(func() { close(release) })
	return endpoint.URL, received
}

// openedEventNumber is an event of type message.opened with seq number.
func openedEventNumber(number int) string {
	return fmt.Sprintf(`{"type":"event","timestamp":0,"event":{"data":{"type":"message.opened","seq":%d}}}`, number)
}

// TestListen_AStoppedEndpointHoldsUpNeitherTheOutputNorThePings checks that
// an endpoint that does not answer keeps no event from printing and no ping
// from its pong: the events that find the queue full are not forwarded, with
// a warning. When the stream ends, the command says how many events wait.
func TestListen_AStoppedEndpointHoldsUpNeitherTheOutputNorThePings(t *testing.T) {
	endpointURL, received := stoppedEndpoint(t)
	pong := make(chan bool, 1)
	errPong := stderrors.New("pong")
	wsURL := streamServerFunc(t, func(conn *websocket.Conn) {
		// The first event is being forwarded before the others arrive, so
		// the queue then holds 100 of them and the last 2 find it full
		_ = conn.WriteMessage(websocket.TextMessage, []byte(openedEventNumber(0)))
		<-received
		for i := 1; i <= forwardQueueSize+2; i++ {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(openedEventNumber(i)))
		}
		answered := false
		conn.SetPongHandler(func(string) error {
			answered = true
			return errPong
		})
		_ = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second))
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, _, _ = conn.ReadMessage()
		pong <- answered
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	})
	cmd, stdout, stderr := listenCommand(t)

	go func() {
		_ = Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook",
			WsURL: wsURL, ForwardTo: endpointURL, SlimOutput: true})
	}()

	select {
	case answered := <-pong:
		require.True(t, answered, "the ping got no pong")
	case <-time.After(10 * time.Second):
		t.Fatal("the stream did not finish")
	}
	waitForOutput(t, stderr, "Waiting for the 101 events not yet forwarded (Ctrl+C to skip)\n")
	assert.Equal(t, forwardQueueSize+3, strings.Count(stdout.String(), "] message.opened\n"))
	assert.Equal(t, 2, strings.Count(stderr.String(),
		"Warning: did not forward the message.opened event: 100 events are already waiting for "+endpointURL+"\n"))
	assert.Contains(t, stderr.String(),
		"Did not forward 2 events, as the queue for "+endpointURL+" was full: 2 message.opened\n")
}

// newTestForwarder returns a forwarder to url that forwards nothing, as its
// run never starts, and whose clock is *now.
func newTestForwarder(now *time.Time) *eventForwarder {
	f := newEventForwarder("webhook", "http://127.0.0.1:1/hook", "whsec_dGVzdA==", false)
	f.now = func() time.Time { return *now }
	return f
}

// TestEventForwarder_WarnsOfTheFirstDroppedEventsThenCountsTheRest checks
// that each of the first events not forwarded gets a warning, the later ones
// a count at most every few seconds, and that the end of the stream counts
// them all by type.
func TestEventForwarder_WarnsOfTheFirstDroppedEventsThenCountsTheRest(t *testing.T) {
	now := time.Now()
	f := newTestForwarder(&now)
	var stderr bytes.Buffer
	for range forwardQueueSize {
		f.enqueue("message.opened", json.RawMessage(`{}`), &stderr)
	}
	require.Empty(t, stderr.String())

	for i := range 20 {
		eventType := "message.opened"
		if i%2 == 1 {
			eventType = "message.clicked"
		}
		f.enqueue(eventType, json.RawMessage(`{}`), &stderr)
	}
	now = now.Add(droppedReportInterval)
	f.enqueue("message.opened", json.RawMessage(`{}`), &stderr)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.finish(ctx, &stderr)

	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	require.Len(t, lines, droppedWarnings+2, stderr.String())
	assert.Equal(t, "Warning: did not forward the message.clicked event: 100 events are already waiting for http://127.0.0.1:1/hook", lines[1])
	assert.Equal(t, "Warning: did not forward 16 more events: 100 events are already waiting for http://127.0.0.1:1/hook", lines[droppedWarnings])
	assert.Equal(t, "Did not forward 21 events, as the queue for http://127.0.0.1:1/hook was full: 10 message.clicked, 11 message.opened",
		lines[droppedWarnings+1])
}

// TestEventForwarder_BoundsTheQueueByItsData checks that an event whose data
// would take the queue over forwardQueueBytes is not forwarded, however few
// events wait.
func TestEventForwarder_BoundsTheQueueByItsData(t *testing.T) {
	now := time.Now()
	f := newTestForwarder(&now)
	var stderr bytes.Buffer

	f.enqueue("message.opened", make(json.RawMessage, forwardQueueBytes-10), &stderr)
	f.enqueue("message.clicked", make(json.RawMessage, 11), &stderr)
	f.enqueue("message.bounced", make(json.RawMessage, 10), &stderr)

	assert.Equal(t, "Warning: did not forward the message.clicked event: 32 MiB of events are already waiting for http://127.0.0.1:1/hook\n",
		stderr.String())
	assert.Equal(t, int64(2), f.pending.Load())
}

// TestListen_WaitsForTheQueuedEventsWhenTheStreamEnds checks that the
// command returns once the events still queued are forwarded.
func TestListen_WaitsForTheQueuedEventsWhenTheStreamEnds(t *testing.T) {
	var forwarded atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		forwarded.Add(1)
	}))
	t.Cleanup(endpoint.Close)
	cmd, _, stderr := listenCommand(t)

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", ForwardTo: endpoint.URL,
		WsURL: streamServer(t, websocket.CloseNormalClosure, openedEventNumber(1), openedEventNumber(2), openedEventNumber(3))})

	require.NoError(t, err)
	assert.Equal(t, int32(3), forwarded.Load())
	assert.Regexp(t, `Waiting for the [1-3] events? not yet forwarded \(Ctrl\+C to skip\)`, stderr.String())
}

func TestListen_EscapesTheSessionAndTheStreamID(t *testing.T) {
	wsURL := streamServer(t, websocket.CloseNormalClosure, `{"type":"connected","session_id":"sess\u001b]0;x\u0007","timestamp":0}`)
	cmd, stdout, _ := confirmCommand(t, "")

	err := Listen(cmd, streamClient(t), Listener{Kind: "webhook", Title: "Webhook", ID: "wh\x1b[2J", WsURL: wsURL, Waiting: "events"})

	require.NoError(t, err)
	out := stdout.String()
	assert.Contains(t, out, `Webhook ID: wh\u001b[2J`+"\n")
	assert.Contains(t, out, `Session established: sess\u001b]0;x\u0007`+"\n")
	assert.NotContains(t, out, "\x1b")
	assert.NotContains(t, out, "\x07")
}

// listenUntilDone runs Listen with l, whose stdout is out, and returns its
// error; it fails the test when Listen does not return within a few seconds.
func listenUntilDone(t *testing.T, out io.Writer, l Listener) error {
	t.Helper()
	cmd, _, _ := confirmCommand(t, "")
	cmd.SetOut(out)
	done := make(chan error, 1)
	go func() { done <- Listen(cmd, streamClient(t), l) }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Listen went on after its stdout failed")
		return nil
	}
}

// closingStream serves a stream that runs before, sends an event and then
// waits for the client to close the connection, which it reports on closed.
func closingStream(t *testing.T, before func()) (string, chan bool) {
	t.Helper()
	closed := make(chan bool, 1)
	wsURL := streamServerFunc(t, func(conn *websocket.Conn) {
		before()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(openedEvent))
		_, _, err := conn.ReadMessage()
		closed <- websocket.IsCloseError(err, websocket.CloseNormalClosure)
	})
	return wsURL, closed
}

// TestListen_StopsWhenStdoutIsAClosedPipe checks that a listener whose
// stdout is a pipe without a reader, as in `ahasend webhooks listen | head`
// once head exits, closes the stream and fails with the write error, which
// exits 1, rather than listening on with SIGPIPE ignored.
func TestListen_StopsWhenStdoutIsAClosedPipe(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })
	// The banner fits in the pipe; the reader exits before the event comes.
	wsURL, closed := closingStream(t, func() { _ = reader.Close() })

	err = listenUntilDone(t, writer, Listener{Kind: "webhook", Title: "Webhook", WsURL: wsURL, SlimOutput: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to print the webhook stream: ")
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	assert.True(t, <-closed, "the listener closes the stream")
}

func TestListen_StopsWhenTheBannerCannotBeWritten(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })
	require.NoError(t, reader.Close())
	wsURL, closed := closingStream(t, func() {})

	err = listenUntilDone(t, writer, Listener{Kind: "route", Title: "Route", WsURL: wsURL})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to print the route stream: ")
	assert.True(t, <-closed, "the listener closes the stream")
}
