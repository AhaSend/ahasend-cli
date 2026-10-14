package lists

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the lists get command.
func NewGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <list-id>",
		Short: "Get a contact list",
		Long:  `Get one contact list, including its contact count.`,
		Example: `  # Get a contact list
  ahasend lists get 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runListsGet,
		SilenceUsage: true,
	}
}

func runListsGet(cmd *cobra.Command, args []string) error {
	listID, err := validation.ParseUUIDField("list ID", args[0])
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"list_id": args[0],
	}).Debug("Executing lists get command")

	list, err := apiClient.GetList(listID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleList(list, printer.SingleConfig{
		SuccessMessage: "List retrieved successfully",
		EmptyMessage:   "List not found",
	})
}
