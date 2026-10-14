package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// TestFixedValuesAcceptAnyCaseAndSendTheAPISpelling gives every fixed-value
// flag that reaches a request a value in another case, and checks that the
// request carries the API's spelling.
func TestFixedValuesAcceptAnyCaseAndSendTheAPISpelling(t *testing.T) {
	tests := []struct {
		args  []string
		call  apiCall
		given string // the value given
		sent  string // the value the request must carry
	}{
		{[]string{"domains", "list", "--sending-type", "MARKETING"},
			returns[responses.PaginatedDomainsResponse]("ListDomains", csvListBody), "MARKETING", `"marketing"`},
		{[]string{"domains", "create", "example.com", "--sending-type", "Marketing"},
			returns[responses.Domain]("CreateDomain", csvItemBody), "Marketing", `"marketing"`},
		{[]string{"domains", "update", "example.com", "--sending-type", "MARKETING"},
			returns[responses.Domain]("UpdateDomain", csvItemBody), "MARKETING", `"marketing"`},
		{[]string{"messages", "list", "--status", "DELIVERED", "--status", "sandbox bounced"},
			returns[responses.PaginatedMessagesResponse]("GetMessages", csvListBody), "DELIVERED", `"Delivered,Sandbox Bounced"`},
		{[]string{"stats", "bounces", "--group-by", "WEEK"},
			returns[responses.BounceStatisticsResponse]("GetBounceStatistics", csvItemBody), "WEEK", `"week"`},
		{[]string{"webhooks", "create", "--name", "Hook", "--url", "https://example.com", "--scope", "GLOBAL", "--all-events"},
			returns[responses.Webhook]("CreateWebhook", csvItemBody), "GLOBAL", `"global"`},
		{[]string{"webhooks", "update", rawTestID, "--scope", "Global"},
			returns[responses.Webhook]("UpdateWebhook", csvItemBody), "Global", `"global"`},
		{[]string{"webhooks", "update", rawTestID, "--events", "DELIVERED"},
			returns[responses.Webhook]("UpdateWebhook", csvItemBody), "DELIVERED", `"on_delivered":true`},
		{[]string{"smtp", "create", "--name", "App", "--scope", "GLOBAL"},
			returns[responses.SMTPCredential]("CreateSMTPCredential", csvItemBody), "GLOBAL", `"global"`},
		{append(sendArgs("--text", "Hi"), "--sandbox", "--sandbox-result", "BOUNCE"),
			returns[responses.CreateMessageResponse]("SendMessageWithIdempotencyKey", csvListBody), "BOUNCE", `"bounce"`},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetAccountID").Return(rawTestID).Maybe()
			expectAnyArgs(mockClient, tt.call.method, tt.call.result(t), nil)
			testutil.InjectClient(t, mockClient)

			_, stderr, code := runRoot(t, append(tt.args, "--output", "JSON")...)

			require.Equal(t, 0, code, stderr)
			request := sentArguments(t, mockClient, tt.call.method)
			assert.Contains(t, request, tt.sent)
			assert.NotContains(t, request, tt.given)
		})
	}
}

// sentArguments returns the arguments of the call to method as JSON.
func sentArguments(t *testing.T, mockClient *mocks.MockClient, method string) string {
	t.Helper()
	for _, call := range mockClient.Calls {
		if call.Method == method {
			encoded, err := json.Marshal(call.Arguments)
			require.NoError(t, err)
			return string(encoded)
		}
	}
	require.Failf(t, "no call", "%s was not called", method)
	return ""
}
