package draft

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the templates draft get command.
func NewGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <template-id>",
		Short: "Get a template's draft",
		Long: `Get a template's unpublished draft. A template with no draft returns a not
found error.`,
		Example: `  # Get a template's draft
  ahasend templates draft get 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runDraftGet,
		SilenceUsage: true,
	}
}

func runDraftGet(cmd *cobra.Command, args []string) error {
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
	}).Debug("Executing templates draft get command")

	draft, err := apiClient.GetTemplateDraft(templateID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleTemplateDraft(draft, printer.SingleConfig{
		SuccessMessage: "Template draft retrieved successfully",
		EmptyMessage:   "Template draft not found",
	})
}
