package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
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
		Args:         cobra.ExactArgs(1),
		RunE:         runSubAccountAPIKeysList,
		SilenceUsage: true,
	}

	cmd.Flags().Int32("limit", 0, "Maximum number of API keys to return")
	cmd.Flags().String("cursor", "", "Pagination cursor for next page")

	return cmd
}

func runSubAccountAPIKeysList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Get flags
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")

	// Validate before auth
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
		return err
	}
	if err := validation.ValidatePageLimit(limit); err != nil {
		return err
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Handle pagination: pass nil pointers when unset
	var limitPtr *int32
	var cursorPtr *string

	if limit > 0 {
		limitPtr = &limit
	}
	if cursor != "" {
		cursorPtr = &cursor
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"limit":          limit,
		"cursor":         cursor,
	}).Debug("Executing subaccounts api-keys list command")

	// Fetch the sub-account's API keys
	response, err := client.ListSubAccountAPIKeys(subAccountID, limitPtr, cursorPtr)
	if err != nil {
		return err
	}

	// Reuse the shared API-key list renderer so output matches the top-level
	// apikeys command and JSON stays a verbatim SDK PaginatedAPIKeysResponse.
	return handler.HandleAPIKeyList(response, printer.ListConfig{
		SuccessMessage: "API Keys Retrieved Successfully",
		EmptyMessage:   "No API keys found",
		ShowPagination: true,
		FieldOrder:     []string{"id", "label", "scopes", "created_at", "updated_at"},
	})
}
