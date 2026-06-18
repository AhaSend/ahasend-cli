package subaccounts

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <sub-account-id>",
		Short: "Get detailed information about a sub-account",
		Long: `Get detailed information about a specific sub-account including its status,
parent account, monthly credit, and domain and member counts.`,
		Example: `  # Get sub-account details
  ahasend subaccounts get 123e4567-e89b-12d3-a456-426614174000

  # Get sub-account details with JSON output
  ahasend subaccounts get 123e4567-e89b-12d3-a456-426614174000 --output json`,
		Args:         cobra.ExactArgs(1),
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
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
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
	response, err := client.GetSubAccount(subAccountID)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewNotFoundError(fmt.Sprintf("sub-account '%s' not found", subAccountID), nil)
	}

	// Handle successful sub-account response
	config := printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Sub-account details for '%s'", subAccountID),
		EmptyMessage:   "Sub-account not found",
		FieldOrder:     []string{"name", "id", "parent_account_id", "status", "website", "monthly_credit", "domain_count", "member_count", "created_at", "last_activity_at"},
	}

	return handler.HandleSingleSubAccount(response, config)
}
