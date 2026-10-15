package lists

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the lists create command.
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --name <name>",
		Short: "Create a contact list",
		Long: `Create a contact list with no members. --tag can be used multiple times and
splits on commas, so --tag news,monthly adds two tags.`,
		Example: `  # Create a list
  ahasend lists create --name Newsletter

  # Create a list with a description and tags
  ahasend lists create --name Newsletter --description "Monthly news" --tag news --tag monthly`,
		Args:         cmdutil.NoArgs,
		RunE:         runListsCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "List name (required)")
	cmd.Flags().String("description", "", "List description")
	cmdutil.AddRepeatableFlag(cmd, "tag", "Tag for the list (can be used multiple times)")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runListsCreate(cmd *cobra.Command, _ []string) error {
	name, err := cmdutil.ReadRequiredText(cmd, "name")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("name", &name, cmdutil.MaxNameLength, strings.TrimSpace); err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("description", cmdutil.ChangedString(cmd, "description"), maxDescriptionLength, strings.TrimSpace); err != nil {
		return err
	}

	tags, err := cmdutil.ReadRepeatable(cmd, "tag", "")
	if err != nil {
		return err
	}

	request := requests.CreateListRequest{
		Name:        name,
		Description: cmdutil.ChangedString(cmd, "description"),
		Tags:        tags,
	}

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"name": name,
	}).Debug("Executing lists create command")

	list, err := apiClient.CreateList(request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleList(list, printer.SingleConfig{
		SuccessMessage: "List created successfully",
		EmptyMessage:   "No list created",
	})
}
