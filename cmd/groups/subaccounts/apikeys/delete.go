package apikeys

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

// NewDeleteCommand creates the `subaccounts api-keys delete` command.
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <sub-account-id> <key-id>",
		Short: "Delete a sub-account API key",
		Long: `Delete an API key that belongs to a sub-account permanently.

⚠️  WARNING: This action is irreversible. Once the API key is deleted:
- The key can no longer be used for authentication
- Any applications using this key will lose access immediately
- The key cannot be recovered or restored

Use the --force flag to skip the confirmation prompt for automation.`,
		Example: `  # Delete a sub-account API key (with confirmation)
  ahasend subaccounts api-keys delete 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000

  # Force delete without confirmation (for automation)
  ahasend subaccounts api-keys delete 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cobra.ExactArgs(2),
		RunE:         runSubAccountAPIKeyDelete,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("force", false, "Skip confirmation prompt")

	return cmd
}

func runSubAccountAPIKeyDelete(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]
	keyID := args[1]

	// Validate before auth and before confirmation.
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
		return err
	}
	if err := validation.ValidateUUIDField("API key ID", keyID); err != nil {
		return err
	}

	// Get flag values
	force, _ := cmd.Flags().GetBool("force")

	// If not force mode, show confirmation
	if !force {
		if err := confirmDeletion(keyID); err != nil {
			return err
		}
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"key_id":         keyID,
		"force":          force,
	}).Debug("Executing subaccounts api-keys delete command")

	if _, err := client.DeleteSubAccountAPIKey(subAccountID, keyID); err != nil {
		return err
	}

	// Reuse the shared deletion renderer for output parity.
	return handler.HandleDeleteAPIKey(true, printer.DeleteConfig{
		SuccessMessage: "✅ API Key Deleted Successfully",
		ItemName:       "API key",
	})
}

func confirmDeletion(keyID string) error {
	fmt.Printf("⚠️  You are about to permanently delete API key: %s\n", keyID)
	fmt.Println("This action cannot be undone and will immediately revoke access for any applications using this key.")
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
