package templates

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the campaigns templates list command.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List campaign templates",
		Long: `List the account's campaign templates, without their content, with cursor
pagination.`,
		Example: `  # List campaign templates
  ahasend campaigns templates list

  # Fetch the next page
  ahasend campaigns templates list --limit 20 --after next-page-token`,
		Args:         cmdutil.NoArgs,
		RunE:         runCampaignTemplatesList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	return cmd
}

func runCampaignTemplatesList(cmd *cobra.Command, _ []string) error {
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}

	params := requests.ListCampaignTemplatesParams{
		Limit:  limit,
		After:  after,
		Before: before,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Debug("Executing campaigns templates list command")

	response, err := apiClient.ListCampaignTemplates(params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleCampaignTemplateList(response, printer.ListConfig{
		SuccessMessage: "Campaign templates retrieved successfully",
		EmptyMessage:   "No campaign templates found",
	})
}
