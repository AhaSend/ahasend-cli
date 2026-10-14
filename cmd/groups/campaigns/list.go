package campaigns

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the campaigns list command.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List campaigns",
		Long: `List the account's campaigns, newest first, with cursor pagination. --status
lists only the campaigns in one status, such as draft, scheduled or sent.`,
		Example: `  # List campaigns
  ahasend campaigns list

  # List draft campaigns
  ahasend campaigns list --status draft

  # Fetch the next page as JSON
  ahasend campaigns list --limit 20 --after next-page-token --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runCampaignsList,
		SilenceUsage: true,
	}

	cmdutil.AddOneOfFlag(cmd, "status", "", "Filter by campaign status: %s", campaignStatuses)
	cmdutil.AddPaginationFlags(cmd)
	return cmd
}

// campaignStatuses are the values of --status.
var campaignStatuses = []string{"draft", "validating", "waiting_for_review", "scheduled", "sending", "paused", "sent", "canceled", "failed"}

func runCampaignsList(cmd *cobra.Command, _ []string) error {
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}
	status, err := cmdutil.ReadFilterOneOf(cmd, "status")
	if err != nil {
		return err
	}

	params := requests.ListCampaignsParams{
		Status: status,
		Limit:  limit,
		After:  after,
		Before: before,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"status": cmd.Flag("status").Value.String(),
	}).Debug("Executing campaigns list command")

	response, err := apiClient.ListCampaigns(params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleCampaignList(response, printer.ListConfig{
		SuccessMessage: "Campaigns retrieved successfully",
		EmptyMessage:   "No campaigns found",
	})
}
