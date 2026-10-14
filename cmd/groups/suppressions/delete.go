package suppressions

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

// NewDeleteCommand creates the suppressions delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <email>",
		Short: "Delete the suppressions of an email address",
		Long: `Delete the suppressions of an email address so it can receive emails again.

An address has a suppression per domain. With --domain, only the suppression
for that domain is deleted. Without it, the address's suppressions on every
domain are deleted, protected ones (unsubscribes and complaints) included.

WARNING: Deleting suppressions may result in sending emails to addresses
that previously bounced, complained, or unsubscribed.

Delete asks for confirmation unless --force is set.`,
		Example: `  # Delete the suppressions of an address on every domain (with confirmation)
  ahasend suppressions delete user@example.com

  # Delete domain-specific suppression
  ahasend suppressions delete user@example.com --domain mydomain.com

  # Delete without confirmation (for automation)
  ahasend suppressions delete user@example.com --force

  # Delete with JSON output
  ahasend suppressions delete user@example.com --force --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSuppressionsDelete,
		SilenceUsage: true,
	}

	// Add flags
	cmd.Flags().String("domain", "", "Delete only the suppression for this domain (default: every domain)")
	prompt.AddForceFlag(cmd)

	return cmd
}

func runSuppressionsDelete(cmd *cobra.Command, args []string) error {
	email := args[0]
	if err := validation.ValidateEmail("email", email); err != nil {
		return err
	}
	domain, err := cmdutil.ReadNonEmpty(cmd, "domain", "")
	if err != nil {
		return err
	}
	if err := validation.ValidateOptionalDomainName("--domain", domain); err != nil {
		return err
	}

	question := fmt.Sprintf("Are you sure you want to delete every suppression of %s, on every domain, "+
		"including unsubscribes and complaints?", email)
	if domain != nil {
		question = fmt.Sprintf("Are you sure you want to delete the suppression of %s for domain %s?", email, *domain)
	}
	if err := prompt.Confirm(cmd, question); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"email":  email,
		"domain": cmd.Flag("domain").Value.String(),
	}).Debug("Removing suppression")

	response, err := client.DeleteSuppression(email, domain)
	if err != nil {
		return err
	}

	successMsg := fmt.Sprintf("Suppression for '%s' deleted successfully", email)
	if domain != nil {
		successMsg += fmt.Sprintf(" (domain '%s')", *domain)
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: successMsg,
	})
}
