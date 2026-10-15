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

// NewMembershipsCommand creates the command that lists a contact's list
// memberships.
func NewMembershipsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memberships <id-or-email>",
		Short: "List the lists a contact belongs to",
		Long: `List one contact's list memberships by contact ID or raw email, each with
its list. Pass email addresses exactly as written.`,
		Example: `  # List the lists a contact belongs to
  ahasend contacts memberships person@example.com

  # List the lists a contact unsubscribed from
  ahasend contacts memberships person@example.com --subscription-status unsubscribed`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runContactsMemberships,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddOneOfFlag(cmd, "subscription-status", "", "Filter by membership status: %s", cmdutil.SubscriptionStatusFilters)
	return cmd
}

func runContactsMemberships(cmd *cobra.Command, args []string) error {
	idOrEmail := args[0]
	if err := validation.ValidatePathSegment("contact ID or email", idOrEmail); err != nil {
		return err
	}
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}
	subscriptionStatus, err := cmdutil.ReadFilterOneOf(cmd, "subscription-status")
	if err != nil {
		return err
	}

	params := requests.GetContactListsParams{
		Limit:              limit,
		After:              after,
		Before:             before,
		SubscriptionStatus: subscriptionStatus,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"contact": idOrEmail,
	}).Debug("Executing contacts memberships command")

	response, err := apiClient.GetContactLists(idOrEmail, params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleContactMembershipList(response, printer.ListConfig{
		SuccessMessage: "Contact lists retrieved successfully",
		EmptyMessage:   "No lists found",
		ItemName:       "lists",
	})
}
