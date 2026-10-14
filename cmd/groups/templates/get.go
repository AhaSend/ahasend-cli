package templates

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the templates get command.
func NewGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <template-id>",
		Short: "Get a transactional template",
		Long: `Get one transactional template as sends use it: its published copy,
including the variables a send must supply.`,
		Example: `  # Get a template
  ahasend templates get 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runTemplatesGet,
		SilenceUsage: true,
	}
}

func runTemplatesGet(cmd *cobra.Command, args []string) error {
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
	}).Debug("Executing templates get command")

	template, err := apiClient.GetTemplate(templateID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleTemplate(template, printer.SingleConfig{
		SuccessMessage: "Template retrieved successfully",
		EmptyMessage:   "Template not found",
	})
}
