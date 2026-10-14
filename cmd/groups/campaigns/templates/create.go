package templates

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the campaigns templates create command.
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --name <name>",
		Short: "Create a campaign template",
		Long: `Create a campaign template. Content comes from files: --mjml-file for an
advanced template or --html-file for an html template, and --text-file for the
plain text version, which is otherwise made from the HTML. --editor is needed
when there is neither an MJML nor an HTML file. A campaign can be made from the
template only once it has HTML.`,
		Example: `  # Create an advanced campaign template
  ahasend campaigns templates create --name Newsletter --mjml-file newsletter.mjml

  # Create an html campaign template with its own text version
  ahasend campaigns templates create --name Announcement --html-file announcement.html --text-file announcement.txt

  # Create an empty simple campaign template
  ahasend campaigns templates create --name Blank --editor simple`,
		Args:         cmdutil.NoArgs,
		RunE:         runCampaignTemplatesCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Campaign template name (required)")
	cmdutil.AddCreateContentFlags(cmd)
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runCampaignTemplatesCreate(cmd *cobra.Command, _ []string) error {
	name, err := cmdutil.ReadRequiredText(cmd, "name")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("name", &name, cmdutil.MaxNameLength, strings.TrimSpace); err != nil {
		return err
	}

	content, err := cmdutil.ReadContent(cmd)
	if err != nil {
		return err
	}

	editor, err := cmdutil.ReadTemplateEditor(cmd, content)
	if err != nil {
		return err
	}

	request := requests.CreateCampaignTemplateRequest{
		Name:    name,
		Editor:  editor,
		Content: content,
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
		"name": name,
	}).Debug("Executing campaigns templates create command")

	template, err := apiClient.CreateCampaignTemplate(request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleCampaignTemplate(template, printer.SingleConfig{
		SuccessMessage: "Campaign template created successfully",
		EmptyMessage:   "No campaign template created",
	})
}
