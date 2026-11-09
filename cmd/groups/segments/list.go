package segments

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the segments list command.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List segments",
		Long:  `List the account's segments, newest first, with cursor pagination.`,
		Example: `  # List segments
  ahasend segments list

  # Fetch the next page as JSON
  ahasend segments list --after next-page-token --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runSegmentsList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	return cmd
}

func runSegmentsList(cmd *cobra.Command, _ []string) error {
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}

	params := requests.GetSegmentsParams{Limit: limit, After: after, Before: before}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Debug("Executing segments list command")

	response, err := apiClient.GetSegments(params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSegmentList(response, printer.ListConfig{
		SuccessMessage: "Segments retrieved successfully",
		EmptyMessage:   "No segments found",
	})
}
