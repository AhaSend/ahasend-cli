package versions

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewListCommand creates the templates versions list command.
func NewListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <template-id>",
		Short: "List a template's published versions",
		Long: `List a template's published versions, newest first, without their content.
The list holds at most 50 versions, so it is not paginated.`,
		Example: `  # List a template's published versions
  ahasend templates versions list 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runVersionsList,
		SilenceUsage: true,
	}
}

func runVersionsList(cmd *cobra.Command, args []string) error {
	templateID, err := validation.ParseUUIDField("template ID", args[0])
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"template_id": args[0],
	}).Debug("Executing templates versions list command")

	response, err := apiClient.GetTemplateVersions(templateID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleTemplateVersionList(response, printer.ListConfig{
		SuccessMessage: "Template versions retrieved successfully",
		EmptyMessage:   "No template versions found",
	})
}
