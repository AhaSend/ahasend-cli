package campaigns

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the campaigns create command.
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --name <name>",
		Short: "Create a draft campaign",
		Long: `Create a draft campaign. Every field but the name can be set later with
"campaigns update", and the campaign lists its issues until it is ready to
send.

The design comes from files, --mjml-file for an advanced design or --html-file
for an html design, with --text-file for the plain text version, which is
otherwise made from the HTML. --editor alone gives the campaign an empty
design of that editor. --campaign-template-id copies the design of a campaign
template instead, and cannot be combined with the content flags or --editor.`,
		Example: `  # Create a campaign ready to send
  ahasend campaigns create --name "October news" --subject "What's new" \
    --from news@example.com --from-name "Example" --list-id 123e4567-e89b-12d3-a456-426614174000 \
    --html-file october.html

  # Start a campaign from a campaign template
  ahasend campaigns create --name "Launch" --campaign-template-id 123e4567-e89b-12d3-a456-426614174001

  # Create an empty draft
  ahasend campaigns create --name "Draft"`,
		Args:         cmdutil.NoArgs,
		RunE:         runCampaignsCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Campaign name, shown in the dashboard only (required)")
	cmd.Flags().String("subject", "", "Subject")
	cmd.Flags().String("preheader", "", "Preview text")
	cmdutil.AddCreateContentFlags(cmd)
	cmdutil.AddSenderFlags(cmd)
	cmd.Flags().String("reply-to", "", "Reply-to email address")
	cmdutil.AddRepeatableFlag(cmd, "list-id", "ID of a list to send to (can be used multiple times)")
	cmd.Flags().String("campaign-template-id", "", "ID of a campaign template whose design the campaign copies")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

// designFlags are the flags that set a design, which --campaign-template-id
// replaces.
var designFlags = []string{"editor", "mjml-file", "html-file", "text-file"}

func runCampaignsCreate(cmd *cobra.Command, _ []string) error {
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

	templateIDValue := cmdutil.ChangedString(cmd, "campaign-template-id")
	var templateID *uuid.UUID
	if templateIDValue != nil {
		for _, flag := range designFlags {
			if err := cmdutil.RejectTogether(cmd, "campaign-template-id", flag); err != nil {
				return err
			}
		}
		id, err := validation.ParseUUIDField("campaign template ID", *templateIDValue)
		if err != nil {
			return err
		}
		templateID = &id
	}

	content, err := cmdutil.ReadContent(cmd)
	if err != nil {
		return err
	}
	from, err := cmdutil.ReadSender(cmd)
	if err != nil {
		return err
	}
	editor, err := cmdutil.ReadOneOf(cmd, "editor")
	if err != nil {
		return err
	}
	replyTo, err := cmdutil.ReadNonEmpty(cmd, "reply-to", "")
	if err != nil {
		return err
	}
	if err := validation.ValidateOptionalEmail("--reply-to", replyTo); err != nil {
		return err
	}
	listIDs, err := readListIDs(cmd)
	if err != nil {
		return err
	}

	request := requests.CreateCampaignRequest{
		Name:       name,
		Subject:    cmdutil.ChangedString(cmd, "subject"),
		Preheader:  cmdutil.ChangedString(cmd, "preheader"),
		From:       from,
		ReplyTo:    replyTo,
		ListIDs:    listIDs,
		Editor:     editor,
		Content:    content,
		TemplateID: templateID,
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
		"name":  name,
		"lists": len(listIDs),
	}).Debug("Executing campaigns create command")

	campaign, err := apiClient.CreateCampaign(request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleCampaign(campaign, printer.SingleConfig{
		SuccessMessage: "Campaign created successfully",
		EmptyMessage:   "No campaign created",
	})
}
