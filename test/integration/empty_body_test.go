package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCLIRefusesAnEmptySuccessBody runs the real binary against a server that
// answers every request with a 2xx status and no body. A command that shows
// data from the answer fails with exit 1 instead of showing empty values, and
// messages send reports the outcome as unknown.
func TestCLIRefusesAnEmptySuccessBody(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the CLI binary")
	}

	binary := buildCLIBinary(t)

	tests := []struct {
		name       string
		status     int
		args       []string
		notStdout  string
		wantStderr []string
	}{
		{
			name:      "domains get",
			status:    http.StatusOK,
			args:      []string{"domains", "get", "example.com"},
			notStdout: "DNS Status",
		},
		{
			name:      "auth status",
			status:    http.StatusOK,
			args:      []string{"auth", "status"},
			notStdout: "Valid: Yes",
		},
		{
			name:      "apikeys create",
			status:    http.StatusCreated,
			args:      []string{"apikeys", "create", "--label", "ci", "--scope", "messages:send:all"},
			notStdout: "created",
		},
		{
			name:   "messages send",
			status: http.StatusAccepted,
			args: []string{"messages", "send", "--from", "a@example.com", "--to", "b@example.com",
				"--subject", "Hi", "--text", "Hi", "--idempotency-key", "empty-body-key"},
			notStdout:  "sent",
			wantStderr: []string{"is unknown", "unknown-", "empty-body-key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			t.Cleanup(server.Close)

			stdout, stderr, code := runCLI(t, binary, writeProfile(t, server.URL), tt.args...)

			assert.Equal(t, 1, code, "stdout: %s\nstderr: %s", stdout, stderr)
			assert.NotContains(t, stdout, tt.notStdout)
			assert.Contains(t, stderr, "could not read the API response: the response is empty")
			for _, want := range tt.wantStderr {
				assert.Contains(t, stderr, want)
			}
		})
	}
}
