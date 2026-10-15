//go:build unix

package integration

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendToAClosedPipeStillTellsWhereTheRecipientsWent runs messages send
// with a stdout pipe whose reader is gone, as in
// `ahasend messages send ... | head`. The command exits 1 instead of dying of
// SIGPIPE, and stderr still names the saved files and the idempotency key of
// a request whose outcome is unknown.
func TestSendToAClosedPipeStillTellsWhereTheRecipientsWent(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the CLI binary")
	}

	binary := buildCLIBinary(t)
	var rows strings.Builder
	rows.WriteString("email\n")
	for i := range 101 {
		fmt.Fprintf(&rows, "user%d@example.com\n", i)
	}
	recipients := filepath.Join(t.TempDir(), "recipients.csv")
	require.NoError(t, os.WriteFile(recipients, []byte(rows.String()), 0o600))

	readerClosed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Answer only once the pipe has no reader, so every write to
		// stdout fails. The first request of 100 recipients is accepted
		// with one rejected, which stdout reports; the second gets an
		// empty answer, so its outcome is unknown.
		<-readerClosed
		w.WriteHeader(http.StatusAccepted)
		if r.Header.Get("Idempotency-Key") == "closed-pipe-key-0" {
			_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"message","id":null,` +
				`"recipient":{"email":"user0@example.com"},"status":"error","error":"Recipient is suppressed"}]}`))
		}
	}))
	t.Cleanup(server.Close)
	home := writeProfile(t, server.URL)

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	command := exec.Command(binary, "messages", "send", "--from", "a@example.com", "--recipients", recipients,
		"--subject", "Hi", "--text", "Hi", "--idempotency-key", "closed-pipe-key")
	command.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "NO_COLOR=1")
	command.Stdout = writer
	command.Stderr = &stderr
	require.NoError(t, command.Start())
	require.NoError(t, writer.Close())
	require.NoError(t, reader.Close())
	close(readerClosed)

	err = command.Wait()

	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "the command must fail: %v", err)
	assert.Equal(t, 1, exitErr.ExitCode(), "a signal ends the process with -1: %v\nstderr: %s", exitErr, stderr.String())
	assert.Contains(t, stderr.String(), "/.ahasend/failed-")
	assert.Contains(t, stderr.String(), "/.ahasend/unknown-")
	assert.Contains(t, stderr.String(), "closed-pipe-key-1")
	assert.Contains(t, stderr.String(), "broken pipe")
}
