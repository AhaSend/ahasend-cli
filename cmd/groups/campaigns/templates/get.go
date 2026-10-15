package templates

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the campaigns templates get command.
func NewGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <campaign-template-id>",
		Short: "Get a campaign template",
		Long:  `Get one campaign template with its content.`,
		Example: `  # Get a campaign template
  ahasend campaigns templates get 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignTemplatesGet,
		SilenceUsage: true,
	}
}

func runCampaignTemplatesGet(cmd *cobra.Command, args []string) error {
	templateID, err := validation.ParseUUIDField("campaign template ID", args[0])
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"campaign_template_id": args[0],
	}).Debug("Executing campaigns templates get command")

	template, err := apiClient.GetCampaignTemplate(templateID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleCampaignTemplate(template, printer.SingleConfig{
		SuccessMessage: "Campaign template retrieved successfully",
		EmptyMessage:   "Campaign template not found",
	})
}
