package printer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
)

// rawAPIBody is an API body that re-encoding a decoded SDK struct cannot
// reproduce: its keys are out of alphabetical order, it carries a key no SDK
// struct has, and its number literals do not survive a float64 round trip.
const rawAPIBody = `{"object":"thing","zeta":1.50,"future_field":{"nested":[1,"x"]},` +
	`"big":12345678901234567890,"alpha":"<b>&</b>"}`

// indentedBody is the form printRaw prints body in.
func indentedBody(t *testing.T, body string) string {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, json.Indent(&buffer, []byte(body), "", "  "))
	return buffer.String() + "\n"
}

// recorded returns a new T with rawAPIBody recorded as its API response.
func recorded[T any]() *T {
	value := new(T)
	apiresponse.Remember(value, []byte(rawAPIBody))
	return value
}

// TestJSONHandlersPrintRecordedAPIBody checks that every JSON handler whose
// argument is an SDK response prints the body the API returned for it.
func TestJSONHandlersPrintRecordedAPIBody(t *testing.T) {
	tests := []struct {
		name   string
		render func(ResponseHandler) error
	}{
		{"domain list", func(h ResponseHandler) error {
			return h.HandleDomainList(recorded[responses.PaginatedDomainsResponse](), ListConfig{})
		}},
		{"single domain", func(h ResponseHandler) error {
			return h.HandleSingleDomain(recorded[responses.Domain](), SingleConfig{})
		}},
		{"message list", func(h ResponseHandler) error {
			return h.HandleMessageList(recorded[responses.PaginatedMessagesResponse](), ListConfig{})
		}},
		{"single message", func(h ResponseHandler) error {
			return h.HandleSingleMessage(recorded[responses.Message](), SingleConfig{})
		}},
		{"create message", func(h ResponseHandler) error {
			return h.HandleCreateMessage(recorded[responses.CreateMessageResponse](), ResultConfig{})
		}},
		{"webhook list", func(h ResponseHandler) error {
			return h.HandleWebhookList(recorded[responses.PaginatedWebhooksResponse](), ListConfig{})
		}},
		{"single webhook", func(h ResponseHandler) error {
			return h.HandleSingleWebhook(recorded[responses.Webhook](), SingleConfig{})
		}},
		{"create webhook", func(h ResponseHandler) error {
			return h.HandleCreateWebhook(recorded[responses.Webhook](), ResultConfig{})
		}},
		{"update webhook", func(h ResponseHandler) error {
			return h.HandleUpdateWebhook(recorded[responses.Webhook](), ResultConfig{})
		}},
		{"route list", func(h ResponseHandler) error {
			return h.HandleRouteList(recorded[responses.PaginatedRoutesResponse](), ListConfig{})
		}},
		{"single route", func(h ResponseHandler) error {
			return h.HandleSingleRoute(recorded[responses.Route](), SingleConfig{})
		}},
		{"create route", func(h ResponseHandler) error {
			return h.HandleCreateRoute(recorded[responses.Route](), ResultConfig{})
		}},
		{"update route", func(h ResponseHandler) error {
			return h.HandleUpdateRoute(recorded[responses.Route](), ResultConfig{})
		}},
		{"suppression list", func(h ResponseHandler) error {
			return h.HandleSuppressionList(recorded[responses.PaginatedSuppressionsResponse](), ListConfig{})
		}},
		{"check suppression", func(h ResponseHandler) error {
			return h.HandleCheckSuppression(recorded[responses.PaginatedSuppressionsResponse](), CheckConfig{})
		}},
		{"create suppression", func(h ResponseHandler) error {
			return h.HandleCreateSuppression(recorded[responses.CreateSuppressionResponse](), ResultConfig{})
		}},
		{"smtp list", func(h ResponseHandler) error {
			return h.HandleSMTPList(recorded[responses.PaginatedSMTPCredentialsResponse](), ListConfig{})
		}},
		{"single smtp", func(h ResponseHandler) error {
			return h.HandleSingleSMTP(recorded[responses.SMTPCredential](), SingleConfig{})
		}},
		{"create smtp", func(h ResponseHandler) error {
			return h.HandleCreateSMTP(recorded[responses.SMTPCredential](), ResultConfig{})
		}},
		{"api key list", func(h ResponseHandler) error {
			return h.HandleAPIKeyList(recorded[responses.PaginatedAPIKeysResponse](), ListConfig{})
		}},
		{"single api key", func(h ResponseHandler) error {
			return h.HandleSingleAPIKey(recorded[responses.APIKey](), SingleConfig{})
		}},
		{"create api key", func(h ResponseHandler) error {
			return h.HandleCreateAPIKey(recorded[responses.APIKey](), ResultConfig{})
		}},
		{"update api key", func(h ResponseHandler) error {
			return h.HandleUpdateAPIKey(recorded[responses.APIKey](), ResultConfig{})
		}},
		{"sub-account list", func(h ResponseHandler) error {
			return h.HandleSubAccountList(recorded[responses.PaginatedSubAccountsResponse](), ListConfig{})
		}},
		{"single sub-account", func(h ResponseHandler) error {
			return h.HandleSingleSubAccount(recorded[responses.SubAccount](), SingleConfig{})
		}},
		{"sub-account usage", func(h ResponseHandler) error {
			return h.HandleSubAccountUsage(recorded[responses.SubAccountUsageResponse](), SingleConfig{})
		}},
		{"deliverability stats", func(h ResponseHandler) error {
			return h.HandleDeliverabilityStats(recorded[responses.DeliverabilityStatisticsResponse](), StatsConfig{})
		}},
		{"bounce stats", func(h ResponseHandler) error {
			return h.HandleBounceStats(recorded[responses.BounceStatisticsResponse](), StatsConfig{})
		}},
		{"delivery time stats", func(h ResponseHandler) error {
			return h.HandleDeliveryTimeStats(recorded[responses.DeliveryTimeStatisticsResponse](), StatsConfig{})
		}},
		{"success response", func(h ResponseHandler) error {
			return h.HandleSuccessResponse(recorded[common.SuccessResponse](), ResultConfig{})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, tt.render(testHandler(t, "json", &output, os.Stderr)))
			assert.Equal(t, indentedBody(t, rawAPIBody), output.String())
		})
	}
}

