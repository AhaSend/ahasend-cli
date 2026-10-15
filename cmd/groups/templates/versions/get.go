package versions

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the templates versions get command.
func NewGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <template-id> <version-id>",
		Short: "Get a published version of a template",
		Long:  `Get one published version of a template with its content.`,
		Example: `  # Get a published version of a template
  ahasend templates versions get 123e4567-e89b-12d3-a456-426614174000 223e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(2),
		RunE:         runVersionsGet,
		SilenceUsage: true,
	}
}

func runVersionsGet(cmd *cobra.Command, args []string) error {
	templateID, versionID := args[0], args[1]
	templateUUID, err := validation.ParseUUIDField("template ID", templateID)
	if err != nil {
		return err
	}
	versionUUID, err := validation.ParseUUIDField("version ID", versionID)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"template_id": templateID,
		"version_id":  versionID,
	}).Debug("Executing templates versions get command")

	version, err := apiClient.GetTemplateVersion(templateUUID, versionUUID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleTemplateVersion(version, printer.SingleConfig{
		SuccessMessage: "Template version retrieved successfully",
		EmptyMessage:   "Template version not found",
	})
}
