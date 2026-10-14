//go:build unix

package cmdutil

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadAttachmentsRefusesFilesThatAreNotRegular checks that a device and a
// named pipe, which have no size to check and could be read without end or
// wait for a writer, are refused without being read.
func TestReadAttachmentsRefusesFilesThatAreNotRegular(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "pipe")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))

	for _, path := range []string{"/dev/zero", fifo} {
		t.Run(path, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				_, err := ReadAttachments([]string{path})
				done <- err
			}()

			select {
			case err := <-done:
				require.EqualError(t, err, "--attach "+path+" is not a regular file")
				assert.Equal(t, 4, clierrors.GetExitCode(err))
			case <-time.After(5 * time.Second):
				t.Fatal("reading the file did not end")
			}
		})
	}
}
