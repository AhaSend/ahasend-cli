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

// NewUnsuspendCommand creates the unsuspend command
func NewUnsuspendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unsuspend <sub-account-id>",
		Short: "Unsuspend a sub-account",
		Long: `Unsuspend a previously suspended sub-account under your AhaSend parent account.

The sub-account is restored to an active state and can send email again.`,
		Example: `  # Unsuspend a sub-account
  ahasend subaccounts unsuspend 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cobra.ExactArgs(1),
		RunE:         runSubAccountsUnsuspend,
		SilenceUsage: true,
	}

	return cmd
}

func runSubAccountsUnsuspend(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Validate the ID before anything else, before auth.
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
		return err
	}

	// Only authenticate after local validation passes.
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
	}).Debug("Executing subaccounts unsuspend command")

	response, err := client.UnsuspendSubAccount(subAccountID)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewNotFoundError(fmt.Sprintf("sub-account '%s' not found", subAccountID), nil)
	}

	config := printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Sub-account '%s' unsuspended successfully", subAccountID),
		EmptyMessage:   "Sub-account not found",
		FieldOrder:     []string{"name", "id", "parent_account_id", "status", "website", "monthly_credit", "domain_count", "member_count", "created_at", "last_activity_at"},
	}

	return handler.HandleSingleSubAccount(response, config)
}
