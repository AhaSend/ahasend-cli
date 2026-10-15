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
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
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

// The CLI binary the tests of the package run, which the first call of
// buildCLIBinary builds into cliBinaryDir, and TestMain removes.
var (
	cliBinaryOnce sync.Once
	cliBinaryDir  string
	cliBinary     string
	cliBinaryErr  error
)

// buildCLIBinary returns the CLI binary, built once for the package, so exit
// codes can be checked on the real process rather than on in-process command
// execution. It is built with the clitest build tag, which makes it retry a
// failed request after a millisecond rather than a second.
func buildCLIBinary(t *testing.T) string {
	t.Helper()
	cliBinaryOnce.Do(func() {
		cliBinaryDir, cliBinaryErr = os.MkdirTemp("", "ahasend-cli-")
		if cliBinaryErr != nil {
			return
		}
		_, thisFile, _, _ := runtime.Caller(0)
		binary := filepath.Join(cliBinaryDir, executableName("ahasend", runtime.GOOS))
		build := exec.Command("go", "build", "-tags", "clitest", "-o", binary, ".")
		build.Dir = filepath.Join(filepath.Dir(thisFile), "..", "..")
		if out, err := build.CombinedOutput(); err != nil {
			cliBinaryErr = fmt.Errorf("go build failed: %w: %s", err, out)
			return
		}
		cliBinary = binary
	})
	require.NoError(t, cliBinaryErr)
	return cliBinary
}

// executableName is the file name of the executable called name on goos:
// Windows runs only files with an executable extension.
func executableName(name, goos string) string {
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}

func TestExecutableNameHasTheExeExtensionOnWindows(t *testing.T) {
	assert.Equal(t, "ahasend.exe", executableName("ahasend", "windows"))
	assert.Equal(t, "ahasend", executableName("ahasend", "linux"))
}

// removeCLIBinary removes the CLI binary, if a test built it.
func removeCLIBinary() {
	if cliBinaryDir != "" {
		_ = os.RemoveAll(cliBinaryDir)
	}
}

// writeProfile creates an isolated HOME whose default profile points the CLI at
// apiURL, and returns that HOME directory.
func writeProfile(t *testing.T, apiURL string) string {
	t.Helper()
	home := t.TempDir()
	testutil.WriteProfiles(t, home, apiURL, "00000000-0000-0000-0000-0000000000aa", "default")
	return home
}

// runCLI runs the binary with the given HOME and returns stdout, stderr, and
// the process exit code.
func runCLI(t *testing.T, binary, home string, args ...string) (string, string, int) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "NO_COLOR=1")
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
		// network scenarios use a listener that hangs up instead of a test
		// server.
		network  bool
		exitCode int
	}{
		{name: "success", status: http.StatusOK, body: exitCodeSubAccountBody},
		{name: "validation", status: http.StatusUnprocessableEntity, body: `{"message":"website must be a valid domain"}`, exitCode: 4},
		{name: "authentication", status: http.StatusUnauthorized, body: `{"message":"invalid API key"}`, exitCode: 2},
		{name: "missing resource", status: http.StatusNotFound, body: `{"message":"sub-account not found"}`, exitCode: 5},
		{name: "conflict", status: http.StatusConflict, body: `{"message":"idempotency key is already in use"}`, exitCode: 1},
		{name: "rate limit", status: http.StatusTooManyRequests, body: `{"message":"rate limit exceeded"}`, exitCode: 8},
		{name: "server", status: http.StatusInternalServerError, body: `{"message":"internal server error"}`, exitCode: 1},
		{name: "network", network: true, exitCode: 7},
	}

	for _, scenario := range scenarios {
		for _, format := range formats {
			t.Run(scenario.name+"/"+format, func(t *testing.T) {
				t.Parallel()

				var apiURL string
				if scenario.network {
					apiURL = testutil.HangUpURL(t)
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

				assert.Equal(t, scenario.exitCode, code, "stdout: %s\nstderr: %s", stdout, stderr)
				assert.NotContains(t, stdout, "Usage:")
				assert.NotContains(t, stderr, "Usage:")
				assert.NotContains(t, stderr, "level=", "no log lines without --verbose or --debug")
				switch {
				case format == "json" && scenario.network:
					assert.Contains(t, stdout, `"error": true`, "JSON mode must print the CLI error object")
					assert.Empty(t, stderr)
				case format == "json":
					assert.JSONEq(t, scenario.body, stdout, "JSON mode must print the raw API body unchanged")
					assert.Empty(t, stderr)
				default:
					assert.Empty(t, stdout, "errors go to stderr outside JSON mode")
					assert.True(t, strings.HasPrefix(stderr, "Error: "), "stderr: %s", stderr)
				}
			})
		}
	}
}
