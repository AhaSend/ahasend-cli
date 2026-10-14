package subaccounts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <sub-account-id>",
		Short: "Get a sub-account",
		Long: `Get detailed information about a specific sub-account including its status,
parent account, monthly credit, and domain and member counts.`,
		Example: `  # Get sub-account details
  ahasend subaccounts get 123e4567-e89b-12d3-a456-426614174000

  # Get sub-account details with JSON output
  ahasend subaccounts get 123e4567-e89b-12d3-a456-426614174000 --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSubAccountsGet,
		SilenceUsage: true,
	}

	return cmd
}

func runSubAccountsGet(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Validate before auth
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
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
	}).Debug("Executing subaccounts get command")

	// Get sub-account details
	response, err := client.GetSubAccount(subAccountUUID)
	if err != nil {
		return err
	}

	// Handle successful sub-account response
	config := printer.SingleConfig{
		SuccessMessage: "Sub-account retrieved successfully",
		EmptyMessage:   "Sub-account not found",
	}

	return handler.HandleSingleSubAccount(response, config)
}
