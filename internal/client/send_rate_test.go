package client

import (
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentSendsKeepTheAPIRate checks that sends from MaxSendConcurrency
// workers stay within the API's send rate, which the SDK's send limiter keeps:
// its burst plus one second of its rate, plus the requests in flight, in any
// one-second window.
func TestConcurrentSendsKeepTheAPIRate(t *testing.T) {
	const (
		sends = 400
		rate  = 100
		burst = 200
	)
	var mu sync.Mutex
	var arrivals []time.Time
	client, cleanup := newClientTestServer(t, uuid.New().String(), func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		arrivals = append(arrivals, time.Now())
		mu.Unlock()
		writeClientTestJSON(t, w, http.StatusAccepted, map[string]any{"object": "list", "data": []any{}})
	})
	defer cleanup()

	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < MaxSendConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				_, err := client.SendMessageWithIdempotencyKey(requests.CreateMessageRequest{
					From:        common.SenderAddress{Email: "sender@example.com"},
					Recipients:  []common.Recipient{{Email: "user@example.com"}},
					Subject:     "Hi",
					TextContent: ahasend.String("Hi"),
				}, uuid.NewString())
				assert.NoError(t, err)
			}
		}()
	}
	for i := 0; i < sends; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, arrivals, sends)
	sort.Slice(arrivals, func(i, j int) bool { return arrivals[i].Before(arrivals[j]) })
	busiest := 0
	for i, j := 0, 0; i < len(arrivals); i++ {
		for arrivals[i].Sub(arrivals[j]) >= time.Second {
			j++
		}
		if n := i - j + 1; n > busiest {
			busiest = n
		}
	}
	assert.LessOrEqual(t, busiest, burst+rate+MaxSendConcurrency, "busiest second held %d sends", busiest)
}
