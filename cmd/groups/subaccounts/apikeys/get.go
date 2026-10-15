package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the `subaccounts api-keys get` command.
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <sub-account-id> <key-id>",
		Short: "Get a sub-account API key",
		Long: `Get detailed information about a specific API key that belongs to a
sub-account, including its label, scopes, and timestamps.

The secret value is never displayed for security reasons.`,
		Example: `  # Get a sub-account API key's details
  ahasend subaccounts api-keys get 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000

  # Get details with JSON output
  ahasend subaccounts api-keys get 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 --output json`,
		Args:         cmdutil.ExactArgs(2),
		RunE:         runSubAccountAPIKeyGet,
		SilenceUsage: true,
	}

	return cmd
}

func runSubAccountAPIKeyGet(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]
	keyID := args[1]

	// Validate before auth
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
		return err
	}
	keyUUID, err := validation.ParseUUIDField("API key ID", keyID)
	if err != nil {
		return err
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"key_id":         keyID,
	}).Debug("Executing subaccounts api-keys get command")

	// Get the sub-account's API key
	apiKey, err := client.GetSubAccountAPIKey(subAccountUUID, keyUUID)
	if err != nil {
		return err
	}

	// Reuse the shared single-API-key renderer for output parity with the
	// top-level apikeys command.
	return handler.HandleSingleAPIKey(apiKey, printer.SingleConfig{
		SuccessMessage: "API key retrieved successfully",
		EmptyMessage:   "API key not found",
	})
}
