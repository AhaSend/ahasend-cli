package templates

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the templates list command.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transactional templates",
		Long: `List the account's transactional templates, newest first, with cursor
pagination.`,
		Example: `  # List templates
  ahasend templates list

  # List up to 20 templates
  ahasend templates list --limit 20

  # Fetch the next page as JSON
  ahasend templates list --after next-page-token --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runTemplatesList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	return cmd
}

func runTemplatesList(cmd *cobra.Command, _ []string) error {
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}

	params := requests.GetTemplatesParams{
		Limit:  limit,
		After:  after,
		Before: before,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Debug("Executing templates list command")

	response, err := apiClient.GetTemplates(params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleTemplateList(response, printer.ListConfig{
		SuccessMessage: "Templates retrieved successfully",
		EmptyMessage:   "No templates found",
	})
}
