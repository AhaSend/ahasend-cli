package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewListCommand creates the `subaccounts api-keys list` command.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <sub-account-id>",
		Short: "List API keys for a sub-account",
		Long: `List all API keys that belong to a specific sub-account with their ID,
label, scopes, and timestamps.

The list can be paginated for large numbers of keys.

The secret value of API keys is never displayed for security reasons.`,
		Example: `  # List a sub-account's API keys
  ahasend subaccounts api-keys list 123e4567-e89b-12d3-a456-426614174000

  # List with JSON output
  ahasend subaccounts api-keys list 123e4567-e89b-12d3-a456-426614174000 --output json

  # List with pagination
  ahasend subaccounts api-keys list 123e4567-e89b-12d3-a456-426614174000 --limit 10`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSubAccountAPIKeysList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)

	return cmd
}

func runSubAccountAPIKeysList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Validate before auth
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
		return err
	}
	pagination, err := cmdutil.ReadPaginationParams(cmd)
	if err != nil {
		return err
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
	}).Debug("Executing subaccounts api-keys list command")

	// Fetch the sub-account's API keys
	response, err := client.ListSubAccountAPIKeys(subAccountUUID, pagination)
	if err != nil {
		return err
	}

	// Reuse the shared API-key list renderer so output matches the top-level
	// apikeys command and JSON stays a verbatim SDK PaginatedAPIKeysResponse.
	return handler.HandleAPIKeyList(response, printer.ListConfig{
		SuccessMessage: "API keys retrieved successfully",
		EmptyMessage:   "No API keys found",
	})
}
