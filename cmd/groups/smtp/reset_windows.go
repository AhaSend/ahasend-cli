package smtp

import "syscall"

// resetErrors are the errors of a connection the server reset or aborted,
// read or written to after that. Windows reports them with its own socket
// error numbers.
var resetErrors = []error{syscall.WSAECONNRESET, syscall.WSAECONNABORTED}
