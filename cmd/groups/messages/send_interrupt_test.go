package messages

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/client"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// recipientsCSV writes a recipients file of count recipients and returns its
// path.
func recipientsCSV(t *testing.T, count int) string {
	t.Helper()
	var csv strings.Builder
	csv.WriteString("email\n")
	for i := range count {
		fmt.Fprintf(&csv, "user%d@example.com\n", i)
	}
	return testutil.WriteTempFile(t, "recipients.csv", csv.String())
}

// runSendWithContext runs messages send with plain output in ctx, as the
// root command would, and returns its stdout, stderr and error.
func runSendWithContext(t *testing.T, ctx context.Context, args ...string) (string, string, error) {
	t.Helper()
	return runSendInFormat(t, ctx, "plain", args...)
}

// runSendInFormat runs messages send with output format in ctx, as the root
// command would, and returns its stdout, stderr and error.
func runSendInFormat(t *testing.T, ctx context.Context, format string, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := NewSendCommand()
	cmd.SetContext(context.WithValue(ctx, printer.ResponseHandlerKey,
		testutil.ResponseHandler(t, format, &stdout, &stderr)))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	// The root command prints errors itself
	cmd.SilenceErrors = true
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

// TestSendInterruptedFinishesTheRequestInFlightAndExits130 checks that an
// interruption during the first of three requests lets that request finish,
// sends no other, prints the summary, saves the recipients not sent to be
// sent again and exits 130.
func TestSendInterruptedFinishesTheRequestInFlightAndExits130(t *testing.T) {
	home := testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	testutil.InjectClient(t, mockClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { cancel() }).
		Return(mocks.NewMessageResponse("msg"), nil).Once()

	stdout, stderr, err := runSendWithContext(t, ctx, "--from", "sender@example.com", "--subject", "Hi",
		"--text", "Hi", "--recipients", recipientsCSV(t, 250))

	require.Error(t, err)
	assert.Equal(t, 130, clierrors.GetExitCode(err))
	assert.Equal(t, "interrupted: 150 of 250 recipients were not sent", err.Error())
	assert.True(t, strings.HasPrefix(stdout, "Message sent to 100 of 250 recipients\nSucceeded: 100\nFailed: 0\n"), stdout)
	files, globErr := filepath.Glob(filepath.Join(home, ".ahasend", "failed-*.json"))
	require.NoError(t, globErr)
	require.Len(t, files, 1)
	assert.Contains(t, stderr, "Failed recipients saved to "+files[0])
	data, readErr := os.ReadFile(files[0])
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "user100@example.com")
	assert.Contains(t, string(data), "user249@example.com")
	assert.NotContains(t, string(data), "user99@example.com")
	mockClient.AssertExpectations(t)
}

// TestSendWithARequestStillInProgressReportsAnUnknownOutcome checks that
// recipients whose request the API was still processing are reported apart
// and kept out of the file of recipients to send again.
func TestSendWithARequestStillInProgressReportsAnUnknownOutcome(t *testing.T) {
	home := testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	testutil.InjectClient(t, mockClient)
	inProgress := &api.APIError{StatusCode: 409, Type: api.ErrorTypeIdempotencyConflict, RetryAfter: 1,
		Message: "request in progress"}
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, mock.Anything).Return(nil, inProgress).Once()
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, mock.Anything).Return(mocks.NewMessageResponse("msg"), nil).Once()

	stdout, stderr, err := runSendWithContext(t, context.Background(), "--from", "sender@example.com",
		"--subject", "Hi", "--text", "Hi", "--recipients", recipientsCSV(t, 150), "--max-retries", "0",
		"--idempotency-key", "order-7")

	require.Error(t, err)
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	assert.Equal(t, "0 of 150 recipients failed and the outcome of 100 is unknown", err.Error())
	assert.Contains(t, stdout, "\nSucceeded: 50\nFailed: 0\nUnknown: 100\n")
	assert.Contains(t, stdout, "Outcome: unknown")
	unknown, globErr := filepath.Glob(filepath.Join(home, ".ahasend", "unknown-*.json"))
	require.NoError(t, globErr)
	require.Len(t, unknown, 1)
	assert.Contains(t, stderr, "The outcome of the request of 100 recipients (idempotency key order-7-0) is unknown: "+
		"they may have been sent, and are saved to "+unknown[0]+". Check them with 'ahasend messages list', "+
		"or run the same command again with --idempotency-key order-7 within 24 hours: the API answers a request "+
		"it already took with its stored result instead of sending it again\n")
	failed, globErr := filepath.Glob(filepath.Join(home, ".ahasend", "failed-*.json"))
	require.NoError(t, globErr)
	assert.Empty(t, failed)
	mockClient.AssertExpectations(t)
}

