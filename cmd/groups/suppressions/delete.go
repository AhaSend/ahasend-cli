package suppressions

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates the suppressions delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <email>",
		Short: "Delete an email address from the suppression list",
		Long: `Delete an email address from the suppression list to allow sending emails.

This command removes a suppression entry for the specified email address.
Use --domain to delete only domain-specific suppressions.
Without --domain, deletes global suppressions.

⚠️  WARNING: Deleting suppressions may result in sending emails to addresses
that previously bounced, complained, or unsubscribed. Use with caution.

Use --force flag for automation and CI/CD pipelines.`,
		Example: `  # Delete global suppression (with confirmation)
  ahasend suppressions delete user@example.com

  # Delete domain-specific suppression
  ahasend suppressions delete user@example.com --domain mydomain.com

  # Delete without confirmation (for automation)
  ahasend suppressions delete user@example.com --force

  # Delete with JSON output
  ahasend suppressions delete user@example.com --output json`,
		Args: cobra.ExactArgs(1),
		RunE: runSuppressionsDelete,
	}

	// Add flags
	cmd.Flags().String("domain", "", "Domain for domain-specific suppression removal (optional)")
	cmd.Flags().Bool("force", false, "Skip confirmation prompt")

	return cmd
}

func runSuppressionsDelete(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	email := args[0]

	// Validate email format (basic validation)
	if email == "" {
		return errors.NewValidationError("email address is required", nil)
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flag values
	domain, _ := cmd.Flags().GetString("domain")
	force, _ := cmd.Flags().GetBool("force")

	var domainPtr *string
	if domain != "" {
		domainPtr = &domain
	}

	// Show suppression details and confirm removal (unless --force is used)
	if !force {
		confirmed, err := confirmRemoval(email, domain)
		if err != nil {
			return err
		}
		if !confirmed {
			return handler.HandleSimpleSuccess("Suppression removal cancelled")
		}
	}

	logger.Get().WithFields(map[string]interface{}{
		"email":  email,
		"domain": domain,
		"force":  force,
	}).Debug("Removing suppression")

	// Remove suppression
	_, err = client.DeleteSuppression(email, domainPtr)
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display deletion success
	successMsg := fmt.Sprintf("Suppression removed successfully for %s", email)
	if domain != "" {
		successMsg += fmt.Sprintf(" (domain: %s)", domain)
	}
	return handler.HandleDeleteSuppression(true, printer.DeleteConfig{
		SuccessMessage: successMsg,
		ItemName:       "suppression",
	})
}

func confirmRemoval(email, domain string) (bool, error) {
	fmt.Printf("Found suppression for %s", email)
	if domain != "" {
		fmt.Printf(" (domain: %s)", domain)
	}
	fmt.Println(":")

	fmt.Println("\n⚠️  WARNING: Removing this suppression will allow emails to be sent to this address again.")
	fmt.Print("Are you sure you want to remove this suppression? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}
