package apiresponse

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type result struct {
	ID string
}

func TestBody_ReturnsRememberedBody(t *testing.T) {
	r := &result{ID: "a"}
	Remember(r, []byte(`{"id":"a"}`))

	body, ok := Body(r)

	require.True(t, ok)
	assert.Equal(t, `{"id":"a"}`, string(body))
}

func TestBody_EqualValuesAtDistinctPointersDoNotCollide(t *testing.T) {
	first := &result{ID: "same"}
	second := &result{ID: "same"}
	Remember(first, []byte(`{"n":1}`))
	Remember(second, []byte(`{"n":2}`))

	body, ok := Body(first)

	require.True(t, ok)
	assert.Equal(t, `{"n":1}`, string(body))
}

func TestBody_MissesForUnrecordedValue(t *testing.T) {
	_, ok := Body(&result{ID: "never recorded"})

	assert.False(t, ok)
}

func TestBody_ConcurrentRememberAndBody(t *testing.T) {
	const workers = 20
	results := make([]*result, workers)
	for i := range results {
		results[i] = &result{}
	}

	var wg sync.WaitGroup
	for i, r := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Remember(r, []byte(fmt.Sprintf(`{"i":%d}`, i)))
			_, _ = Body(r)
		}()
	}
	wg.Wait()

	for i, r := range results {
		body, ok := Body(r)
		require.True(t, ok)
		assert.Equal(t, fmt.Sprintf(`{"i":%d}`, i), string(body))
	}
}
