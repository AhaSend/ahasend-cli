package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// TestDeprecationNoticeGoesToStderr runs a deprecated flag alias in JSON mode
// and checks that stdout holds only the API response and the notice is on
// stderr.
func TestDeprecationNoticeGoesToStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the CLI binary")
	}

	const body = `{"object":"webhook","id":"11111111-1111-4111-8111-111111111111","name":"Events","url":"https://example.com/hook","enabled":false}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	stdout, stderr, code := runCLI(t, buildCLIBinary(t), writeProfile(t, server.URL),
		"webhooks", "update", "11111111-1111-4111-8111-111111111111", "--disable", "--output", "json")

	assert.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, testutil.IndentedJSON(t, body), stdout)
	assert.Equal(t, "Flag --disable has been deprecated, use --enabled=false instead\n", stderr)
}
