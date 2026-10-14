package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the apikeys get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key-id>",
		Short: "Get an API key",
		Long: `Get detailed information about a specific API key by its ID.

This command shows comprehensive details about an API key including:
- Key ID and label
- Scopes and permissions
- Creation and last updated timestamps
- Key status

The secret value is never displayed for security reasons. If you need to
retrieve the secret, you must create a new API key.`,
		Example: `  # Get API key details
  ahasend apikeys get fcb3f3bc-4ac8-4330-948d-1671fcf9a768

  # JSON output for automation
  ahasend apikeys get fcb3f3bc-4ac8-4330-948d-1671fcf9a768 --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runAPIKeyGet,
		SilenceUsage: true,
	}

	return cmd
}

func runAPIKeyGet(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	keyID := args[0]
	keyUUID, err := validation.ParseUUIDField("API key ID", keyID)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get API key details
	apiKey, err := client.GetAPIKey(keyUUID)
	if err != nil {
		return err
	}

	// Handle successful response
	return handler.HandleSingleAPIKey(apiKey, printer.SingleConfig{
		SuccessMessage: "API key retrieved successfully",
		EmptyMessage:   "API key not found",
	})
}
