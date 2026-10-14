package draft

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

// NewDiscardCommand creates the templates draft discard command.
func NewDiscardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discard <template-id>",
		Short: "Discard a template's draft",
		Long: `Discard a template's unpublished draft and print the template. A template with
no draft is printed as it is. Discard asks for confirmation unless --force is
set.`,
		Example: `  # Discard a template's draft
  ahasend templates draft discard 123e4567-e89b-12d3-a456-426614174000

  # Discard a template's draft without confirmation
  ahasend templates draft discard 123e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runDraftDiscard,
		SilenceUsage: true,
	}
	prompt.AddForceFlag(cmd)
	return cmd
}

func runDraftDiscard(cmd *cobra.Command, args []string) error {
	templateID := args[0]
	templateUUID, err := validation.ParseUUIDField("template ID", templateID)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to discard the draft of template %s?", templateID)); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"template_id": templateID,
	}).Debug("Executing templates draft discard command")

	template, err := apiClient.DiscardTemplateDraft(templateUUID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleTemplate(template, printer.SingleConfig{
		SuccessMessage: "Template draft discarded successfully",
		EmptyMessage:   "No template returned",
	})
}
