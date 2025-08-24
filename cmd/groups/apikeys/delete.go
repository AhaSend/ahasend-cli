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
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates the apikeys delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <key-id>",
		Short: "Delete an API key",
		Long: `Delete an API key permanently.

⚠️  WARNING: This action is irreversible. Once an API key is deleted:
- The key can no longer be used for authentication
- Any applications using this key will lose access immediately
- The key cannot be recovered or restored

Before deleting an API key, ensure:
- No applications or scripts are actively using it
- You have alternative authentication methods configured
- You have documented any systems that might be affected

Use the --force flag to skip the confirmation prompt for automation.`,
		Example: `  # Delete an API key (with confirmation)
  ahasend apikeys delete fcb3f3bc-4ac8-4330-948d-1671fcf9a768

  # Force delete without confirmation (for automation)
  ahasend apikeys delete fcb3f3bc-4ac8-4330-948d-1671fcf9a768 --force

  # JSON output for automation
  ahasend apikeys delete fcb3f3bc-4ac8-4330-948d-1671fcf9a768 --force --output json`,
		Args: cobra.ExactArgs(1),
		RunE: runAPIKeyDelete,
	}

	// Delete flags
	cmd.Flags().Bool("force", false, "Skip confirmation prompt")

	return cmd
}

func runAPIKeyDelete(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	keyID := args[0]

	// Validate keyID is a valid UUID
	if _, err := uuid.Parse(keyID); err != nil {
		return errors.NewValidationError(fmt.Sprintf("invalid API key ID format: %s", keyID), err)
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
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

	// Log the operation
	logger.Get().WithFields(map[string]interface{}{
		"key_id": keyID,
		"force":  force,
	}).Debug("Deleting API key")

	// Delete the API key
	_, err = client.DeleteAPIKey(keyID)
	if err != nil {
		return err
	}

	// Handle successful deletion
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
