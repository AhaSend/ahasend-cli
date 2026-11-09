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

// NewSegmentsCommand creates the command that lists the segments a contact
// is a member of.
func NewSegmentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "segments <id-or-email>",
		Short: "List the segments a contact is a member of",
		Long: `List the segments one contact is a member of, by contact ID or raw email,
in the order of "segments list". Pass email addresses exactly as written. A
segment that is building may still list the contact by its previous rules.
It needs the contacts:read scope as well as segments:read.`,
		Example: `  # List the segments a contact is a member of
  ahasend contacts segments person@example.com

  # List them as CSV
  ahasend contacts segments person@example.com --output csv`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runContactsSegments,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	return cmd
}

func runContactsSegments(cmd *cobra.Command, args []string) error {
	idOrEmail := args[0]
	if err := validation.ValidatePathSegment("contact ID or email", idOrEmail); err != nil {
		return err
	}
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}

	params := requests.GetContactSegmentsParams{Limit: limit, After: after, Before: before}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"contact": idOrEmail,
	}).Debug("Executing contacts segments command")

	response, err := apiClient.GetContactSegments(idOrEmail, params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSegmentList(response, printer.ListConfig{
		SuccessMessage: "Contact segments retrieved successfully",
		EmptyMessage:   "No segments found",
	})
}
