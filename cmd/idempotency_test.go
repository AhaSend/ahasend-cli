package cmd

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// TestCreateCommandsSendTheIdempotencyKey checks that the create commands of
// the domain, webhook, route, SMTP credential and suppression groups send
// --idempotency-key as the request's Idempotency-Key.
func TestCreateCommandsSendTheIdempotencyKey(t *testing.T) {
	tests := []struct {
		args []string
		call apiCall
	}{
		{[]string{"domains", "create", "example.com"}, returns[responses.Domain]("CreateDomain", csvItemBody)},
		{[]string{"webhooks", "create", "--name", "Hook", "--url", "https://example.com", "--all-events"},
			returns[responses.Webhook]("CreateWebhook", csvItemBody)},
		{[]string{"routes", "create", "--name", "Inbound", "--url", "https://example.com/hook", "--recipient", "*@example.com"},
			returns[responses.Route]("CreateRoute", csvItemBody)},
		{[]string{"smtp", "create", "--name", "App", "--scope", "global"},
			returns[responses.SMTPCredential]("CreateSMTPCredential", csvItemBody)},
		{[]string{"suppressions", "create", "user@example.com", "--expires", "30d"},
			returns[responses.CreateSuppressionResponse]("CreateSuppression", csvListBody)},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On(tt.call.method, mock.Anything, testutil.IdempotencyKeyArg("retry-1")).Return(tt.call.result(t), nil)
			testutil.InjectClient(t, mockClient)

			_, stderr, code := runRoot(t, append(tt.args, "--idempotency-key", "retry-1", "--output", "json")...)

			require.Equal(t, 0, code, stderr)
			mockClient.AssertExpectations(t)
		})
	}
}
