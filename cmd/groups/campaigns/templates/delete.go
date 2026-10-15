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

// NewDeleteCommand creates the campaigns templates delete command.
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <campaign-template-id>",
		Short: "Delete a campaign template",
		Long: `Delete a campaign template. Campaigns made from it keep their own copies.
Delete asks for confirmation unless --force is set.`,
		Example: `  # Delete a campaign template
  ahasend campaigns templates delete 123e4567-e89b-12d3-a456-426614174000

  # Delete a campaign template without confirmation
  ahasend campaigns templates delete 123e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignTemplatesDelete,
		SilenceUsage: true,
	}
	prompt.AddForceFlag(cmd)
	return cmd
}

func runCampaignTemplatesDelete(cmd *cobra.Command, args []string) error {
	templateID := args[0]
	templateUUID, err := validation.ParseUUIDField("campaign template ID", templateID)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete campaign template %s?", templateID)); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"campaign_template_id": templateID,
	}).Debug("Executing campaigns templates delete command")

	response, err := apiClient.DeleteCampaignTemplate(templateUUID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Campaign template '%s' deleted successfully", templateID),
	})
}