// TestSendStopsOnSIGINTAndSIGTERM checks that the signals interrupt the send
// like a cancelled context does.
func TestSendStopsOnSIGINTAndSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a process cannot send itself these signals on Windows")
	}
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			testutil.UseNewHome(t)
			mockClient := &mocks.MockClient{}
			testutil.InjectClient(t, mockClient)
			mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, mock.Anything).
				Run(func(mock.Arguments) {
					process, err := os.FindProcess(os.Getpid())
					require.NoError(t, err)
					require.NoError(t, process.Signal(sig))
					time.Sleep(50 * time.Millisecond) // the signal arrives while the request is in flight
				}).
				Return(mocks.NewMessageResponse("msg"), nil).Once()

			_, _, err := runSendWithContext(t, context.Background(), "--from", "sender@example.com", "--subject", "Hi",
				"--text", "Hi", "--recipients", recipientsCSV(t, 150))

			require.Error(t, err)
			assert.Equal(t, 130, clierrors.GetExitCode(err))
			mockClient.AssertExpectations(t)
		})
	}
}

// TestAnInterruptedSingleRequestPrintsItsError checks that a single request
// the API refused with 429, interrupted while waiting for its Retry-After,
// prints the API's error, counts its recipient as failed and exits 130.
func TestAnInterruptedSingleRequestPrintsItsError(t *testing.T) {
	const body = `{"message":"rate limit exceeded"}`
	tests := map[string]struct {
		stdout, stderr string
	}{
		"json":  {testutil.IndentedJSON(t, body), ""},
		"table": {"", "Error: rate_limit error (HTTP 429): rate limit exceeded (the API asked to wait 30 s)\n"},
	}
	for format, test := range tests {
		t.Run(format, func(t *testing.T) {
			home := testutil.UseNewHome(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				cancel() // the interruption comes during the wait for the retry
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, body)
			}))
			t.Cleanup(server.Close)
			sendClient, err := client.NewClient("aha-sk-test", testAccountID, server.URL)
			require.NoError(t, err)
			testutil.InjectClient(t, sendClient)

			start := time.Now()
			stdout, stderr, err := runSendInFormat(t, ctx, format, "--from", "sender@example.com", "--to", "user@example.com",
				"--subject", "Hi", "--text", "Hi")

			require.Error(t, err)
			assert.Less(t, time.Since(start), 10*time.Second, "the interruption ends the wait")
			assert.Equal(t, 130, clierrors.GetExitCode(err))
			assert.Equal(t, "interrupted: 0 of 1 recipients were not sent, 1 failed", err.Error())
			assert.Equal(t, test.stdout, stdout)
			if test.stderr != "" {
				assert.Contains(t, stderr, test.stderr)
			}
			files, globErr := filepath.Glob(filepath.Join(home, ".ahasend", "failed-*.json"))
			require.NoError(t, globErr)
			assert.Len(t, files, 1)
		})
	}
}

