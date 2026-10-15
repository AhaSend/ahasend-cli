package templates

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the templates create command.
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --name <name>",
		Short: "Create a transactional template",
		Long: `Create a transactional template. Its fields go to the template's draft, so
sends fail until the draft is published; --publish publishes it in the same
request.

Content comes from files: --mjml-file for an advanced template or --html-file
for an html template, and --text-file for the plain text version, which is
otherwise made from the HTML. --editor is needed when there is neither an MJML
nor an HTML file.`,
		Example: `  # Create and publish an advanced template
  ahasend templates create --name Welcome --subject "Welcome, {{ first_name }}" \
    --mjml-file welcome.mjml --from hello@example.com --from-name "Example" --publish

  # Create an html template with its own text version
  ahasend templates create --name Receipt --html-file receipt.html --text-file receipt.txt

  # Create a simple template from text only
  ahasend templates create --name Notice --editor simple --text-file notice.txt`,
		Args:         cmdutil.NoArgs,
		RunE:         runTemplatesCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Template name (required)")
	cmd.Flags().String("subject", "", "Subject")
	cmd.Flags().String("preheader", "", "Preview text")
	cmdutil.AddCreateContentFlags(cmd)
	cmdutil.AddSenderFlags(cmd)
	cmd.Flags().String("reply-to", "", "Default reply-to email address (needs --from)")
	cmd.Flags().Bool("publish", false, "Publish the draft once it is written")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runTemplatesCreate(cmd *cobra.Command, _ []string) error {
	name, err := cmdutil.ReadRequiredText(cmd, "name")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("name", &name, cmdutil.MaxNameLength, strings.TrimSpace); err != nil {
		return err
	}
	if err := cmdutil.CheckSubjectAndPreheader(cmd); err != nil {
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
	from, err := cmdutil.ReadSender(cmd)
	if err != nil {
		return err
	}
	replyTo, err := readReplyTo(cmd)
	if err != nil {
		return err
	}
	if replyTo != nil && from == nil {
		return errors.NewValidationError("--reply-to needs --from", nil)
	}
	publish, _ := cmd.Flags().GetBool("publish")

	request := requests.CreateTemplateRequest{
		Name:      name,
		Editor:    editor,
		Subject:   cmdutil.ChangedString(cmd, "subject"),
		Preheader: cmdutil.ChangedString(cmd, "preheader"),
		From:      from,
		ReplyTo:   replyTo,
		Content:   content,
		Publish:   publish,
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
		"name":    name,
		"publish": publish,
	}).Debug("Executing templates create command")

	template, err := apiClient.CreateTemplate(request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleTemplate(template, printer.SingleConfig{
		SuccessMessage: "Template created successfully",
		EmptyMessage:   "No template created",
	})
}
