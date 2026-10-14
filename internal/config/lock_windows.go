//go:build windows

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// lockDir takes the exclusive lock of the configuration directory dir,
// waiting while another process holds it, and returns the function that
// releases it. The lock is a LockFileEx lock on the file config.lock in dir;
// the operating system releases it when the process exits.
func lockDir(dir string) (unlock func(), err error) {
	file, err := os.OpenFile(filepath.Join(dir, lockFileName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &windows.Overlapped{})
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to lock %s: %w", file.Name(), err)
	}
	return func() { _ = file.Close() }, nil
}
