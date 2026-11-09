package segments

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the segments get command.
func NewGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <segment-id>",
		Short: "Get a segment",
		Long: `Get one segment, with its state and its definition. A failed segment shows
the reason it failed.

Members counts every contact that matches the rules, whatever its consent.
Marketable counts those a campaign to the segment alone would reach; it is
unknown while the segment is not ready, and when the API cannot count the
members in time.`,
		Example: `  # Get a segment
  ahasend segments get 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSegmentsGet,
		SilenceUsage: true,
	}
}

func runSegmentsGet(cmd *cobra.Command, args []string) error {
	segmentID, err := validation.ParseUUIDField("segment ID", args[0])
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"segment_id": args[0],
	}).Debug("Executing segments get command")

	segment, err := apiClient.GetSegment(segmentID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSegmentDetail(segment, printer.SingleConfig{
		SuccessMessage: "Segment retrieved successfully",
		EmptyMessage:   "Segment not found",
	})
}
