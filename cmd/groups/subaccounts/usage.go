package subaccounts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewUsageCommand creates the usage command
func NewUsageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show usage allocation across sub-accounts",
		Long: `Show usage allocation for the current billing period across the parent
account and its sub-accounts, including reception counts and allocated cost.`,
		Example: `  # Show sub-account usage allocation
  ahasend subaccounts usage

  # Show sub-account usage with JSON output
  ahasend subaccounts usage --output json`,
		Args:         cobra.NoArgs,
		RunE:         runSubAccountsUsage,
		SilenceUsage: true,
	}

	return cmd
}

func runSubAccountsUsage(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// No local input to validate; authenticate before fetching usage
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().Debug("Executing subaccounts usage command")

	// Fetch usage allocation
	response, err := client.GetSubAccountsUsage()
	if err != nil {
		return err
	}

	if response == nil {
		return handler.HandleEmpty("No sub-account usage data found")
	}

	// Handle successful usage response
	config := printer.SingleConfig{
		SuccessMessage: "Sub-account usage retrieved successfully",
		EmptyMessage:   "No sub-account usage data found",
	}

	return handler.HandleSubAccountUsage(response, config)
}
