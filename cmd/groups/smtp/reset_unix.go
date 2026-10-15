//go:build unix

package smtp

import "syscall"

// resetErrors are the errors of a connection the server reset or aborted,
// read or written to after that.
var resetErrors = []error{syscall.ECONNRESET, syscall.ECONNABORTED, syscall.EPIPE}
