package logger

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// debugLogOfRoundTrip sends a request with requestBody through the logging
// transport, in debug mode, to a server that answers responseBody, and
// returns the log.
func debugLogOfRoundTrip(t *testing.T, requestBody, responseBody string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, responseBody)
	}))
	t.Cleanup(server.Close)

	var log bytes.Buffer
	debugLogger := newLogger(true, false, false)
	debugLogger.SetOutput(&log)
	client := &http.Client{Transport: NewHTTPTransport(http.DefaultTransport, debugLogger)}

	resp, err := client.Post(server.URL, "application/json", strings.NewReader(requestBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, responseBody, string(body), "the caller reads the body unchanged")
	return log.String()
}

func TestDebugLogRedactsSecretsInRequestAndResponseBodies(t *testing.T) {
	for _, key := range []string{"password", "secret_key", "secret", "api_key", "token", "Webhook_Secret", "access_token", "dkim_private_key", "private_key"} {
		t.Run(key, func(t *testing.T) {
			request := fmt.Sprintf(`{"name":"visible-name","%s":"request-value"}`, key)
			response := fmt.Sprintf(`{"items":[{"nested":{"%s":"response-value"}}]}`, key)

			log := debugLogOfRoundTrip(t, request, response)

			assert.NotContains(t, log, "request-value")
			assert.NotContains(t, log, "response-value")
			assert.Equal(t, 2, strings.Count(log, "[REDACTED]"), log)
			assert.Contains(t, log, "visible-name")
		})
	}
}

// trackingBody is a request body that records whether it was closed.
type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

func TestDebugRoundTripLeavesTheCallersRequestUnchangedAndClosesItsBody(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
	}))
	t.Cleanup(server.Close)

	debugLogger := newLogger(true, false, false)
	debugLogger.SetOutput(io.Discard)
	body := &trackingBody{Reader: strings.NewReader(`{"name":"value"}`)}
	req, err := http.NewRequest(http.MethodPost, server.URL, body)
	require.NoError(t, err)

	resp, err := NewHTTPTransport(http.DefaultTransport, debugLogger).RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, `{"name":"value"}`, received, "the server gets the whole body")
	assert.Same(t, body, req.Body, "the caller's request keeps its body")
	assert.True(t, body.closed, "the transport closes the body it read")
}

// TestDebugLogRedactsCredentialHeaders checks that the debug log of a request
// and its response shows no credential header value: Authorization,
// X-Api-Key, Cookie and Set-Cookie.
func TestDebugLogRedactsCredentialHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Set-Cookie", "session=response-cookie")
		w.Header().Set("X-Request-Id", "visible-request-id")
	}))
	t.Cleanup(server.Close)
	var log bytes.Buffer
	debugLogger := newLogger(true, false, false)
	debugLogger.SetOutput(&log)
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer authorization-value")
	req.Header.Set("X-Api-Key", "api-key-value")
	req.Header.Set("Cookie", "session=request-cookie")

	resp, err := NewHTTPTransport(http.DefaultTransport, debugLogger).RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	for _, secret := range []string{"authorization-value", "api-key-value", "request-cookie", "response-cookie"} {
		assert.NotContains(t, log.String(), secret)
	}
	assert.Equal(t, 4, strings.Count(log.String(), "[REDACTED]"), log.String())
	assert.Contains(t, log.String(), "visible-request-id")
}

// TestDebugLogCutsALargeBody checks that debug mode logs only the first
// maxLoggedBodyBytes of a body, with a mark that says how much it left out,
// while the caller still reads the whole body.
func TestDebugLogCutsALargeBody(t *testing.T) {
	large := `{"content":"` + strings.Repeat("x", 3*maxLoggedBodyBytes) + `"}`

	log := debugLogOfRoundTrip(t, large, large)

	assert.Equal(t, 2, strings.Count(log, "more bytes not logged]"), "the request and the response are cut")
	assert.NotContains(t, log, strings.Repeat("x", maxLoggedBodyBytes+1))
	assert.Less(t, len(log), 3*maxLoggedBodyBytes)
}

func TestLoggedBodyKeepsASmallBodyWhole(t *testing.T) {
	assert.Equal(t, "{\n  \"name\": \"x\"\n}", loggedBody(map[string]interface{}{"name": "x"}))
}

func TestLoggedBodyCutsAtARuneBoundary(t *testing.T) {
	text := strings.Repeat("é", maxLoggedBodyBytes)

	logged := loggedBody(text)

	kept, mark, found := strings.Cut(logged, "\n[cut: ")
	require.True(t, found, logged[len(logged)-40:])
	assert.True(t, utf8.ValidString(kept))
	assert.Equal(t, fmt.Sprintf("%d more bytes not logged]", len(`"`+text+`"`)-len(kept)), mark)
}
