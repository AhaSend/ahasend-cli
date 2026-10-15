package subaccounts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sub-accounts",
		Long: `List all sub-accounts under your AhaSend parent account with their status,
domain and member counts, and monthly credit.

The list can be paginated for large numbers of sub-accounts.`,
		Example: `  # List all sub-accounts
  ahasend subaccounts list

  # List sub-accounts with JSON output
  ahasend subaccounts list --output json

  # List sub-accounts with pagination
  ahasend subaccounts list --limit 10`,
		Args:         cmdutil.NoArgs,
		RunE:         runSubAccountsList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)

	return cmd
}

func runSubAccountsList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// Validate before auth
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
	logger.Get().Debug("Executing subaccounts list command")

	// Fetch sub-accounts
	response, err := client.ListSubAccounts(pagination)
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
	}

	return handler.HandleSubAccountList(response, config)
}
