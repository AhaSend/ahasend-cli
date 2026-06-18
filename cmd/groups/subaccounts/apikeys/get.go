package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the `subaccounts api-keys get` command.
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <sub-account-id> <key-id>",
		Short: "Get detailed information about a sub-account API key",
		Long: `Get detailed information about a specific API key that belongs to a
sub-account, including its label, scopes, and timestamps.

The secret value is never displayed for security reasons.`,
		Example: `  # Get a sub-account API key's details
  ahasend subaccounts api-keys get 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000

  # Get details with JSON output
  ahasend subaccounts api-keys get 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 --output json`,
		Args:         cobra.ExactArgs(2),
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
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
		return err
	}
	if err := validation.ValidateUUIDField("API key ID", keyID); err != nil {
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
	apiKey, err := client.GetSubAccountAPIKey(subAccountID, keyID)
	if err != nil {
		return err
	}

	// Reuse the shared single-API-key renderer for output parity with the
	// top-level apikeys command.
	return handler.HandleSingleAPIKey(apiKey, printer.SingleConfig{
		SuccessMessage: "API Key Details",
		EmptyMessage:   "API key not found",
		FieldOrder:     []string{"id", "label", "scopes", "created_at", "updated_at"},
	})
}
