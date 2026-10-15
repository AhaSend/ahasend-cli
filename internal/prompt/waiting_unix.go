//go:build unix

package prompt

import (
	"os"

	"golang.org/x/sys/unix"
)

// inputWaiting reports whether the terminal in has input that was not read
// yet, by polling it without waiting: the bytes of a paste arrive together,
// so the LF of a pasted CRLF already waits when its CR is read, while a typed
// Enter leaves nothing. A terminal that cannot be polled reports none.
func inputWaiting(in *os.File) bool {
	fds := []unix.PollFd{{Fd: int32(in.Fd()), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, 0)
	return err == nil && n > 0 && fds[0].Revents&unix.POLLIN != 0
}
