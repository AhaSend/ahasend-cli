package suppressions

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the suppressions list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List suppressions",
		Long: `List suppressed email addresses, with cursor pagination. Filter by email
address, domain and creation time.`,
		Example: `  # List all suppressions
  ahasend suppressions list

  # Search for specific email suppression
  ahasend suppressions list --email user@example.com

  # Filter by domain
  ahasend suppressions list --domain example.com

  # List the suppressions created in the last 7 days
  ahasend suppressions list --from-time 7d

  # Export to JSON
  ahasend suppressions list --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runSuppressionsList,
		SilenceUsage: true,
	}

	// Add flags
	cmd.Flags().String("email", "", "Filter by email address")
	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddLegacyCursorAlias(cmd)
	cmd.Flags().String("domain", "", "Filter by specific domain")
	cmd.Flags().String("from-time", "", "Filter suppressions created after this time (RFC3339 or relative like '24h', '7d')")
	cmd.Flags().String("to-time", "", "Filter suppressions created before this time (RFC3339 or relative)")

	return cmd
}

func runSuppressionsList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	pagination, err := cmdutil.ReadPaginationParams(cmd)
	if err != nil {
		return err
	}
	email := cmdutil.NonEmptyString(cmd, "email")
	if err := validation.ValidateOptionalEmail("--email", email); err != nil {
		return err
	}
	domain, err := cmdutil.ReadNonEmpty(cmd, "domain", "")
	if err != nil {
		return err
	}
	if err := validation.ValidateOptionalDomainName("--domain", domain); err != nil {
		return err
	}
	fromTime, toTime, err := cmdutil.ReadPastTimeRange(cmd)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"email":  email,
		"domain": cmd.Flag("domain").Value.String(),
	}).Debug("Executing suppressions list command")

	// Fetch suppressions
	response, err := client.ListSuppressions(requests.GetSuppressionsParams{
		Email:            email,
		Domain:           domain,
		FromTime:         fromTime,
		ToTime:           toTime,
		PaginationParams: pagination,
	})
	if err != nil {
		return err
	}

	// Use the new ResponseHandler to display suppressions list
	emptyMessage := "No suppressions found"
	if email != nil {
		emptyMessage = "No suppressions found for the specified email address"
	}

	return handler.HandleSuppressionList(response, printer.ListConfig{
		SuccessMessage: "Suppressions retrieved successfully",
		EmptyMessage:   emptyMessage,
	})
}
