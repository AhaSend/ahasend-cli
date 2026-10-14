package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONHandlersPrintAPIResponsesFaithfully checks that JSON output is the
// API response encoded as it is: keys in the SDK type's order, absent
// omitempty fields left out, and a nil list kept as null.
func TestJSONHandlersPrintAPIResponsesFaithfully(t *testing.T) {
	domain := newTestPausedDomain()
	domain.Paused, domain.PausedAt, domain.PauseReason = false, nil, nil
	message := faithfulJSONMessage()
	suppression := newTestProtectedSuppression()
	suppression.Domain = ""
	key := apiKeyAllowing(nil)
	createdMessage := &responses.CreateMessageResponse{
		Object: "list",
		Data: []responses.CreateSingleMessageResponse{
			{Object: "message", Recipient: common.Recipient{Email: "user@example.com"}, Status: "queued"},
		},
	}

	tests := []struct {
		name   string
		value  any
		render func(ResponseHandler) error
	}{
		{"domain list", &responses.PaginatedDomainsResponse{Object: "list", Data: []responses.Domain{*domain}},
			func(h ResponseHandler) error {
				return h.HandleDomainList(&responses.PaginatedDomainsResponse{Object: "list", Data: []responses.Domain{*domain}}, ListConfig{})
			}},
		{"single domain", domain,
			func(h ResponseHandler) error { return h.HandleSingleDomain(domain, SingleConfig{}) }},
		{"message list", &responses.PaginatedMessagesResponse{Object: "list", Data: []responses.Message{*message}},
			func(h ResponseHandler) error {
				return h.HandleMessageList(&responses.PaginatedMessagesResponse{Object: "list", Data: []responses.Message{*message}}, ListConfig{})
			}},
		{"single message", message,
			func(h ResponseHandler) error { return h.HandleSingleMessage(message, SingleConfig{}) }},
		{"create message", createdMessage,
			func(h ResponseHandler) error { return h.HandleCreateMessage(createdMessage, ResultConfig{}) }},
		{"suppression list", &responses.PaginatedSuppressionsResponse{Object: "list", Data: []responses.Suppression{*suppression}},
			func(h ResponseHandler) error {
				return h.HandleSuppressionList(&responses.PaginatedSuppressionsResponse{Object: "list", Data: []responses.Suppression{*suppression}}, ListConfig{})
			}},
		{"create suppression", &responses.CreateSuppressionResponse{Object: "list", Data: []responses.Suppression{*suppression}},
			func(h ResponseHandler) error {
				return h.HandleCreateSuppression(&responses.CreateSuppressionResponse{Object: "list", Data: []responses.Suppression{*suppression}}, ResultConfig{})
			}},
		{"check suppression", &responses.PaginatedSuppressionsResponse{Object: "list", Data: []responses.Suppression{*suppression}},
			func(h ResponseHandler) error {
				return h.HandleCheckSuppression(&responses.PaginatedSuppressionsResponse{Object: "list", Data: []responses.Suppression{*suppression}}, CheckConfig{})
			}},
		{"api key list", &responses.PaginatedAPIKeysResponse{Object: "list", Data: []responses.APIKey{*key}},
			func(h ResponseHandler) error {
				return h.HandleAPIKeyList(&responses.PaginatedAPIKeysResponse{Object: "list", Data: []responses.APIKey{*key}}, ListConfig{})
			}},
		{"single api key", key,
			func(h ResponseHandler) error { return h.HandleSingleAPIKey(key, SingleConfig{}) }},
		{"create api key", key,
			func(h ResponseHandler) error { return h.HandleCreateAPIKey(key, ResultConfig{}) }},
		{"update api key", key,
			func(h ResponseHandler) error { return h.HandleUpdateAPIKey(key, ResultConfig{}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expected bytes.Buffer
			encoder := json.NewEncoder(&expected)
			encoder.SetEscapeHTML(false)
			encoder.SetIndent("", "  ")
			require.NoError(t, encoder.Encode(tt.value))

			var output bytes.Buffer
			require.NoError(t, tt.render(testHandler(t, "json", &output, os.Stderr)))
			assert.Equal(t, expected.String(), output.String())
		})
	}
}

func faithfulJSONMessage() *responses.Message {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &responses.Message{
		Object:      "message",
		ID:          uuid.MustParse("77777777-7777-4777-8777-777777777777"),
		MessageID:   "<message@example.com>",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
		RetainUntil: createdAt.Add(24 * time.Hour),
		Subject:     "Hello",
		Sender:      "sender@example.com",
		Recipient:   "user@example.com",
		Direction:   "outgoing",
		Status:      "delivered",
		DomainID:    uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		AccountID:   uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	}
}

// TestUnreadableResponsePrintsTheBodyAndAnError checks that a successful
// response the CLI could not read prints the API's body on stdout in JSON
// output, and the error on stderr in every format.
func TestUnreadableResponsePrintsTheBodyAndAnError(t *testing.T) {
	err := clierrors.NewUnreadableResponseError([]byte(`{"domain":5}`), "domain must be a string")
	for _, format := range []string{"json", "table", "plain", "csv"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			require.Error(t, testHandler(t, format, &stdout, &stderr).HandleError(err))

			assert.Equal(t, "Error: could not read the API response: domain must be a string\n", stderr.String())
			if format == "json" {
				assert.Equal(t, "{\n  \"domain\": 5\n}\n", stdout.String())
			} else {
				assert.Empty(t, stdout.String())
			}
		})
	}
}
