package templates

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

// NewUpdateCommand creates the campaigns templates update command.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <campaign-template-id>",
		Short: "Update a campaign template",
		Long: `Partially update a campaign template. Only the flags you set are sent, and the
change applies at once. Campaigns already made from the template keep their
own copies. --regenerate-text makes the text version again from the HTML. The
editor cannot change.`,
		Example: `  # Rename a campaign template
  ahasend campaigns templates update 123e4567-e89b-12d3-a456-426614174000 --name "Newsletter v2"

  # Replace the design of a campaign template
  ahasend campaigns templates update 123e4567-e89b-12d3-a456-426614174000 --html-file newsletter.html

  # Make the text version again from the HTML
  ahasend campaigns templates update 123e4567-e89b-12d3-a456-426614174000 --regenerate-text`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignTemplatesUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New campaign template name")
	cmdutil.AddUpdateContentFlags(cmd)
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runCampaignTemplatesUpdate(cmd *cobra.Command, args []string) error {
	templateID, err := validation.ParseUUIDField("campaign template ID", args[0])
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
	content, err := cmdutil.ReadContent(cmd)
	if err != nil {
		return err
	}

	request := requests.UpdateCampaignTemplateRequest{
		Name:    name,
		Content: content,
	}
	if request.Name == nil && request.Content == nil {
		return errors.NewValidationError("at least one of --name, --mjml-file, --html-file, --text-file "+
			"or --regenerate-text must be provided", nil)
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
		"campaign_template_id": args[0],
	}).Debug("Executing campaigns templates update command")

	template, err := apiClient.UpdateCampaignTemplate(templateID, request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleCampaignTemplate(template, printer.SingleConfig{
		SuccessMessage: "Campaign template updated successfully",
		EmptyMessage:   "No campaign template updated",
	})
}
