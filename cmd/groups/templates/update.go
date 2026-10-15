package templates

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the templates update command.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <template-id>",
		Short: "Update a transactional template",
		Long: `Partially update a transactional template. Only the flags you set are sent.
Every field but the name goes to the template's draft; the name changes at
once. --publish publishes the whole draft, and on its own only publishes.

--subject "" and --preheader "" clear those fields. --from replaces the sender
address and name together, and --clear-from removes the sender and the
reply-to. --clear-reply-to removes the reply-to. --regenerate-text makes the
text version again from the HTML. The editor cannot change.`,
		Example: `  # Change the subject in the draft
  ahasend templates update 123e4567-e89b-12d3-a456-426614174000 --subject "Welcome aboard"

  # Replace the design and publish it
  ahasend templates update 123e4567-e89b-12d3-a456-426614174000 --mjml-file welcome.mjml --publish

  # Make the text version again from the HTML
  ahasend templates update 123e4567-e89b-12d3-a456-426614174000 --regenerate-text

  # Remove the sender and the reply-to
  ahasend templates update 123e4567-e89b-12d3-a456-426614174000 --clear-from`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runTemplatesUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New template name")
	cmd.Flags().String("subject", "", "New subject (empty clears it)")
	cmd.Flags().String("preheader", "", "New preview text (empty clears it)")
	cmdutil.AddUpdateContentFlags(cmd)
	cmdutil.AddUpdateSenderFlags(cmd, "Remove the sender and the reply-to")
	cmd.Flags().String("reply-to", "", "New default reply-to email address (the template must have a sender; not with --clear-from)")
	cmd.Flags().Bool("clear-reply-to", false, "Remove the reply-to")
	cmd.Flags().Bool("publish", false, "Publish the draft once it is written")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runTemplatesUpdate(cmd *cobra.Command, args []string) error {
	templateID, err := validation.ParseUUIDField("template ID", args[0])
	if err != nil {
		return err
	}

	name, err := cmdutil.ReadNonEmpty(cmd, "name", "")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("name", name, cmdutil.MaxNameLength, strings.TrimSpace); err != nil {
		return err
	}
	if err := cmdutil.CheckSubjectAndPreheader(cmd); err != nil {
		return err
	}
	content, err := cmdutil.ReadContent(cmd)
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
	publish, _ := cmd.Flags().GetBool("publish")

	request := requests.UpdateTemplateRequest{
		Name:      name,
		Subject:   cmdutil.ChangedString(cmd, "subject"),
		Preheader: cmdutil.ChangedString(cmd, "preheader"),
		From:      from,
		ReplyTo:   replyTo,
		Content:   content,
		Publish:   publish,
	}
	if request.Name == nil && request.Subject == nil && request.Preheader == nil &&
		request.From == nil && request.ReplyTo == nil && request.Content == nil && !request.Publish {
		return errors.NewValidationError("at least one of --name, --subject, --preheader, --mjml-file, --html-file, "+
			"--text-file, --regenerate-text, --from, --clear-from, --reply-to, --clear-reply-to or --publish must be provided", nil)
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
		"publish":     publish,
	}).Debug("Executing templates update command")

	template, err := apiClient.UpdateTemplate(templateID, request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleTemplate(template, printer.SingleConfig{
		SuccessMessage: "Template updated successfully",
		EmptyMessage:   "No template updated",
	})
}
