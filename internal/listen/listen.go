// Package listen runs the listen commands of webhooks and routes: it
// connects to the AhaSend event stream, prints each event and forwards it,
// signed, to a local endpoint.
package listen

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Listener describes the stream a listen command shows and forwards.
type Listener struct {
	// Kind names the stream in messages, such as "webhook" or "route".
	Kind string
	// Title starts the connection banner, such as "Webhook".
	Title string
	// ID is the ID of the webhook or route the stream belongs to.
	ID string
	// WsURL is the websocket URL the stream API returned.
	WsURL string
	// Waiting names what the command listens for, such as "events".
	Waiting string
	// Details prints connection details after the ID line; nil prints none.
	Details func(out io.Writer)
	// Note is printed on stderr before connecting, so it is shown even when
	// connecting fails or is interrupted, such as how to delete the
	// temporary webhook the stream belongs to; "" prints nothing.
	Note string
	// Keep reports whether to show and forward an event of eventType; nil
	// keeps every event.
	Keep func(eventType string) bool
	// SlimDetails prints what a slim event line shows after the event type;
	// nil prints only the type. It passes every value from data through
	// printer.EscapeControl, since the stream's events carry text from email senders.
	SlimDetails func(out io.Writer, data map[string]interface{})

	ForwardTo string
	// SkipVerify skips TLS certificate verification when forwarding to
	// ForwardTo; the AhaSend stream is always verified.
	SkipVerify bool
	SlimOutput bool
}

// Listen connects to the stream of l, prints its banner and then every event
// until the user interrupts or the server closes the connection, forwarding
// each event to l.ForwardTo, signed with a secret it generates, when set.
// An interruption while connecting ends the command too, with no error. When
// stdout cannot be written, as when it is a pipe whose reader exited, the
// stream ends and Listen returns the write error.
func Listen(cmd *cobra.Command, apiClient client.AhaSendClient, l Listener) error {
	out := &stickyWriter{w: cmd.OutOrStdout()}
	errOut := cmd.ErrOrStderr()

	secret, err := webhooks.GenerateWebhookSecret()
	if err != nil {
		return fmt.Errorf("failed to generate webhook secret: %w", err)
	}

	// Handle interrupt signal, until the command returns
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		select {
		case <-sigChan:
			fmt.Fprintf(errOut, "\n\nStopping %s listener...\n", l.Kind)
			cancel()
		case <-ctx.Done():
		}
	}()

	if l.Note != "" {
		stderrColor(cmd, color.FgYellow).Fprintln(errOut, l.Note)
	}
	wsClient, err := connectUntilDone(ctx, cmd, apiClient, l)
	if err != nil || wsClient == nil {
		return err
	}
	defer wsClient.Close()

	printListenBanner(out, l, secret)
	if out.err != nil {
		return outputError(l, out.err)
	}

	// Events are forwarded one at a time, in the order they arrived, apart
	// from reading and printing the stream, which never wait for them. When
	// the stream ends, the command waits for the events still queued; an
	// interruption does not.
	var forwarder *eventForwarder
	if l.ForwardTo != "" {
		forwarder = newEventForwarder(l.Kind, l.ForwardTo, secret, l.SkipVerify)
		go forwarder.run()
	}

	err = showStream(ctx, wsClient, l, out, errOut, forwarder)
	if forwarder != nil {
		forwarder.finish(ctx, errOut)
	}
	return err
}

// connectUntilDone connects to the stream of l, unless ctx is done first, as
// on Ctrl+C during the handshake or the reconnect prompt: then it returns no
// connection and no error, and closes the connection if it is made later.
func connectUntilDone(ctx context.Context, cmd *cobra.Command, apiClient client.AhaSendClient, l Listener) (*client.WebSocketClient, error) {
	type connection struct {
		wsClient *client.WebSocketClient
		err      error
	}
	connected := make(chan connection, 1)
	go func() {
		wsClient, err := connectStream(cmd, apiClient, l.WsURL, l.Kind)
		connected <- connection{wsClient, err}
	}()

	select {
	case c := <-connected:
		if c.err != nil {
			return nil, fmt.Errorf("failed to connect to websocket: %w", c.err)
		}
		return c.wsClient, nil
	case <-ctx.Done():
		go func() {
			if c := <-connected; c.wsClient != nil {
				_ = c.wsClient.Close()
			}
		}()
		return nil, nil
	}
}

