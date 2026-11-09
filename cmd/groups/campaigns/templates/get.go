package templates

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the campaigns templates get command.
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <campaign-template-id>",
		Short: "Get a campaign template",
		Long: `Get one campaign template with its content.

--markup writes only the AhaML of a Simple Editor design to stdout. Save it,
edit it and write it back with "campaigns templates update --markup-file".
It fails when the design has no AhaML, and cannot be used with --output.`,
		Example: `  # Get a campaign template
  ahasend campaigns templates get 123e4567-e89b-12d3-a456-426614174000

  # Save the design as AhaML to edit it
  ahasend campaigns templates get 123e4567-e89b-12d3-a456-426614174000 --markup > design.ahaml`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignTemplatesGet,
		SilenceUsage: true,
	}
	cmdutil.AddMarkupFlag(cmd)
	return cmd
}

func runCampaignTemplatesGet(cmd *cobra.Command, args []string) error {
	templateID, err := validation.ParseUUIDField("campaign template ID", args[0])
	if err != nil {
		return err
	}
	markup, err := cmdutil.ReadMarkupFlag(cmd)
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
	if markup {
		var content *responses.TemplateContent
		if template != nil {
			content = template.Content
		}
		return cmdutil.WriteMarkup(cmd, "campaign template", args[0], content)
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleCampaignTemplate(template, printer.SingleConfig{
		SuccessMessage: "Campaign template retrieved successfully",
		EmptyMessage:   "Campaign template not found",
	})
}
