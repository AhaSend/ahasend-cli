package testenv_test

import (
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/testenv"
)

func TestMain(m *testing.M) {
	testenv.Main(m)
}

// moduleRoot returns the directory holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func TestEveryTestPackageRunsInTheIsolatedEnvironment(t *testing.T) {
	isolated := map[string]bool{}
	err := filepath.WalkDir(moduleRoot(t), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != moduleRoot(t) && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		isolated[dir] = isolated[dir] || strings.Contains(string(content), "testenv.Main(m)") ||
			strings.Contains(string(content), "testenv.Run(m)")
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, isolated)

	for dir, ok := range isolated {
		assert.True(t, ok, "%s has tests but no TestMain calling testenv.Main(m) or testenv.Run(m)", dir)
	}
}

func TestHomeIsANewEmptyDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(filepath.Base(home), "ahasend-test-home-"), home)
	assert.NoFileExists(t, filepath.Join(home, ".ahasend", "config.yaml"))
}

func TestAhaSendEnvironmentVariablesAreUnset(t *testing.T) {
	for _, variable := range os.Environ() {
		assert.False(t, strings.HasPrefix(variable, "AHASEND_"), variable)
	}
}

// The CLI's client sends every request through http.DefaultTransport, so a
// request to the SDK's default server through it is what any test reaching
// the API would send.
func TestRequestsToTheDefaultAPIHostFailAtTheBlockedProxy(t *testing.T) {
	defaultURL := api.NewConfiguration().Servers[0].URL
	require.Contains(t, defaultURL, "ahasend.com")

	request, err := http.NewRequest(http.MethodGet, defaultURL+"/v2/ping", nil)
	require.NoError(t, err)
	response, err := http.DefaultTransport.RoundTrip(request)
	if response != nil {
		response.Body.Close()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proxyconnect")
	assert.Contains(t, err.Error(), strings.TrimPrefix(testenv.BlockedProxy, "http://"))
}

func TestRequestsToLoopbackServersAreNotProxied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	response, err := http.Get(server.URL)
	require.NoError(t, err)
	response.Body.Close()
	assert.Equal(t, http.StatusNoContent, response.StatusCode)
}

// smtp send dials its server directly, which no proxy covers. The default
// server is a host name, so blocked DNS keeps a test that leaves --server
// unset away from it. The name used here is reserved and never resolves, so
// the test sends nothing even where DNS is not blocked.
func TestConnectionsToNamedHostsFailAtTheBlockedResolver(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "send.ahasend.invalid:587", 5*time.Second)
	if conn != nil {
		conn.Close()
	}
	require.Error(t, err)
	assert.ErrorContains(t, err, testenv.ErrDNSBlocked.Error())
}
