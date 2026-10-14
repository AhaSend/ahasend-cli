package suppressions

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCheckCommand creates the suppressions check command
func NewCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <email>",
		Short: "Check if an email address is suppressed",
		Long: `Check if an email address is suppressed and cannot receive emails.

An address has a suppression per domain. Without --domain, the check shows
every suppression of the address; with --domain, only the one for that
domain.`,
		Example: `  # Check the suppressions of an address on every domain
  ahasend suppressions check user@example.com

  # Check if email is suppressed for specific domain
  ahasend suppressions check user@example.com --domain mydomain.com

  # Check with JSON output for automation
  ahasend suppressions check user@example.com --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSuppressionsCheck,
		SilenceUsage: true,
	}

	// Add flags
	cmd.Flags().String("domain", "", "Check suppression for specific domain only")

	return cmd
}

func runSuppressionsCheck(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

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

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"email":  email,
		"domain": cmd.Flag("domain").Value.String(),
	}).Debug("Checking suppression status")

	// Check suppression
	suppressions, err := client.ListSuppressions(requests.GetSuppressionsParams{
		Email:  &email,
		Domain: domain,
	})
	if err != nil {
		return err
	}

	return handler.HandleCheckSuppression(suppressions, printer.CheckConfig{
		FoundMessage:    fmt.Sprintf("Email %s is suppressed", email),
		NotFoundMessage: fmt.Sprintf("Email %s is not suppressed", email),
	})
}
