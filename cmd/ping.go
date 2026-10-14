package cmd

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// newPingCommand creates the ping command.
func newPingCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Test the API connection and that the API accepts the key",
		Long: `Test the connection to the AhaSend API and that the API accepts your API key.

This command sends a ping request to the AhaSend API, which checks that:
- The API can be reached
- The API accepts the API key, from this IP address

The API answers a ping before it looks at the account, so the ping does not
check that the key belongs to the account ID or what the key may do; run
'ahasend auth status' for that.`,
		Example: `  # Test with current profile
  ahasend ping

  # Test with specific profile
  ahasend ping --profile production`,
		Args:         cmdutil.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get client from auth helper
			ahasendClient, err := auth.GetAuthenticatedClient(cmd)
			if err != nil {
				return err
			}

			// Get response handler instance
			handler := printer.GetResponseHandlerFromCommand(cmd)

			// Perform ping
			response, err := ahasendClient.Ping()
			if err != nil {
				return err
			}

			return handler.HandleSuccessResponse(response, printer.ResultConfig{SuccessMessage: "pong"})
		},
	}
}
