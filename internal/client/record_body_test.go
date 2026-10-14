package client

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
)

// recordedBodyFor serves body for every request and returns the client.
func recordedBodyFor(t *testing.T, status int, body string) *Client {
	t.Helper()
	client, cleanup := newClientTestServer(t, uuid.NewString(), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	t.Cleanup(cleanup)
	return client
}

// TestClient_RecordsAPIResponseBody checks that each response shape keeps the
// exact body the server sent, including keys and number literals the SDK
// struct cannot hold.
func TestClient_RecordsAPIResponseBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		call func(*Client) (any, error)
	}{
		{
			name: "single object",
			body: `{"object":"domain","future_field":{"nested":[1.50,"x"]},"domain":"example.com","big":12345678901234567890}`,
			call: func(c *Client) (any, error) { return c.GetDomain("example.com") },
		},
		{
			name: "paginated list",
			body: `{"object":"list","data":[{"domain":"example.com","zeta":1.50}],"pagination":{"has_more":false},"extra":true}`,
			call: func(c *Client) (any, error) { return c.ListDomains(requests.GetDomainsParams{}) },
		},
		{
			name: "success response",
			body: `{"message":"Domain deleted","future_field":1.50}`,
			call: func(c *Client) (any, error) { return c.DeleteDomain("example.com") },
		},
		{
			name: "ping",
			body: `{"message":"pong","future_field":1.50}`,
			call: func(c *Client) (any, error) { return c.Ping() },
		},
		{
			name: "webhook trigger",
			body: `{"message":"Webhook triggered","future_field":1.50}`,
			call: func(c *Client) (any, error) {
				return c.TriggerWebhook(uuid.New(), []string{"message.delivered"})
			},
		},
		{
			name: "route trigger",
			body: `{"message":"Route triggered","future_field":12345678901234567890}`,
			call: func(c *Client) (any, error) { return c.TriggerRoute(uuid.New()) },
		},
		{
			name: "logged call",
			body: `{"object":"list","data":[],"pagination":{"has_more":false},"future_field":12345678901234567890}`,
			call: func(c *Client) (any, error) { return c.GetMessages(requests.GetMessagesParams{}) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.call(recordedBodyFor(t, http.StatusOK, tt.body))
			require.NoError(t, err)

			body, ok := apiresponse.Body(result)

			require.True(t, ok, "no body recorded for %T", result)
			assert.Equal(t, tt.body, string(body))
		})
	}
}

func TestClient_FailedCallRecordsNothing(t *testing.T) {
	client := recordedBodyFor(t, http.StatusNotFound, `{"message":"Domain not found"}`)

	result, err := client.GetDomain("example.com")

	require.Error(t, err)
	_, ok := apiresponse.Body(result)
	assert.False(t, ok)
}

func TestClient_TriggerRouteErrorCarriesAPIBody(t *testing.T) {
	body := `{"message":"route is disabled","future_field":1.50}`
	client := recordedBodyFor(t, http.StatusUnprocessableEntity, body)

	result, err := client.TriggerRoute(uuid.New())

	assert.Nil(t, result)
	var apiErr *api.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
	assert.Equal(t, "route is disabled", apiErr.Message)
	assert.Equal(t, body, string(apiErr.Raw))
}

// TestClient_RawCallErrorsNameTheStatusForANonJSONBody checks that a call the
// SDK does not cover gives a non-JSON error body, such as a proxy's HTML page,
// the status text the SDK gives its own calls, and keeps the body. A 4xx
// status keeps the SDK call from retrying.
func TestClient_RawCallErrorsNameTheStatusForANonJSONBody(t *testing.T) {
	body := "<html><body>403 Forbidden</body></html>"
	tests := map[string]func(*Client) (any, error){
		"route trigger":    func(c *Client) (any, error) { return c.TriggerRoute(uuid.New()) },
		"webhook stream":   func(c *Client) (any, error) { return c.InitiateWebhookStream(nil) },
		"domain get (SDK)": func(c *Client) (any, error) { return c.GetDomain("example.com") },
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := call(recordedBodyFor(t, http.StatusForbidden, body))

			var apiErr *api.APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, "Forbidden", apiErr.Message)
			assert.Equal(t, body, string(apiErr.Raw))
		})
	}
}

