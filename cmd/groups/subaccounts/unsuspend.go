package subaccounts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewUnsuspendCommand creates the unsuspend command
func NewUnsuspendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unsuspend <sub-account-id>",
		Aliases: []string{"resume", "unpause"},
		Short:   "Unsuspend a sub-account",
		Long: `Unsuspend a previously suspended sub-account under your AhaSend parent
account.

The sub-account is restored to an active state and can send email again.
"resume" and "unpause" are aliases of this command. A suspension that AhaSend
started can only be lifted by AhaSend support.`,
		Example: `  # Unsuspend a sub-account
  ahasend subaccounts unsuspend 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
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
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
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

	response, err := client.UnsuspendSubAccount(subAccountUUID)
	if err != nil {
		return err
	}

	config := printer.SingleConfig{
		SuccessMessage: "Sub-account unsuspended successfully",
		EmptyMessage:   "Sub-account not found",
	}

	return handler.HandleSingleSubAccount(response, config)
}
