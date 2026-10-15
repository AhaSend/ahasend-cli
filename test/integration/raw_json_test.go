package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawDomainBody is a domain response that re-encoding the SDK struct cannot
// reproduce: "domain" comes before "object", "future_field" is unknown to the
// SDK, "dns_valid" (which the SDK always encodes) is left out, and the number
// literals do not survive a float64 round trip.
const rawDomainBody = `{"domain":"example.com","object":"domain",` +
	`"id":"22222222-2222-2222-2222-222222222222",` +
	`"account_id":"00000000-0000-0000-0000-0000000000aa",` +
	`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z",` +
	`"dns_records":[],"future_field":{"ratio":1.50,"big":12345678901234567890}}`

const rawValidationErrorBody = `{"message":"domain is invalid","errors":{"domain":["must be a valid domain"]},"ratio":1.50}`

// TestCLIPrintsRawAPIBodyInJSONMode runs the real binary against a server that
// sends rawDomainBody, so the SDK decode, the client wrapper and the printer
// all take part.
func TestCLIPrintsRawAPIBodyInJSONMode(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the CLI binary")
	}

	binary := buildCLIBinary(t)

	tests := []struct {
		name     string
		status   int
		body     string
		wantCode int
	}{
		{name: "success", status: http.StatusOK, body: rawDomainBody, wantCode: 0},
		{name: "validation error", status: http.StatusUnprocessableEntity, body: rawValidationErrorBody, wantCode: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			stdout, stderr, code := runCLI(t, binary, writeProfile(t, server.URL),
				"domains", "get", "example.com", "--output", "json")

			assert.Equal(t, tt.wantCode, code, "stderr: %s", stderr)
			var want bytes.Buffer
			require.NoError(t, json.Indent(&want, []byte(tt.body), "", "  "))
			assert.Equal(t, want.String()+"\n", stdout)
		})
	}
}
