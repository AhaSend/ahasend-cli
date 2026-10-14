package client

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientReusesAConnectionForEachConcurrentSend checks that the most
// concurrent sends messages send makes, 20 requests each, open about as many
// connections as there are sends in flight: each finished request leaves its
// connection open for the next.
func TestClientReusesAConnectionForEachConcurrentSend(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses.CreateMessageResponse{Object: "list", Data: []responses.CreateSingleMessageResponse{
			{Object: "message", Status: "queued"},
		}})
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()
	client, err := NewClient("test-api-key", "11111111-1111-4111-8111-111111111111", server.URL)
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, MaxSendConcurrency*20)
	for worker := range MaxSendConcurrency {
		wg.Go(func() {
			for i := range 20 {
				_, err := client.SendTemplateMessageWithIdempotencyKey(requests.CreateTemplateMessageRequest{
					TemplateID: uuid.New(), Recipients: []common.Recipient{{Email: "user@example.com"}},
				}, fmt.Sprintf("key-%d-%d", worker, i))
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	// A send can start just before the connection of the one before is back
	// in the pool, so a few more may open; without the pool, most do.
	assert.LessOrEqual(t, int(connections.Load()), MaxSendConcurrency+MaxSendConcurrency/2)
}
