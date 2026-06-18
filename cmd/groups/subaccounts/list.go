package subaccounts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all sub-accounts",
		Long: `List all sub-accounts under your AhaSend parent account with their status,
domain and member counts, and monthly credit.

The list can be paginated for large numbers of sub-accounts.`,
		Example: `  # List all sub-accounts
  ahasend subaccounts list

  # List sub-accounts with JSON output
  ahasend subaccounts list --output json

  # List sub-accounts with pagination
  ahasend subaccounts list --limit 10`,
		RunE:         runSubAccountsList,
		SilenceUsage: true,
	}

	cmd.Flags().Int32("limit", 0, "Maximum number of sub-accounts to return")
	cmd.Flags().String("cursor", "", "Pagination cursor for next page")

	return cmd
}

func runSubAccountsList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// Get flags
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")

	// Validate before auth
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
		"limit":  limit,
		"cursor": cursor,
	}).Debug("Executing subaccounts list command")

	// Fetch sub-accounts
	response, err := client.ListSubAccounts(limitPtr, cursorPtr)
	if err != nil {
		return err
	}

	// Render via the sub-account list renderer. The renderer owns empty-state
	// handling per format: human formats print EmptyMessage, while JSON passes
	// the SDK PaginatedSubAccountsResponse through verbatim so an empty page
	// still preserves its pagination metadata instead of CLI wrapper fields.
	config := printer.ListConfig{
		SuccessMessage: "Sub-accounts retrieved successfully",
		EmptyMessage:   "No sub-accounts found",
		ShowPagination: true,
		FieldOrder:     []string{"id", "name", "status", "domain_count", "member_count", "monthly_credit", "created_at"},
	}

	return handler.HandleSubAccountList(response, config)
}
