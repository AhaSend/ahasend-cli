package segments

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

// NewDeleteCommand creates the segments delete command.
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <segment-id>",
		Short: "Delete a segment",
		Long: `Delete a segment; its members are removed shortly after. The API refuses
while a campaign that still needs the segment, to send to its members or to
leave them out, refers to it. Delete asks for confirmation unless --force is
set.`,
		Example: `  # Delete a segment
  ahasend segments delete 123e4567-e89b-12d3-a456-426614174000

  # Delete a segment without confirmation
  ahasend segments delete 123e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSegmentsDelete,
		SilenceUsage: true,
	}
	prompt.AddForceFlag(cmd)
	return cmd
}

func runSegmentsDelete(cmd *cobra.Command, args []string) error {
	segmentID := args[0]
	segmentUUID, err := validation.ParseUUIDField("segment ID", segmentID)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete segment %s?", segmentID)); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"segment_id": segmentID,
	}).Debug("Executing segments delete command")

	response, err := apiClient.DeleteSegment(segmentUUID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Segment '%s' deleted successfully", segmentID),
	})
}
