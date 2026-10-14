package client

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/testenv"
)

// TestDefaultHostIsUnreachableInTests checks that a client built without an
// API URL, as tests do when they forget to point one at a test server, cannot
// reach the AhaSend API: both the SDK calls and the CLI's own requests stop
// at the blocked proxy testenv.Main installs.
func TestDefaultHostIsUnreachableInTests(t *testing.T) {
	client, err := NewClient("aha-sk-test", uuid.New().String())
	require.NoError(t, err)
	require.Equal(t, "api.ahasend.com", client.config.Host)
	// Retrying a refused connection only slows the test down.
	client.config.RetryConfig.Enabled = false

	calls := map[string]func() error{
		"SDK ping": func() error {
			_, err := client.Ping()
			return err
		},
		"webhook trigger": func() error {
			_, err := client.TriggerWebhook(uuid.New(), []string{"message.delivered"})
			return err
		},
		"route trigger": func() error {
			_, err := client.TriggerRoute(uuid.New())
			return err
		},
		"webhook stream": func() error {
			webhookID := uuid.New()
			_, err := client.InitiateWebhookStream(&webhookID)
			return err
		},
		"route stream": func() error {
			routeID := uuid.New()
			_, err := client.InitiateRouteStream(&routeID, "")
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "proxyconnect")
			assert.Contains(t, err.Error(), strings.TrimPrefix(testenv.BlockedProxy, "http://"))
		})
	}
}
