package subaccounts

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates the delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <sub-account-id>",
		Short: "Delete a sub-account",
		Long: `Delete a sub-account under your AhaSend parent account.

This is a soft delete: the sub-account is deactivated and can no longer be
used, but its historical data is retained by AhaSend.

Delete asks for confirmation unless --force is set.`,
		Example: `  # Delete a sub-account (with confirmation)
  ahasend subaccounts delete 123e4567-e89b-12d3-a456-426614174000

  # Force delete without confirmation (for automation)
  ahasend subaccounts delete 123e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSubAccountsDelete,
		SilenceUsage: true,
	}

	prompt.AddForceFlag(cmd)

	return cmd
}

func runSubAccountsDelete(cmd *cobra.Command, args []string) error {
	subAccountID := args[0]
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete sub-account %s?", subAccountID)); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
	}).Debug("Executing subaccounts delete command")

	response, err := client.DeleteSubAccount(subAccountUUID)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Sub-account '%s' deleted successfully", subAccountID),
	})
}
