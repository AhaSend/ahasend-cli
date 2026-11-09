package segments

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewMembersCommand creates the segments members command.
func NewMembersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members <segment-id>",
		Short: "List the members of a segment",
		Long: `List a segment's members by contact ID, from the lowest, with cursor
pagination. A member is a contact that matches the rules, whatever its
consent. While the segment is building, its members may still follow its
previous rules. Listing members needs the contacts:read scope as well as
segments:read.`,
		Example: `  # List the members of a segment
  ahasend segments members 123e4567-e89b-12d3-a456-426614174000

  # List 50 members as CSV
  ahasend segments members 123e4567-e89b-12d3-a456-426614174000 --limit 50 --output csv`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSegmentsMembers,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	return cmd
}

func runSegmentsMembers(cmd *cobra.Command, args []string) error {
	segmentID, err := validation.ParseUUIDField("segment ID", args[0])
	if err != nil {
		return err
	}
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}

	params := requests.GetSegmentContactsParams{Limit: limit, After: after, Before: before}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"segment_id": args[0],
	}).Debug("Executing segments members command")

	response, err := apiClient.GetSegmentContacts(segmentID, params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSegmentMemberList(response, printer.ListConfig{
		SuccessMessage: "Segment members retrieved successfully",
		EmptyMessage:   "No members found",
	})
}