// TestClient_UnreadableSuccessKeepsTheBody checks that a successful response
// whose body does not decode is an UnreadableResponseError holding the body,
// for SDK calls and for the calls the SDK does not cover.
func TestClient_UnreadableSuccessKeepsTheBody(t *testing.T) {
	body := `{"domain":5,"route_id":5}`
	tests := map[string]struct {
		call  func(*Client) (any, error)
		field string
	}{
		"domain get (SDK)": {func(c *Client) (any, error) { return c.GetDomain("example.com") }, "domain"},
		"route stream":     {func(c *Client) (any, error) { return c.InitiateRouteStream(nil, "*@example.com") }, "route_id"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := test.call(recordedBodyFor(t, http.StatusOK, body))

			var unreadable *clierrors.UnreadableResponseError
			require.ErrorAs(t, err, &unreadable)
			assert.Equal(t, body, string(unreadable.Body))
			assert.Equal(t, "could not read the API response: unexpected value for field "+test.field, err.Error())
			assert.Equal(t, 1, clierrors.GetExitCode(err))
		})
	}
}

// TestClient_UnreadableSuccessThatIsNotJSON checks the reason of a successful
// response whose body is not JSON at all.
func TestClient_UnreadableSuccessThatIsNotJSON(t *testing.T) {
	_, err := recordedBodyFor(t, http.StatusOK, "<html>OK</html>").GetDomain("example.com")

	require.Error(t, err)
	assert.Equal(t, "could not read the API response: the response is not JSON", err.Error())
}

