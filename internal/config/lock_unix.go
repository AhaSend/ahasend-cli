//go:build unix

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// lockDir takes the exclusive lock of the configuration directory dir,
// waiting while another process holds it, and returns the function that
// releases it. The lock is an flock on the file config.lock in dir; the
// operating system releases it when the process exits.
func lockDir(dir string) (unlock func(), err error) {
	file, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to lock %s: %w", file.Name(), err)
	}
	return func() { _ = file.Close() }, nil
}
