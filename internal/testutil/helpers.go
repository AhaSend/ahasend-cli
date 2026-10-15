// Package testutil provides testing utilities and helpers for the AhaSend CLI:
//
//   - Command execution helpers that capture output and inject a mock client
//   - A home directory per test, so tests never read the real configuration
//
// Fixtures of API responses live in the mocks package.
package testutil

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/testenv"
)

// UseNewHome points the home directory at a new empty directory.
func UseNewHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	return home
}

// WriteConfig writes config as the CLI's config file under home and returns
// the file's path.
func WriteConfig(t *testing.T, home, config string) string {
	t.Helper()
	configDir := filepath.Join(home, ".ahasend")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	configPath := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))
	return configPath
}

// WriteProfiles writes a config file under home with a profile for each of
// names, the first one the default, and returns the file's path. Each profile
// points at apiURL, such as a local test server, with accountID and the API
// key "aha-sk-" followed by its name.
func WriteProfiles(t *testing.T, home, apiURL, accountID string, names ...string) string {
	t.Helper()
	require.NotEmpty(t, names)
	var config strings.Builder
	fmt.Fprintf(&config, "default_profile: %s\nprofiles:\n", names[0])
	for _, name := range names {
		fmt.Fprintf(&config, "  %[1]s:\n    api_key: aha-sk-%[1]s\n    api_url: %[2]s\n    account_id: %[3]s\n",
			name, apiURL, accountID)
	}
	return WriteConfig(t, home, config.String())
}

// HangUpURL returns the URL of a local listener that closes every connection
// before answering, so each request fails to reach the API. The listener
// keeps its port for the whole test: a closed port could be taken by another
// test's server and answer the request.
func HangUpURL(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	return "http://" + listener.Addr().String()
}

// OpenError returns the error text of opening the missing file path, as this
// operating system words it.
func OpenError(t *testing.T, path string) string {
	t.Helper()
	_, err := os.Open(path) //nolint:gosec // the test names a missing file
	require.Error(t, err)
	return err.Error()
}

// StatError returns the error text of reading the details of the missing file
// path, as this operating system words it.
func StatError(t *testing.T, path string) string {
	t.Helper()
	_, err := os.Stat(path)
	require.Error(t, err)
	return err.Error()
}
