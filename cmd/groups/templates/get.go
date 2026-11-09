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

// NewGetCommand creates the templates get command.
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <template-id>",
		Short: "Get a transactional template",
		Long: `Get one transactional template as its working copy: the draft while the
template has one, else the published copy. Has Draft says which one it is.
"templates update" changes this copy, so a change builds on what this command
shows.

--published gets the published copy instead, the one sends use, with the
variables a send must supply.

--markup writes only the AhaML of a Simple Editor design to stdout. Save it,
edit it and write it back with "templates update --markup-file". It fails
when the design has no AhaML, and cannot be used with --output.`,
		Example: `  # Get a template's working copy
  ahasend templates get 123e4567-e89b-12d3-a456-426614174000

  # Get the published copy that sends use
  ahasend templates get 123e4567-e89b-12d3-a456-426614174000 --published

  # Save the design as AhaML to edit it
  ahasend templates get 123e4567-e89b-12d3-a456-426614174000 --markup > design.ahaml`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runTemplatesGet,
		SilenceUsage: true,
	}
	cmd.Flags().Bool("published", false, "Get the published copy, which sends use, instead of the working copy")
	cmdutil.AddMarkupFlag(cmd)
	return cmd
}

func runTemplatesGet(cmd *cobra.Command, args []string) error {
	templateID, err := validation.ParseUUIDField("template ID", args[0])
	if err != nil {
		return err
	}
	published, _ := cmd.Flags().GetBool("published")
	markup, err := cmdutil.ReadMarkupFlag(cmd)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"template_id": args[0],
		"published":   published,
	}).Debug("Executing templates get command")

	var template *responses.Template
	if published {
		template, err = apiClient.GetPublishedTemplate(templateID)
	} else {
		template, err = apiClient.GetTemplate(templateID)
	}
	if err != nil {
		return err
	}
	if markup {
		var content *responses.TemplateContent
		if template != nil {
			content = template.Content
		}
		return cmdutil.WriteMarkup(cmd, "template", args[0], content)
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleTemplate(template, printer.SingleConfig{
		SuccessMessage: "Template retrieved successfully",
		EmptyMessage:   "Template not found",
	})
}
