package cmd

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Test API connection and key validity",
	Long: `Test the connection to AhaSend API and validate your API key.

This command sends a ping request to the AhaSend API to verify:
- Network connectivity to AhaSend servers
- API key authentication and validity
- Account access permissions

Examples:
  # Test with current profile
  ahasend ping

  # Test with specific API key
  ahasend ping --api-key aha-sk-... --account-id <account-id>

  # Test with specific profile
  ahasend ping --profile production`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get client from auth helper
		ahasendClient, err := auth.GetAuthenticatedClient(cmd)
		if err != nil {
			return err
		}

		// Get response handler instance
		handler := printer.GetResponseHandlerFromCommand(cmd)

		// Perform ping
		err = ahasendClient.Ping()

		if err != nil {
			// Return the original error to let wrapper handle it
			return err
		}

		// Success response with pong message
		return handler.HandleSimpleSuccess("pong")
	},
}
