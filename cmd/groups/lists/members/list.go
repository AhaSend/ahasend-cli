package members

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the lists members list command.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <list-id>",
		Short: "List the members of a list",
		Long: `List a contact list's members, newest first, with cursor pagination.
--include-contacts embeds each whole contact, which also needs the
contacts:read scope.`,
		Example: `  # List the members of a list
  ahasend lists members list 123e4567-e89b-12d3-a456-426614174000

  # List confirmed members only
  ahasend lists members list 123e4567-e89b-12d3-a456-426614174000 --subscription-status confirmed

  # List members with their contacts
  ahasend lists members list 123e4567-e89b-12d3-a456-426614174000 --include-contacts`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runMembersList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	cmdutil.AddOneOfFlag(cmd, "subscription-status", "", "Filter by membership status: %s", cmdutil.SubscriptionStatusFilters)
	cmd.Flags().String("email", "", "Filter by normalized exact email address")
	cmd.Flags().Bool("include-contacts", false, "Embed each member's contact")
	return cmd
}

func runMembersList(cmd *cobra.Command, args []string) error {
	listID := args[0]
	listUUID, err := validation.ParseUUIDField("list ID", listID)
	if err != nil {
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

	email := cmdutil.NonEmptyString(cmd, "email")
	if err := validation.ValidateOptionalEmail("--email", email); err != nil {
		return err
	}

	params := requests.GetListContactsParams{
		Limit:              limit,
		After:              after,
		Before:             before,
		IncludeContacts:    cmdutil.ChangedBool(cmd, "include-contacts"),
		SubscriptionStatus: subscriptionStatus,
		Email:              email,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"list_id": listID,
	}).Debug("Executing lists members list command")

	response, err := apiClient.GetListContacts(listUUID, params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleListMemberList(response, printer.ListConfig{
		SuccessMessage: "List members retrieved successfully",
		EmptyMessage:   "No members found",
		ItemName:       "members",
	})
}