// showStream prints every event of the stream of wsClient, and queues it for
// forwarder when there is one, until ctx is cancelled, the stream ends or out
// fails to write.
func showStream(ctx context.Context, wsClient *client.WebSocketClient, l Listener, out *stickyWriter, errOut io.Writer, forwarder *eventForwarder) error {
	// Read in a goroutine, so the websocket library answers pings while an
	// event prints, and pass messages and the final error on one channel, so
	// every message received before the connection closed is shown.
	reads := make(chan streamRead, 10)
	go func() {
		defer close(reads)
		for {
			msg, err := wsClient.ReadMessage()
			select {
			case reads <- streamRead{msg: msg, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil && !stderrors.Is(err, client.ErrUndecodableMessage) {
				return
			}
		}
	}()

	for {
		var read streamRead
		var open bool
		select {
		case <-ctx.Done():
			return nil
		case read, open = <-reads:
		}
		if !open {
			return nil
		}

		if stderrors.Is(read.err, client.ErrStreamClosed) {
			fmt.Fprintln(errOut, "\nWebSocket connection closed")
			return nil
		}
		if stderrors.Is(read.err, client.ErrStreamFailed) {
			return errors.NewNetworkError("websocket error", read.err)
		}
		if stderrors.Is(read.err, client.ErrUndecodableMessage) {
			fmt.Fprintf(errOut, "Warning: skipped a %s stream message: %s\n", l.Kind, printer.EscapeControl(read.err.Error()))
			continue
		}
		if read.err != nil {
			return fmt.Errorf("websocket error: %w", read.err)
		}

		msg := read.msg
		switch msg.Type {
		case "connected":
			color.New(color.FgGreen).Fprintf(out, "Session established: %s\n", printer.EscapeControl(msg.SessionID))
			fmt.Fprintln(out, strings.Repeat("─", 60))
			for _, warning := range msg.Warnings {
				fmt.Fprintf(errOut, "Warning: %s\n", printer.EscapeControl(warning))
			}

		case "event", "replay":
			if msg.Event == nil {
				continue
			}
			var data map[string]interface{}
			_ = json.Unmarshal(msg.Event.Data, &data)
			eventType, _ := data["type"].(string)
			if l.Keep != nil && !l.Keep(eventType) {
				continue
			}

			displayEvent(out, msg, eventType, data, l)

			if forwarder != nil {
				forwarder.enqueue(eventType, msg.Event.Data, errOut)
			}

		default:
			logger.Get().WithField("type", msg.Type).Debug("Received unknown message type")
		}
		if out.err != nil {
			return outputError(l, out.err)
		}
	}
}

// outputError is the error of a stream of l whose output failed to write.
func outputError(l Listener, err error) error {
	return fmt.Errorf("failed to print the %s stream: %w", l.Kind, err)
}

// stickyWriter writes to w until a write fails; it then keeps that error in
// err and writes nothing more, so a stream stops printing at its first
// failed write.
type stickyWriter struct {
	w   io.Writer
	err error
}

func (s *stickyWriter) Write(p []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	n, err := s.w.Write(p)
	s.err = err
	return n, err
}

// streamRead is one result of reading the stream: a message or the error that
// ended it.
type streamRead struct {
	msg *client.WebSocketMessage
	err error
}

func printListenBanner(out io.Writer, l Listener, secret string) {
	fmt.Fprintln(out)
	color.New(color.FgGreen, color.Bold).Fprintln(out, l.Title+" connected")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s ID: %s\n", l.Title, color.YellowString(printer.EscapeControl(l.ID)))
	if l.Details != nil {
		l.Details(out)
	}
	fmt.Fprintf(out, "Forwarding secret (signs forwarded events): %s\n", color.YellowString(secret))
	if l.ForwardTo != "" {
		fmt.Fprintf(out, "Forwarding to: %s\n", color.CyanString(l.ForwardTo))
	}
	fmt.Fprintf(out, "Connected at: %s\n", color.GreenString(time.Now().Format("15:04:05")))
	fmt.Fprintln(out)
	color.New(color.FgWhite).Fprintln(out, "Listening for "+l.Waiting+"... (Press Ctrl+C to stop)")
	fmt.Fprintln(out, strings.Repeat("─", 60))
}

// eventTime returns when the stream sent msg: the frame's timestamp, in
// seconds, or for a replay frame, which has none, the time in nanoseconds the
// server stamped the event with.
func eventTime(msg *client.WebSocketMessage) time.Time {
	if msg.Timestamp == 0 && msg.Event != nil && msg.Event.Timestamp != 0 {
		return time.Unix(0, msg.Event.Timestamp)
	}
	return time.Unix(msg.Timestamp, 0)
}

func displayEvent(out io.Writer, msg *client.WebSocketMessage, eventType string, data map[string]interface{}, l Listener) {
	eventType = printer.EscapeControl(eventType)
	timestamp := eventTime(msg).Format("15:04:05")

	// Display based on message type (event vs replay)
	if msg.Type == "replay" {
		color.New(color.FgYellow).Fprintf(out, "[%s] REPLAY: ", timestamp)
	} else {
		color.New(color.FgCyan).Fprintf(out, "[%s] ", timestamp)
	}

	if l.SlimOutput {
		color.New(color.FgWhite, color.Bold).Fprint(out, eventType)
		if l.SlimDetails != nil && data != nil {
			l.SlimDetails(out, data)
		}
		fmt.Fprintln(out)
	} else {
		color.New(color.FgWhite, color.Bold).Fprintln(out, eventType)

		// Pretty print the data as the server sent it. JSON escapes C0
		// characters in strings but not DEL or C1 ones, which are escaped here.
		var jsonData bytes.Buffer
		if err := json.Indent(&jsonData, msg.Event.Data, "  ", "  "); err == nil {
			fmt.Fprintf(out, "  %s\n", printer.EscapeDisplayText(jsonData.String()))
		}
	}

	fmt.Fprintln(out, strings.Repeat("─", 60))
}

// eventForwarder posts events to a local endpoint, signed as the
// standard-webhooks specification describes, one at a time in the order
// they were queued.
type eventForwarder struct {
	kind       string
	url        string
	signer     *webhooks.Signer
	httpClient *http.Client
	// queue holds the payloads waiting to be forwarded; an event that finds
	// it full is not forwarded.
	queue chan json.RawMessage
	// pending counts the events not yet forwarded: those queued and the one
	// being forwarded. pendingBytes counts their data.
	pending      atomic.Int64
	pendingBytes atomic.Int64
	done         chan struct{}

	// The events not forwarded, which the stream's goroutine alone counts:
	// in all, by type, and since the last warning that counted them.
	dropped       int
	droppedByType map[string]int
	unreported    int
	lastReport    time.Time
	// now returns the current time; a test replaces it.
	now func() time.Time
}

// The most that waits for the endpoint: forwardQueueSize events, and
// forwardQueueBytes of their data, twice the largest stream message. An
// event that would go over either is not forwarded.
const (
	forwardQueueSize  = 100
	forwardQueueBytes = 32 << 20
)

// The warnings about events not forwarded: the first droppedWarnings events
// get one each; after them, a warning counts the events not forwarded since
// the last one, at most once per droppedReportInterval.
const (
	droppedWarnings       = 5
	droppedReportInterval = 5 * time.Second
)

func newEventForwarder(kind, url, secret string, skipVerify bool) *eventForwarder {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	if skipVerify {
		// A clone keeps the default proxy, HTTP/2 and timeout settings.
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		httpClient.Transport = transport
	}
	return &eventForwarder{
		kind:          kind,
		url:           url,
		signer:        webhooks.NewSigner(secret),
		httpClient:    httpClient,
		queue:         make(chan json.RawMessage, forwardQueueSize),
		done:          make(chan struct{}),
		droppedByType: map[string]int{},
		now:           time.Now,
	}
}

// run forwards the queued payloads until finish.
func (f *eventForwarder) run() {
	defer close(f.done)
	for payload := range f.queue {
		f.forward(payload)
		f.pending.Add(-1)
		f.pendingBytes.Add(-int64(len(payload)))
	}
}

// enqueue queues payload, the data of an event of eventType, to be
// forwarded. It never waits: when the queue is full, as when the endpoint is
// slow or stopped, the event is not forwarded and errOut says so, as
// droppedWarnings describes, so the stream goes on.
func (f *eventForwarder) enqueue(eventType string, payload json.RawMessage, errOut io.Writer) {
	size := int64(len(payload))
	// Only run lowers pendingBytes meanwhile, so the event still fits
	if f.pendingBytes.Load()+size > forwardQueueBytes {
		f.drop(eventType, fmt.Sprintf("%d MiB of events are already waiting", forwardQueueBytes>>20), errOut)
		return
	}
	f.pending.Add(1)
	f.pendingBytes.Add(size)
	select {
	case f.queue <- payload:
	default:
		f.pending.Add(-1)
		f.pendingBytes.Add(-size)
		f.drop(eventType, fmt.Sprintf("%d events are already waiting", forwardQueueSize), errOut)
	}
}

// drop counts an event of eventType that was not forwarded because of
// reason, and warns on errOut as droppedWarnings describes.
func (f *eventForwarder) drop(eventType, reason string, errOut io.Writer) {
	f.dropped++
	f.droppedByType[eventType]++
	if f.dropped <= droppedWarnings {
		fmt.Fprintf(errOut, "Warning: did not forward the %s event: %s for %s\n",
			printer.EscapeControl(eventType), reason, f.url)
		f.lastReport = f.now()
		return
	}
	f.unreported++
	if f.now().Sub(f.lastReport) < droppedReportInterval {
		return
	}
	fmt.Fprintf(errOut, "Warning: did not forward %s: %s for %s\n", countOf(f.unreported, "more event"), reason, f.url)
	f.unreported = 0
	f.lastReport = f.now()
}

// finish ends the queue, says on errOut how many events were not forwarded,
// by type, and waits until the events not yet forwarded are, saying how many
// there are, unless ctx is cancelled: an interruption, before or during the
// wait, ends it.
func (f *eventForwarder) finish(ctx context.Context, errOut io.Writer) {
	close(f.queue)
	if f.dropped > 0 {
		types := make([]string, 0, len(f.droppedByType))
		for eventType := range f.droppedByType {
			types = append(types, eventType)
		}
		sort.Strings(types)
		counts := make([]string, 0, len(types))
		for _, eventType := range types {
			counts = append(counts, fmt.Sprintf("%d %s", f.droppedByType[eventType], printer.EscapeControl(eventType)))
		}
		fmt.Fprintf(errOut, "Did not forward %s, as the queue for %s was full: %s\n",
			countOf(f.dropped, "event"), f.url, strings.Join(counts, ", "))
	}
	if ctx.Err() != nil {
		return
	}
	if pending := f.pending.Load(); pending > 0 {
		fmt.Fprintf(errOut, "Waiting for the %s not yet forwarded (Ctrl+C to skip)\n", countOf(int(pending), "event"))
	}
	select {
	case <-f.done:
	case <-ctx.Done():
	}
}

// countOf names count things of noun, such as "1 event" or "2 events".
func countOf(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

// forward posts payload, the event's data as the server sent it.
func (f *eventForwarder) forward(payload json.RawMessage) {
	// Generate message ID and timestamp
	msgID := webhooks.GenerateMsgID()
	timestamp := time.Now()

	logger.Get().WithFields(map[string]interface{}{
		"msg_id":       msgID,
		"timestamp":    timestamp.Unix(),
		"url":          f.url,
		"payload_size": len(payload),
	}).Debugf("Preparing to forward %s event", f.kind)

	// Sign the payload
	signature, err := f.signer.Sign(msgID, timestamp, payload)
	if err != nil {
		logger.Get().WithError(err).Errorf("Failed to sign %s event payload", f.kind)
		return
	}

	req, err := http.NewRequest("POST", f.url, bytes.NewReader(payload))
	if err != nil {
		logger.Get().WithError(err).Error("Failed to create forward request")
		return
	}

	headers := map[string]string{
		"Content-Type":      "application/json",
		"webhook-id":        msgID,
		"webhook-timestamp": fmt.Sprintf("%d", timestamp.Unix()),
		"webhook-signature": signature,
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	logger.Get().WithFields(map[string]interface{}{
		"method":  "POST",
		"url":     f.url,
		"headers": headers,
	}).Debugf("Sending %s event forward request", f.kind)

	startTime := time.Now()
	resp, err := f.httpClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		logger.Get().WithError(err).WithFields(map[string]interface{}{
			"url":      f.url,
			"duration": duration.String(),
		}).Errorf("Failed to forward %s event", f.kind)
		return
	}
	defer resp.Body.Close()

	logger.Get().WithFields(map[string]interface{}{
		"url":      f.url,
		"status":   resp.StatusCode,
		"duration": duration.String(),
	}).Debugf("Received %s event forward response", f.kind)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Get().WithFields(map[string]interface{}{
			"url":    f.url,
			"status": resp.StatusCode,
		}).Debugf("Successfully forwarded %s event", f.kind)
	} else {
		logger.Get().WithFields(map[string]interface{}{
			"url":    f.url,
			"status": resp.StatusCode,
		}).Warnf("Forwarding the %s event returned a non-2xx status", f.kind)
	}
}
