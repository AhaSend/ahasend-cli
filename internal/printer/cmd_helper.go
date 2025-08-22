package printer

import (
	"github.com/spf13/cobra"
)

// GetResponseHandlerFromCommand retrieves the response handler instance from the command context
// This is the main function commands should use to get their response handler
func GetResponseHandlerFromCommand(cmd *cobra.Command) ResponseHandler {
	// Context key for the response handler instance - must match root.go
	const handlerKey = "responseHandler"

	if h := cmd.Context().Value(handlerKey); h != nil {
		if handlerInstance, ok := h.(ResponseHandler); ok {
			return handlerInstance
		}
	}

	// Fallback to table handler if context is not available
	// This shouldn't happen in normal execution since root command sets it up
	return GetResponseHandler("table", true, cmd.OutOrStdout())
}

// ValidateOutputFormat validates the --output flag value
// Commands can use this for early validation if needed
func ValidateOutputFormat(cmd *cobra.Command) error {
	outputFormat, _ := cmd.Flags().GetString("output")
	return ValidateFormat(outputFormat)
}
