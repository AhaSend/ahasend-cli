package templates

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

// NewDeleteCommand creates the templates delete command.
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <template-id>",
		Short: "Delete a transactional template",
		Long: `Delete a transactional template. Sends of it fail from then on. Delete asks
for confirmation unless --force is set.`,
		Example: `  # Delete a template
  ahasend templates delete 123e4567-e89b-12d3-a456-426614174000

  # Delete a template without confirmation
  ahasend templates delete 123e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runTemplatesDelete,
		SilenceUsage: true,
	}
	prompt.AddForceFlag(cmd)
	return cmd
}

func runTemplatesDelete(cmd *cobra.Command, args []string) error {
	templateID := args[0]
	templateUUID, err := validation.ParseUUIDField("template ID", templateID)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete template %s?", templateID)); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"template_id": templateID,
	}).Debug("Executing templates delete command")

	response, err := apiClient.DeleteTemplate(templateUUID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Template '%s' deleted successfully", templateID),
	})
}
