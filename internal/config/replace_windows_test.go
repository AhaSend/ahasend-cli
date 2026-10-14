//go:build windows

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/testenv"
	"github.com/stretchr/testify/require"
)

// TestSaveWaitsForAReaderToCloseTheFile checks that Save replaces a
// configuration file another process reads, which Windows refuses while the
// file is open.
func TestSaveWaitsForAReaderToCloseTheFile(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	mgr, err := NewManager()
	require.NoError(t, err)
	require.NoError(t, mgr.Save())
	reader, err := os.Open(filepath.Join(home, ".ahasend", "config.yaml"))
	require.NoError(t, err)
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = reader.Close()
	}()

	require.NoError(t, mgr.SetProfile("work", Profile{APIKey: "aha-sk-work"}))
	require.NoError(t, mgr.Save())
}
