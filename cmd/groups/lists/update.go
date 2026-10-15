package lists

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the lists update command.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <list-id>",
		Short: "Update a contact list",
		Long: `Partially update a contact list. Only the flags you set are sent.
--description "" clears the description. --tag replaces all tags with the
given ones, and --clear-tags removes every tag.`,
		Example: `  # Rename a list
  ahasend lists update 123e4567-e89b-12d3-a456-426614174000 --name "Weekly news"

  # Replace the tags of a list
  ahasend lists update 123e4567-e89b-12d3-a456-426614174000 --tag news --tag weekly

  # Remove every tag from a list
  ahasend lists update 123e4567-e89b-12d3-a456-426614174000 --clear-tags`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runListsUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New list name")
	cmd.Flags().String("description", "", "New list description (empty clears it)")
	cmdutil.AddRepeatableFlag(cmd, "tag", "New tag for the list, replacing all tags (can be used multiple times)")
	cmd.Flags().Bool("clear-tags", false, "Remove every tag from the list")
	return cmd
}

func runListsUpdate(cmd *cobra.Command, args []string) error {
	listID, err := validation.ParseUUIDField("list ID", args[0])
	if err != nil {
		return err
	}

	name, err := cmdutil.ReadNonEmpty(cmd, "name", "")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("name", name, cmdutil.MaxNameLength, strings.TrimSpace); err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("description", cmdutil.ChangedString(cmd, "description"), maxDescriptionLength, strings.TrimSpace); err != nil {
		return err
	}
	tags, err := cmdutil.ReadListUpdate(cmd, "tag", "clear-tags")
	if err != nil {
		return err
	}

	request := requests.UpdateListRequest{
		Name:        name,
		Description: cmdutil.ChangedString(cmd, "description"),
		Tags:        tags,
	}
	if request.Name == nil && request.Description == nil && request.Tags == nil {
		return errors.NewValidationError("at least one of --name, --description, --tag or --clear-tags must be provided", nil)
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"list_id": args[0],
	}).Debug("Executing lists update command")

	list, err := apiClient.UpdateList(listID, request)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleList(list, printer.SingleConfig{
		SuccessMessage: "List updated successfully",
		EmptyMessage:   "No list updated",
	})
}