// TestClient_RecordsAnEmptyBody checks the empty bodies the CLI accepts: any
// 2xx on a delete-style call.
func TestClient_RecordsAnEmptyBody(t *testing.T) {
	tests := []struct {
		name   string
		status int
		call   func(*Client) (any, error)
	}{
		{"delete, 204", http.StatusNoContent, func(c *Client) (any, error) { return c.DeleteDomain("example.com") }},
		{"delete, 200", http.StatusOK, func(c *Client) (any, error) { return c.DeleteDomain("example.com") }},
		{"cancel, 200", http.StatusOK, func(c *Client) (any, error) { return c.CancelMessage(uuid.NewString()) }},
		{"wipe, 200", http.StatusOK, func(c *Client) (any, error) { return c.WipeSuppressions(nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.call(recordedBodyFor(t, tt.status, ""))
			require.NoError(t, err)

			body, ok := apiresponse.Body(result)

			require.True(t, ok, "an empty body must be recorded so JSON output prints nothing")
			assert.Empty(t, body)
		})
	}
}

// TestClient_EmptySuccessBodyIsUnreadable checks that a call that reads data
// from the API's answer fails on an empty 2xx body instead of returning a
// result with no data, for SDK calls and for the calls the SDK does not
// cover.
func TestClient_EmptySuccessBodyIsUnreadable(t *testing.T) {
	tests := []struct {
		name   string
		status int
		call   func(*Client) (any, error)
	}{
		{"domain get", http.StatusOK, func(c *Client) (any, error) { return c.GetDomain("example.com") }},
		{"account get", http.StatusOK, func(c *Client) (any, error) { return c.GetAccount() }},
		{"ping", http.StatusOK, func(c *Client) (any, error) { return c.Ping() }},
		{"api key create", http.StatusCreated, func(c *Client) (any, error) {
			return c.CreateAPIKey(requests.CreateAPIKeyRequest{Label: "x", Scopes: []string{"messages:send:all"}})
		}},
		{"template draft discard", http.StatusOK, func(c *Client) (any, error) { return c.DiscardTemplateDraft(uuid.New()) }},
		{"message send", http.StatusAccepted, func(c *Client) (any, error) {
			return c.SendMessageWithIdempotencyKey(requests.CreateMessageRequest{}, "key")
		}},
		{"route trigger", http.StatusOK, func(c *Client) (any, error) { return c.TriggerRoute(uuid.New()) }},
		{"webhook stream", http.StatusOK, func(c *Client) (any, error) { return c.InitiateWebhookStream(nil) }},
		// No endpoint answers 204, so one with no body says nothing of the result
		{"domain get, 204", http.StatusNoContent, func(c *Client) (any, error) { return c.GetDomain("example.com") }},
		{"route trigger, 204", http.StatusNoContent, func(c *Client) (any, error) { return c.TriggerRoute(uuid.New()) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.call(recordedBodyFor(t, tt.status, ""))

			var unreadable *clierrors.UnreadableResponseError
			require.ErrorAs(t, err, &unreadable)
			assert.Equal(t, "could not read the API response: the response is empty", err.Error())
			assert.Equal(t, 1, clierrors.GetExitCode(err))
			assert.True(t, isNilPointer(result), "an empty answer must not return a result: %#v", result)
		})
	}
}

// isNilPointer reports whether value holds a nil pointer.
func isNilPointer(value any) bool {
	v := reflect.ValueOf(value)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// TestClient_AConnectionLostMidBodyIsANetworkError checks that a response
// whose connection closes before its whole body arrived exits 7 like any
// other lost connection, for SDK calls and for the calls the SDK does not
// cover.
func TestClient_AConnectionLostMidBodyIsANetworkError(t *testing.T) {
	tests := map[string]func(*Client) (any, error){
		"domain get (SDK)": func(c *Client) (any, error) { return c.GetDomain("example.com") },
		"route trigger":    func(c *Client) (any, error) { return c.TriggerRoute(uuid.New()) },
		"webhook stream":   func(c *Client) (any, error) { return c.InitiateWebhookStream(nil) },
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			client, cleanup := newClientTestServer(t, uuid.NewString(), func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"message":`))
				_ = http.NewResponseController(w).Flush()
				conn, _, err := http.NewResponseController(w).Hijack()
				if err == nil {
					_ = conn.Close()
				}
			})
			t.Cleanup(cleanup)

			_, err := call(client)

			require.Error(t, err)
			assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
			assert.Equal(t, 7, clierrors.GetExitCode(err))
		})
	}
}

// TestClient_AResponseLargerThanTheLimitFails checks that a response body
// larger than maxResponseBytes fails the call with exit 1, naming the limit,
// on a success and on an error status, for SDK calls and for the calls the
// SDK does not cover, in debug mode too, whose log reads the body first.
func TestClient_AResponseLargerThanTheLimitFails(t *testing.T) {
	previous := maxResponseBytes
	maxResponseBytes = 16
	t.Cleanup(func() { maxResponseBytes = previous })
	body := `{"message":"a body of more than sixteen bytes"}`

	tests := map[string]func(*Client) (any, error){
		"domain get (SDK)": func(c *Client) (any, error) { return c.GetDomain("example.com") },
		"route trigger":    func(c *Client) (any, error) { return c.TriggerRoute(uuid.New()) },
	}
	for name, call := range tests {
		for _, status := range []int{http.StatusOK, http.StatusNotFound} {
			for _, debug := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/debug=%t", name, status, debug), func(t *testing.T) {
					useLogger(t, debug)
					client := recordedBodyFor(t, status, body)

					_, err := call(client)

					var tooLarge *responseTooLargeError
					require.ErrorAs(t, err, &tooLarge)
					assert.Equal(t, "the API response is larger than 16 bytes, the most the CLI reads", err.Error())
					assert.Equal(t, 1, clierrors.GetExitCode(err))
				})
			}
		}
	}
}

func TestClient_AResponseAtTheLimitIsRead(t *testing.T) {
	body := `{"message":"ok"}`
	previous := maxResponseBytes
	maxResponseBytes = int64(len(body))
	t.Cleanup(func() { maxResponseBytes = previous })

	result, err := recordedBodyFor(t, http.StatusOK, body).TriggerRoute(uuid.New())

	require.NoError(t, err)
	assert.Equal(t, "ok", result.Message)
}

func TestResponseTooLargeErrorNamesTheLimitInMiB(t *testing.T) {
	assert.Equal(t, "the API response is larger than 32 MiB, the most the CLI reads",
		(&responseTooLargeError{Limit: 32 << 20}).Error())
}

// useLogger makes the global logger log in debug mode, or not, for the rest
// of the test, to a discarded output.
func useLogger(t *testing.T, debug bool) {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("debug", debug, "")
	cmd.SetErr(io.Discard)
	logger.Initialize(cmd)
	t.Cleanup(func() { logger.Initialize(&cobra.Command{}) })
}
