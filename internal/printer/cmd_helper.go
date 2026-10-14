package printer

import (
	"github.com/spf13/cobra"
)

type responseHandlerKeyType string

// Context keys for printer instances
const ResponseHandlerKey = responseHandlerKeyType("responseHandler")

// GetResponseHandlerFromCommand retrieves the response handler instance from the command context
// This is the main function commands should use to get their response handler
func GetResponseHandlerFromCommand(cmd *cobra.Command) ResponseHandler {
	// Context key for the response handler instance - must match root.go
	// Using the same type as defined in root.go

	if h := cmd.Context().Value(ResponseHandlerKey); h != nil {
		if handlerInstance, ok := h.(ResponseHandler); ok {
			return handlerInstance
		}
	}

	// Fall back to plain, the CLI's default format, when the root command
	// did not set the handler up, as for a command run on its own
	return &plainHandler{handlerBase: handlerBase{writer: displayWriter{w: cmd.OutOrStdout()}, errWriter: displayWriter{w: cmd.ErrOrStderr()}}}
}
