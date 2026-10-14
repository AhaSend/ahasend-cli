package printer

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unreachableHint is what an error line says to check when the API could not
// be reached.
const unreachableHint = " (check the network connection and the API URL: auth login --api-url or the profile's api_url)"

func TestHandleErrorSaysHowLongTheAPIAskedToWait(t *testing.T) {
	err := fmt.Errorf("failed to list domains: %w",
		&api.APIError{Type: api.ErrorTypeRateLimit, StatusCode: 429, Message: "slow down", RetryAfter: 12})
	for _, format := range []string{"table", "plain", "csv"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			require.Error(t, testHandler(t, format, &stdout, &stderr).HandleError(err))

			assert.Equal(t, "Error: failed to list domains: rate_limit error (HTTP 429): slow down (the API asked to wait 12 s)\n",
				stderr.String())
		})
	}
}

// TestHandleErrorSaysHowLongTheAPIAskedToWaitAfterA503 checks that the wait
// a Retry-After header asks for is given for any API error that has one, such
// as a 503, which the SDK fails at once when the wait is over 30 seconds.
func TestHandleErrorSaysHowLongTheAPIAskedToWaitAfterA503(t *testing.T) {
	var stderr bytes.Buffer

	_ = testHandler(t, "plain", &bytes.Buffer{}, &stderr).HandleError(
		&api.APIError{Type: api.ErrorTypeServer, StatusCode: 503, Message: "maintenance", RetryAfter: 120})

	assert.Equal(t, "Error: server error (HTTP 503): maintenance (the API asked to wait 120 s)\n", stderr.String())
}

// namedWaitError is an error whose text names the wait, as a send's
// RetryAfterTooLongError does.
type namedWaitError struct{ err error }

func (e namedWaitError) Error() string {
	return "the API asked to retry after 3600 seconds: " + e.err.Error()
}
func (e namedWaitError) Unwrap() error { return e.err }
func (e namedWaitError) NamesTheWait() {}

func TestHandleErrorDoesNotNameTheWaitTwice(t *testing.T) {
	var stderr bytes.Buffer

	_ = testHandler(t, "plain", &bytes.Buffer{}, &stderr).HandleError(namedWaitError{
		&api.APIError{Type: api.ErrorTypeRateLimit, StatusCode: 429, Message: "slow down", RetryAfter: 3600}})

	assert.Equal(t, "Error: the API asked to retry after 3600 seconds: rate_limit error (HTTP 429): slow down\n", stderr.String())
}

func TestHandleErrorAddsNoWaitWithoutRetryAfter(t *testing.T) {
	var stderr bytes.Buffer

	_ = testHandler(t, "plain", &bytes.Buffer{}, &stderr).HandleError(
		&api.APIError{Type: api.ErrorTypeRateLimit, StatusCode: 429, Message: "slow down"})

	assert.Equal(t, "Error: rate_limit error (HTTP 429): slow down\n", stderr.String())
}

func TestJSONHandleErrorKeepsTheRateLimitBodyAlone(t *testing.T) {
	var stdout, stderr bytes.Buffer
	body := `{"message":"slow down"}`

	_ = testHandler(t, "json", &stdout, &stderr).HandleError(
		&api.APIError{StatusCode: 429, Message: "slow down", RetryAfter: 12, Raw: []byte(body)})

	assert.JSONEq(t, body, stdout.String())
	assert.Empty(t, stderr.String())
}

func TestHandleErrorSaysWhatToCheckWhenTheAPICannotBeReached(t *testing.T) {
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("connection refused")}
	tests := map[string]error{
		"failed request": &url.Error{Op: "Get", URL: "https://api.example.com/v2/ping", Err: refused},
		"SDK":            &api.NetworkError{Op: "reading response", Err: fmt.Errorf("unexpected EOF")},
	}
	for name, err := range tests {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer

			_ = testHandler(t, "table", &bytes.Buffer{}, &stderr).HandleError(err)

			assert.Equal(t, "Error: "+err.Error()+unreachableHint+"\n", stderr.String())
		})
	}
}

// TestHandleErrorAddsNoAPIHintForAnotherServer checks that a failure to reach
// a server other than the API, such as the SMTP server of smtp send, does not
// point at the API URL.
func TestHandleErrorAddsNoAPIHintForAnotherServer(t *testing.T) {
	var stderr bytes.Buffer
	err := &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("connection refused")}

	_ = testHandler(t, "table", &bytes.Buffer{}, &stderr).HandleError(err)

	assert.Equal(t, "Error: "+err.Error()+"\n", stderr.String())
}
