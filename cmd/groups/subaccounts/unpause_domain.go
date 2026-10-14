package subaccounts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewUnpauseDomainCommand creates the unpause-domain command
func NewUnpauseDomainCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpause-domain <sub-account-id> <domain>",
		Short: "Unpause a sub-account's domain",
		Long: `Lift the pause on a domain of a sub-account, so that the domain can send again.

AhaSend pauses a domain, for example when too many of its recent emails
bounced. Unpausing a domain that is not paused changes nothing and returns the
domain. A change can take some minutes to apply to new email.`,
		Example: `  # Unpause a sub-account's domain
  ahasend subaccounts unpause-domain 123e4567-e89b-12d3-a456-426614174000 example.com`,
		Args:         cmdutil.ExactArgs(2),
		RunE:         runSubAccountsUnpauseDomain,
		SilenceUsage: true,
	}

	return cmd
}

func runSubAccountsUnpauseDomain(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]
	domain := args[1]

	// Validate the arguments before auth.
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
		return err
	}
	if err := validation.ValidatePathSegment("domain", domain); err != nil {
		return err
	}

	// Only authenticate after local validation passes.
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"domain":         domain,
	}).Debug("Executing subaccounts unpause-domain command")

	response, err := client.UnpauseSubAccountDomain(subAccountUUID, domain)
	if err != nil {
		return err
	}

	config := printer.SingleConfig{
		SuccessMessage: "Domain unpaused successfully",
		EmptyMessage:   "Domain not found",
	}

	return handler.HandleSingleDomain(response, config)
}
