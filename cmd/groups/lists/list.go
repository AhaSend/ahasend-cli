package lists

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewListCommand creates the lists list command.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contact lists",
		Long: `List the account's contact lists, newest first, with cursor pagination.

--name lists the lists whose name contains its value, ignoring case, so
--name news finds "Newsletter" and "Weekly News".`,
		Example: `  # List contact lists
  ahasend lists list

  # Filter lists by name
  ahasend lists list --name Newsletter --limit 20

  # Fetch the next page as JSON
  ahasend lists list --after next-page-token --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runListsList,
		SilenceUsage: true,
	}

	cmdutil.AddPaginationFlags(cmd)
	cmd.Flags().String("name", "", "Filter by list name: lists whose name contains it, in any case")
	return cmd
}

func runListsList(cmd *cobra.Command, _ []string) error {
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}

	params := requests.GetListsParams{
		Name:   cmdutil.NonEmptyString(cmd, "name"),
		Limit:  limit,
		After:  after,
		Before: before,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Debug("Executing lists list command")

	response, err := apiClient.GetLists(params)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleListsList(response, printer.ListConfig{
		SuccessMessage: "Lists retrieved successfully",
		EmptyMessage:   "No lists found",
	})
}
