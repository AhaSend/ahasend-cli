package contacts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the contact list command.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contacts",
		Long: `List the account's contacts, newest first, with cursor pagination. Filter by
exact email, status, subscription and creation time.

Times are in RFC3339 format (e.g., 2026-01-15T10:30:00Z) or relative, such as
"24h" for 24 hours ago or "7d" for 7 days ago.`,
		Example: `  # List enabled, subscribed contacts
  ahasend contacts list --status enabled --subscribed=true

  # List contacts created in the last 7 days
  ahasend contacts list --from-time 7d --limit 50

  # Fetch the next page as JSON
  ahasend contacts list --after next-page-token --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runContactsList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	cmd.Flags().String("email", "", "Filter by normalized exact email address")
	cmdutil.AddOneOfFlag(cmd, "status", "", "Filter by status: %s", contactStatuses)
	cmd.Flags().Bool("subscribed", false, "Filter by global subscription state")
	cmd.Flags().String("from-time", "", "Filter contacts created after this time (RFC3339 or relative like '24h', '7d')")
	cmd.Flags().String("to-time", "", "Filter contacts created before this time (RFC3339 or relative)")
	return cmd
}

func runContactsList(cmd *cobra.Command, _ []string) error {
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}
	fromTime, toTime, err := cmdutil.ReadPastTimeRange(cmd)
	if err != nil {
		return err
	}
	status, err := cmdutil.ReadFilterOneOf(cmd, "status")
	if err != nil {
		return err
	}

	email := cmdutil.NonEmptyString(cmd, "email")
	if err := validation.ValidateOptionalEmail("--email", email); err != nil {
		return err
	}

	params := requests.GetContactsParams{
		Limit:      limit,
		After:      after,
		Before:     before,
		Email:      email,
		Status:     status,
		Subscribed: cmdutil.ChangedBool(cmd, "subscribed"),
		FromTime:   fromTime,
		ToTime:     toTime,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Debug("Executing contacts list command")

	response, err := apiClient.GetContacts(params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleContactList(response, printer.ListConfig{
		SuccessMessage: "Contacts retrieved successfully",
		EmptyMessage:   "No contacts found",
	})
}
