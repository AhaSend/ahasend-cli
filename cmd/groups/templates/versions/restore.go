package versions

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewRestoreCommand creates the templates versions restore command.
func NewRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <template-id> <version-id>",
		Short: "Restore a published version into a template's draft",
		Long: `Restore a published version into the template's draft, replacing the draft.
--publish publishes the draft once the version is restored; a version the
publish checks refuse is refused before the restore, and the draft is left as
it was. Restore asks for confirmation unless --force is set.`,
		Example: `  # Restore a version into the draft
  ahasend templates versions restore 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000

  # Restore a version and publish it without confirmation
  ahasend templates versions restore 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000 --publish --force`,
		Args:         cmdutil.ExactArgs(2),
		RunE:         runVersionsRestore,
		SilenceUsage: true,
	}
	cmd.Flags().Bool("publish", false, "Publish the draft once the version is restored")
	prompt.AddForceFlag(cmd)
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runVersionsRestore(cmd *cobra.Command, args []string) error {
	templateID, versionID := args[0], args[1]
	templateUUID, err := validation.ParseUUIDField("template ID", templateID)
	if err != nil {
		return err
	}
	versionUUID, err := validation.ParseUUIDField("version ID", versionID)
	if err != nil {
		return err
	}
	publish, _ := cmd.Flags().GetBool("publish")

	question := fmt.Sprintf("Are you sure you want to restore version %s into the draft of template %s?", versionID, templateID)
	if publish {
		question = fmt.Sprintf("Are you sure you want to restore version %s into the draft of template %s and publish it?",
			versionID, templateID)
	}
	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, question); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"template_id": templateID,
		"version_id":  versionID,
		"publish":     publish,
	}).Debug("Executing templates versions restore command")

	template, err := apiClient.RestoreTemplateVersion(templateUUID, versionUUID,
		requests.RestoreTemplateVersionRequest{Publish: publish}, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleTemplate(template, printer.SingleConfig{
		SuccessMessage: "Template version restored successfully",
		EmptyMessage:   "No template returned",
	})
}
