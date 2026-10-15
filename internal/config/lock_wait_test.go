//go:build unix || windows

package config

import (
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/testenv"
)

// lockedBuffer is a buffer that Update's notice and the test may use at
// once.
type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestUpdateSaysWhatItWaitsFor checks that an Update that waits for the
// lock another command holds says so on stderr, naming the lock file, and
// goes on once the lock is released.
func TestUpdateSaysWhatItWaitsFor(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	lockWaitNotice = 10 * time.Millisecond
	t.Cleanup(func() { lockWaitNotice = time.Second })
	require.NoError(t, Update(io.Discard, func(*Manager) error { return nil }))
	dir := filepath.Join(home, ".ahasend")
	unlock, err := lockDir(dir)
	require.NoError(t, err)

	var stderr lockedBuffer
	done := make(chan error, 1)
	go func() { done <- Update(&stderr, func(*Manager) error { return nil }) }()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(stderr.String(), "Waiting for") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	unlock()

	require.NoError(t, <-done)
	assert.Equal(t, "Waiting for "+filepath.Join(dir, "config.lock")+
		": another ahasend command is changing the configuration\n", stderr.String())
}

// TestUpdateSaysNothingWithoutAWait checks that an Update that gets the lock
// at once writes nothing to stderr.
func TestUpdateSaysNothingWithoutAWait(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	var stderr lockedBuffer

	require.NoError(t, Update(&stderr, func(*Manager) error { return nil }))

	assert.Empty(t, stderr.String())
}
