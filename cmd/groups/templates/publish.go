package templates

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewPublishCommand creates the templates publish command.
func NewPublishCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish <template-id>",
		Short: "Publish a template's draft",
		Long: `Publish a template's draft, including changes made in the dashboard, and keep
it as a new version. Sends use the published copy from then on.`,
		Example: `  # Publish a template's draft
  ahasend templates publish 123e4567-e89b-12d3-a456-426614174000

  # Publish with an idempotency key for safe retries
  ahasend templates publish 123e4567-e89b-12d3-a456-426614174000 --idempotency-key publish-1`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runTemplatesPublish,
		SilenceUsage: true,
	}
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runTemplatesPublish(cmd *cobra.Command, args []string) error {
	templateID, err := validation.ParseUUIDField("template ID", args[0])
	if err != nil {
		return err
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
		"template_id": args[0],
	}).Debug("Executing templates publish command")

	template, err := apiClient.PublishTemplate(templateID, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleTemplate(template, printer.SingleConfig{
		SuccessMessage: "Template published successfully",
		EmptyMessage:   "No template published",
	})
}
