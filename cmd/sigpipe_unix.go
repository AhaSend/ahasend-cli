//go:build unix

package cmd

import (
	"os/signal"
	"syscall"
)

// ignoreSIGPIPE makes a write to a closed stdout or stderr pipe, such as
// that of `ahasend messages send ... | head`, fail with EPIPE instead of
// killing the process, so the command still reports the failure and exits 1.
func ignoreSIGPIPE() {
	signal.Ignore(syscall.SIGPIPE)
}