// TestPrintRaw_WritesInvalidJSONUnchanged checks that a body that is not JSON
// is written as it is, ending with a newline, and an empty body prints
// nothing.
func TestPrintRaw_WritesInvalidJSONUnchanged(t *testing.T) {
	tests := map[string]string{
		"not json":     "not json\n",
		"not json\n":   "not json\n",
		"not json  \n": "not json  \n",
		"":             "",
	}
	for body, want := range tests {
		t.Run(body, func(t *testing.T) {
			var output bytes.Buffer
			handler := &jsonHandler{handlerBase{writer: &output}}

			require.NoError(t, handler.printRaw([]byte(body)))

			assert.Equal(t, want, output.String())
		})
	}
}

func TestJSONHandleError_PrintsWrappedAPIErrorBodyUnchanged(t *testing.T) {
	body := `{"message":"invalid request","zeta":1.50,"big":12345678901234567890,"alpha":true}`
	apiErr := &api.APIError{StatusCode: 422, Message: "invalid request", Raw: []byte(body)}
	var output bytes.Buffer

	err := testHandler(t, "json", &output, os.Stderr).HandleError(fmt.Errorf("failed to get stats: %w", apiErr))

	require.ErrorIs(t, err, apiErr)
	assert.Equal(t, indentedBody(t, body), output.String())
}

func TestJSONHandleError_NonJSONAPIErrorBodyPrintsErrorObject(t *testing.T) {
	apiErr := &api.APIError{StatusCode: 502, Message: "bad gateway", Raw: []byte("<html>502</html>")}
	var output bytes.Buffer

	err := testHandler(t, "json", &output, os.Stderr).HandleError(apiErr)

	require.ErrorIs(t, err, apiErr)
	var printed map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &printed))
	assert.Equal(t, map[string]any{
		"error": true, "message": "bad gateway", "status_code": float64(502), "request_id": "",
	}, printed)
}
