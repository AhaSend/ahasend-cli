package domains

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
		Use:   "delete <domain>",
		Short: "Delete a domain",
		Long: `Delete a domain from your AhaSend account. This action cannot be undone.

WARNING: Deleting a domain will:
• Remove the domain from your account permanently
• Stop all email sending from this domain
• Cannot be undone

The DNS records you added for the domain stay with your DNS provider, which
the command cannot change; remove them there when you no longer need them.

Delete asks for confirmation unless --force is set.`,
		Example: `  # Delete a domain (with confirmation prompt)
  ahasend domains delete example.com

  # Force delete without confirmation prompt
  ahasend domains delete example.com --force

  # Delete with explicit confirmation
  echo "yes" | ahasend domains delete example.com`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runDomainsDelete,
		SilenceUsage: true,
	}

	prompt.AddForceFlag(cmd)

	return cmd
}

func runDomainsDelete(cmd *cobra.Command, args []string) error {
	domain := args[0]
	if err := validation.ValidatePathSegment("domain", domain); err != nil {
		return err
	}
	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete domain %s?", domain)); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"domain": domain,
	}).Debug("Executing domain delete command")

	response, err := client.DeleteDomain(domain)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Domain '%s' deleted successfully", domain),
	})
}