// TestSendFailsA503TheAPIAsksToRetryTooLateAtOnce checks that a 503 whose
// Retry-After is over the 300 seconds a send waits fails at once, with the
// wait the API asked for, as a 429 does.
func TestSendFailsA503TheAPIAsksToRetryTooLateAtOnce(t *testing.T) {
	testutil.UseNewHome(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "301")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"message":"maintenance"}`)
	}))
	t.Cleanup(server.Close)
	sendClient, err := client.NewClient("aha-sk-test", testAccountID, server.URL)
	require.NoError(t, err)
	testutil.InjectClient(t, sendClient)

	start := time.Now()
	_, _, err = runSendWithContext(t, context.Background(), "--from", "sender@example.com", "--to", "user@example.com",
		"--subject", "Hi", "--text", "Hi")

	require.Error(t, err)
	assert.Less(t, time.Since(start), 10*time.Second)
	assert.Equal(t, int32(1), requests.Load())
	assert.Contains(t, err.Error(), "the API asked to retry after 301 seconds, more than the 300 a send waits: ")
	assert.Contains(t, err.Error(), "(HTTP 503): maintenance")
	assert.Equal(t, 1, clierrors.GetExitCode(err))
}

// TestASendInterruptedBeforeItsRequestPrintsNothing checks that a single
// request send interrupted before the request was made prints nothing on
// stdout, says so on stderr in JSON mode, where the error is not printed,
// and exits 130.
func TestASendInterruptedBeforeItsRequestPrintsNothing(t *testing.T) {
	for _, format := range []string{"json", "table", "plain", "csv"} {
		t.Run(format, func(t *testing.T) {
			testutil.UseNewHome(t)
			mockClient := &mocks.MockClient{}
			testutil.InjectClient(t, mockClient)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			stdout, stderr, err := runSendInFormat(t, ctx, format, "--from", "sender@example.com", "--to", "user@example.com",
				"--subject", "Hi", "--text", "Hi")

			require.Error(t, err)
			assert.Equal(t, 130, clierrors.GetExitCode(err))
			assert.Equal(t, "interrupted: 1 of 1 recipients were not sent", err.Error())
			assert.Empty(t, stdout)
			if format == "json" {
				assert.Contains(t, stderr, "interrupted: 1 of 1 recipients were not sent\n")
			}
			mockClient.AssertNotCalled(t, "SendMessageWithIdempotencyKey", mock.Anything, mock.Anything)
		})
	}
}

// TestASignalAfterTheLastRequestDoesNotInterruptTheSend checks that a send
// whose every request finished exits as if no signal came.
func TestASignalAfterTheLastRequestDoesNotInterruptTheSend(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	testutil.InjectClient(t, mockClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { cancel() }).
		Return(mocks.NewMessageResponse("msg"), nil).Once()

	stdout, _, err := runSendWithContext(t, ctx, "--from", "sender@example.com", "--to", "user@example.com",
		"--subject", "Hi", "--text", "Hi")

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(stdout, "Message sent successfully\n"), stdout)
	mockClient.AssertExpectations(t)
}

// TestTheUnknownOutcomeNoticeNamesAGeneratedKey checks that the notice of a
// request of unknown outcome names the idempotency key the command
// generated, to send it again with.
func TestTheUnknownOutcomeNoticeNamesAGeneratedKey(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	testutil.InjectClient(t, mockClient)
	var key string
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { key = args.String(1) }).
		Return(nil, &api.APIError{StatusCode: 504, Message: "gateway timeout"}).Once()

	_, stderr, err := runSendWithContext(t, context.Background(), "--from", "sender@example.com", "--to", "user@example.com",
		"--subject", "Hi", "--text", "Hi", "--max-retries", "0")

	require.Error(t, err)
	require.NotEmpty(t, key)
	assert.Contains(t, stderr, "(idempotency key "+key+") is unknown")
	assert.Contains(t, stderr, "run the same command again with --idempotency-key "+key+" within 24 hours")
	mockClient.AssertExpectations(t)
}

// TestTheUnknownOutcomeNoticeQuotesAKeyWithShellCharacters checks that the
// notice of a request of unknown outcome names a key with a space or a
// character a shell reads, such as a quote, in the form a user can paste.
func TestTheUnknownOutcomeNoticeQuotesAKeyWithShellCharacters(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	testutil.InjectClient(t, mockClient)
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, "Bob's order 7").
		Return(nil, &api.APIError{StatusCode: 504, Message: "gateway timeout"}).Once()

	_, stderr, err := runSendWithContext(t, context.Background(), "--from", "sender@example.com", "--to", "user@example.com",
		"--subject", "Hi", "--text", "Hi", "--max-retries", "0", "--idempotency-key", "Bob's order 7")

	require.Error(t, err)
	assert.Contains(t, stderr, `(idempotency key 'Bob'\''s order 7') is unknown`)
	assert.Contains(t, stderr, `run the same command again with --idempotency-key 'Bob'\''s order 7' within 24 hours`)
	mockClient.AssertExpectations(t)
}

// TestTheUnknownOutcomeNoticeGivesTheTimeOfARelativeSchedule checks that the
// notice of a request of unknown outcome sent with a relative --schedule
// names the time it stood for, and that the same command with that time and
// the key sends the same request body, which the API needs to take the key
// again.
func TestTheUnknownOutcomeNoticeGivesTheTimeOfARelativeSchedule(t *testing.T) {
	testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	testutil.InjectClient(t, mockClient)
	var bodies [][]byte
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			body, err := json.Marshal(args.Get(0))
			require.NoError(t, err)
			bodies = append(bodies, body)
		}).
		Return(nil, &api.APIError{StatusCode: 504, Message: "gateway timeout"}).Twice()
	send := []string{"--from", "sender@example.com", "--to", "user@example.com",
		"--subject", "Hi", "--text", "Hi", "--max-retries", "0", "--idempotency-key", "order-7"}

	_, stderr, err := runSendWithContext(t, context.Background(), append(send, "--schedule", "2h")...)
	require.Error(t, err)
	_, schedule, found := strings.Cut(stderr, "--idempotency-key order-7 and --schedule ")
	require.True(t, found, stderr)
	schedule, _, _ = strings.Cut(schedule, ",")
	assert.Contains(t, stderr, ", the time the relative --schedule stood for, within 24 hours")

	_, stderr, err = runSendWithContext(t, context.Background(), append(send, "--schedule", schedule)...)
	require.Error(t, err)
	require.Len(t, bodies, 2)
	assert.Equal(t, string(bodies[0]), string(bodies[1]))
	assert.Contains(t, stderr, "run the same command again with --idempotency-key order-7 within 24 hours")
	mockClient.AssertExpectations(t)
}
