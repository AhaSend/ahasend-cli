package subaccounts

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates the delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <sub-account-id>",
		Short: "Delete a sub-account",
		Long: `Delete a sub-account under your AhaSend parent account.

This is a soft delete: the sub-account is deactivated and can no longer be used,
but its historical data is retained by AhaSend.

Use the --force flag to skip the confirmation prompt for automation.`,
		Example: `  # Delete a sub-account (with confirmation)
  ahasend subaccounts delete 123e4567-e89b-12d3-a456-426614174000

  # Force delete without confirmation (for automation)
  ahasend subaccounts delete 123e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cobra.ExactArgs(1),
		RunE:         runSubAccountsDelete,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("force", false, "Skip confirmation prompt")

	return cmd
}

func runSubAccountsDelete(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Validate the ID before anything else: before any confirmation prompt and
	// before auth, so malformed input cannot be masked by a prompt or an auth
	// failure.
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")

	// Confirm only after the ID is known to be well-formed.
	if !force {
		if err := confirmSubAccountDeletion(subAccountID); err != nil {
			return err
		}
	}

	// Only authenticate after local validation (and confirmation) pass.
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"force":          force,
	}).Debug("Executing subaccounts delete command")

	if _, err := client.DeleteSubAccount(subAccountID); err != nil {
		return err
	}

	return handler.HandleSimpleSuccess(fmt.Sprintf("Sub-account '%s' deleted successfully", subAccountID))
}

func confirmSubAccountDeletion(subAccountID string) error {
	fmt.Printf("⚠️  You are about to delete sub-account: %s\n", subAccountID)
	fmt.Println("This deactivates the sub-account so it can no longer be used.")
	fmt.Print("Are you sure you want to continue? (y/N): ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if response == "y" || response == "yes" {
			return nil
		}
	}

	return errors.NewValidationError("operation cancelled", nil)
}
