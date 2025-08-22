package domains

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

// NewDeleteCommand creates the delete command
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <domain>",
		Short: "Delete a domain",
		Long: `Delete a domain from your AhaSend account. This action cannot be undone.

⚠️  WARNING: Deleting a domain will:
• Remove the domain from your account permanently
• Stop all email sending from this domain
• Remove all DNS verification records
• Cannot be undone

Make sure you really want to delete the domain before confirming.`,
		Example: `  # Delete a domain (with confirmation prompt)
  ahasend domains delete example.com

  # Force delete without confirmation prompt
  ahasend domains delete example.com --force

  # Delete with explicit confirmation
  echo "yes" | ahasend domains delete example.com`,
		Args:         cobra.ExactArgs(1),
		RunE:         runDomainsDelete,
		SilenceUsage: true,
	}

	cmd.Flags().Bool("force", false, "Skip confirmation prompt (use with caution)")

	return cmd
}

func runDomainsDelete(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	domain := args[0]
	force, _ := cmd.Flags().GetBool("force")

	logger.Get().WithFields(map[string]interface{}{
		"domain": domain,
		"force":  force,
	}).Debug("Executing domain delete command")

	// Get domain details first to show what's being deleted and verify it exists
	domainInfo, err := client.GetDomain(domain)
	if err != nil {
		return handler.HandleError(err)
	}

	if domainInfo == nil {
		return handler.HandleError(errors.NewNotFoundError(fmt.Sprintf("domain '%s' not found", domain), nil))
	}

	// Confirmation prompt unless force flag is used
	// Note: Interactive prompts should ideally be handled by the printer,
	// but for now keeping here to maintain existing behavior
	if !force {
		logger.Get().WithField("domain", domain).Debug("Prompting for confirmation")

		confirmed, err := promptConfirmation(domain)
		if err != nil {
			return handler.HandleError(errors.NewValidationError("failed to read confirmation", err))
		}

		if !confirmed {
			logger.Get().WithField("domain", domain).Debug("Domain deletion cancelled by user")
			return handler.HandleSimpleSuccess("Domain deletion cancelled")
		}
		logger.Get().WithField("domain", domain).Debug("Deletion confirmed by user")
	} else {
		logger.Get().WithField("domain", domain).Debug("Skipping confirmation due to force flag")
	}

	// Delete the domain
	_, err = client.DeleteDomain(domain)
	if err != nil {
		return handler.HandleError(err)
	}

	// Handle successful deletion
	// TODO: Should add HandleDeleteDomain method for consistency with other delete operations
	return handler.HandleSimpleSuccess(fmt.Sprintf("✅ Domain '%s' has been deleted successfully", domain))
}

func promptConfirmation(domain string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("Are you sure you want to delete domain '%s'? This cannot be undone. (yes/no): ", domain)

		response, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		response = strings.ToLower(strings.TrimSpace(response))

		switch response {
		case "yes", "y":
			return true, nil
		case "no", "n":
			return false, nil
		default:
			fmt.Println("Please answer 'yes' or 'no'")
			continue
		}
	}
}
