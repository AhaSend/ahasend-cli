//go:build windows

package config

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// Windows refuses to replace a file another process has open, such as a
// command reading the configuration, which reads without the lock. The
// reader closes it at once, so replaceFile tries again a few times.
const (
	replaceAttempts   = 10
	replaceRetryDelay = 50 * time.Millisecond
)

// replaceFile replaces the file to with the file from, trying again while
// another process has the file open.
func replaceFile(from, to string) error {
	var err error
	for attempt := 1; attempt <= replaceAttempts; attempt++ {
		err = os.Rename(from, to)
		inUse := errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
		if !inUse {
			return err
		}
		time.Sleep(replaceRetryDelay)
	}
	return err
}
