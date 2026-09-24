package integration

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const exitCodeSubAccountBody = `{
  "object": "sub_account",
  "id": "11111111-1111-1111-1111-111111111111",
  "parent_account_id": "00000000-0000-0000-0000-0000000000aa",
  "created_at": "2026-01-01T00:00:00Z",
  "name": "Acme",
  "website": "acme.example.com",
  "status": "active",
  "monthly_credit": 0,
  "domain_count": 0,
  "member_count": 1
}`

// buildCLIBinary compiles the CLI once per test run so exit codes can be
// checked on the real process rather than on in-process command execution.
func buildCLIBinary(t *testing.T) string {
	t.Helper()

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	binary := filepath.Join(t.TempDir(), "ahasend")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = repoRoot
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	return binary
}

// writeProfile creates an isolated HOME whose default profile points the CLI at
// apiURL, and returns that HOME directory.
func writeProfile(t *testing.T, apiURL string) string {
	t.Helper()

	home := t.TempDir()
	configDir := filepath.Join(home, ".ahasend")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	config := fmt.Sprintf(`default_profile: default
profiles:
  default:
    name: default
    api_key: aha-sk-test
    api_url: %s
    account_id: 00000000-0000-0000-0000-0000000000aa
preferences:
  output_format: table
  color_output: false
`, apiURL)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(config), 0o600))

	return home
}

// runCLI runs the binary with the given HOME and returns stdout, stderr, and
// the process exit code.
func runCLI(t *testing.T, binary, home string, args ...string) (string, string, int) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	require.NoError(t, err)

	return stdout.String(), stderr.String(), 0
}

// closedURL returns a URL for a local port with nothing listening on it.
func closedURL(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	return "http://" + addr
}

func TestCLIExitCodes(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the CLI binary")
	}

	binary := buildCLIBinary(t)
	formats := []string{"json", "table", "plain", "csv"}

	scenarios := []struct {
		name   string
		status int
		body   string
		// network scenarios use a closed port instead of a test server.
		network bool
	}{
		{name: "success", status: http.StatusOK, body: exitCodeSubAccountBody},
		{name: "validation", status: http.StatusUnprocessableEntity, body: `{"message":"website must be a valid domain"}`},
		{name: "authentication", status: http.StatusUnauthorized, body: `{"message":"invalid API key"}`},
		{name: "missing resource", status: http.StatusNotFound, body: `{"message":"sub-account not found"}`},
		{name: "conflict", status: http.StatusConflict, body: `{"message":"idempotency key is already in use"}`},
		{name: "rate limit", status: http.StatusTooManyRequests, body: `{"message":"rate limit exceeded"}`},
		{name: "server", status: http.StatusInternalServerError, body: `{"message":"internal server error"}`},
		{name: "network", network: true},
	}

	for _, scenario := range scenarios {
		for _, format := range formats {
			t.Run(scenario.name+"/"+format, func(t *testing.T) {
				t.Parallel()

				var apiURL string
				if scenario.network {
					apiURL = closedURL(t)
				} else {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(scenario.status)
						_, _ = w.Write([]byte(scenario.body))
					}))
					t.Cleanup(server.Close)
					apiURL = server.URL
				}

				home := writeProfile(t, apiURL)
				stdout, stderr, code := runCLI(t, binary, home,
					"subaccounts", "get", "11111111-1111-1111-1111-111111111111", "--output", format)

				if scenario.status == http.StatusOK {
					assert.Equal(t, 0, code, "stdout: %s\nstderr: %s", stdout, stderr)
					assert.Contains(t, stdout, "Acme")
					return
				}

				assert.NotEqual(t, 0, code, "failed request must exit nonzero\nstdout: %s\nstderr: %s", stdout, stderr)
				assert.NotContains(t, stdout, "Usage:")
				assert.NotContains(t, stderr, "Usage:")
				if format == "json" && !scenario.network {
					assert.JSONEq(t, scenario.body, stdout, "JSON mode must print the raw API body unchanged")
				} else {
					assert.NotEmpty(t, strings.TrimSpace(stdout+stderr), "an error message must be printed")
				}
			})
		}
	}
}
