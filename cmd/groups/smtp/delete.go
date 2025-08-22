package smtp

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates the smtp delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <credential-id>",
		Short: "Delete an SMTP credential",
		Long: `Delete an SMTP credential permanently.

This action cannot be undone. Any applications or services using this
credential will immediately lose access to send emails through SMTP.`,
		Example: `  # Delete with confirmation prompt
  ahasend smtp delete 550e8400-e29b-41d4-a716-446655440000

  # Delete without confirmation (for automation)
  ahasend smtp delete 550e8400-e29b-41d4-a716-446655440000 --force

  # Delete with JSON output
  ahasend smtp delete 550e8400-e29b-41d4-a716-446655440000 --force --output json`,
		Args: cobra.ExactArgs(1),
		RunE: runSMTPDelete,
	}

	// Add flags
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runSMTPDelete(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	credentialID := args[0]

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flag values
	force, _ := cmd.Flags().GetBool("force")

	// Get credential details for confirmation (unless forced)
	if !force {
		credential, err := client.GetSMTPCredential(credentialID)
		if err != nil {
			// If credential not found, still try to delete (might be stale)
			if !strings.Contains(err.Error(), "not found") {
				return handler.HandleError(err)
			}
		} else if credential != nil {
			// Show credential details and confirm deletion
			confirmed, err := confirmDeletion(credential.Name, credential.Username)
			if err != nil {
				return handler.HandleError(err)
			}
			if !confirmed {
				return handler.HandleSimpleSuccess("SMTP credential deletion cancelled")
			}
		}
	}

	logger.Get().WithFields(map[string]interface{}{
		"credential_id": credentialID,
		"force":         force,
	}).Info("Deleting SMTP credential")

	// Delete the credential
	err = client.DeleteSMTPCredential(credentialID)
	if err != nil {
		return handler.HandleError(err)
	}

	// Use the new ResponseHandler to display deletion success
	return handler.HandleDeleteSMTP(true, printer.DeleteConfig{
		SuccessMessage: fmt.Sprintf("SMTP credential %s deleted successfully", credentialID),
		ItemName:       "smtp_credential",
	})
}

func confirmDeletion(name, username string) (bool, error) {
	fmt.Println("You are about to delete the following SMTP credential:")
	fmt.Printf("  Name:     %s\n", name)
	fmt.Printf("  Username: %s\n", username)
	fmt.Println()
	fmt.Println("⚠️  This action cannot be undone!")
	fmt.Println("Any applications using this credential will lose SMTP access.")
	fmt.Print("\nDo you want to delete this SMTP credential? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}
