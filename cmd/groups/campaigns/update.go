package campaigns

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

// NewUpdateCommand creates the campaigns update command.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <campaign-id>",
		Short: "Update a draft campaign",
		Long: `Partially update a draft campaign. Only the flags you set are sent.

--subject "" and --preheader "" clear those fields. --from replaces the
sender address and name together, and --clear-from removes the sender but
keeps the reply-to. --clear-reply-to removes the reply-to.
--regenerate-text makes the text version again from the HTML. The editor
cannot change: --markup-file replaces a simple design, --mjml-file an
advanced one and --html-file an html one. A campaign with no design yet
takes any of the three, which sets its editor.

--list-id, --segment-id, --exclude-list-id and --exclude-segment-id each
replace their set of the audience, as "campaigns create" describes them, and
--clear-lists, --clear-segments, --clear-exclude-lists and
--clear-exclude-segments empty it; a set no flag names is kept. The API
refuses a campaign left with no list and no segment to send to. While a
segment of the audience is building, an update that changes the audience
can fail with 409 after the API waits for it; run the command again, with
the same --idempotency-key or a new one. "segments get" shows a segment's
state.`,
		Example: `  # Change the subject
  ahasend campaigns update 123e4567-e89b-12d3-a456-426614174000 --subject "What's new in October"

  # Replace the design and make the text version again
  ahasend campaigns update 123e4567-e89b-12d3-a456-426614174000 --html-file october.html --regenerate-text

  # Write back an edited AhaML design
  ahasend campaigns update 123e4567-e89b-12d3-a456-426614174000 --markup-file october.ahaml

  # Replace the lists the campaign sends to
  ahasend campaigns update 123e4567-e89b-12d3-a456-426614174000 --list-id 123e4567-e89b-12d3-a456-426614174001

  # Send to a segment instead of the campaign's lists
  ahasend campaigns update 123e4567-e89b-12d3-a456-426614174000 --segment-id 123e4567-e89b-12d3-a456-426614174002 \
    --clear-lists

  # Stop leaving out the members of segments
  ahasend campaigns update 123e4567-e89b-12d3-a456-426614174000 --clear-exclude-segments

  # Remove the reply-to
  ahasend campaigns update 123e4567-e89b-12d3-a456-426614174000 --clear-reply-to`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New campaign name")
	cmd.Flags().String("subject", "", "New subject (empty clears it)")
	cmd.Flags().String("preheader", "", "New preview text (empty clears it)")
	cmdutil.AddUpdateContentFlags(cmd)
	cmdutil.AddUpdateSenderFlags(cmd, "Remove the sender")
	cmd.Flags().String("reply-to", "", "New reply-to email address")
	cmd.Flags().Bool("clear-reply-to", false, "Remove the reply-to")
	cmdutil.AddRepeatableFlag(cmd, "list-id", "New list to send to, replacing the lists (can be used multiple times)")
	cmd.Flags().Bool("clear-lists", false, "Remove every list the campaign sends to")
	cmdutil.AddRepeatableFlag(cmd, "segment-id", "New segment to send to, replacing the segments (can be used multiple times)")
	cmd.Flags().Bool("clear-segments", false, "Remove every segment the campaign sends to")
	cmdutil.AddRepeatableFlag(cmd, "exclude-list-id",
		"New list whose contacts get no message, replacing the excluded lists (can be used multiple times)")
	cmd.Flags().Bool("clear-exclude-lists", false, "Exclude no list")
	cmdutil.AddRepeatableFlag(cmd, "exclude-segment-id",
		"New segment whose members get no message, replacing the excluded segments (can be used multiple times)")
	cmd.Flags().Bool("clear-exclude-segments", false, "Exclude no segment")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runCampaignsUpdate(cmd *cobra.Command, args []string) error {
	campaignID, err := validation.ParseUUIDField("campaign ID", args[0])
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
	sets, err := readAudience(cmd)
	if err != nil {
		return err
	}
	replyTo, clearReplyTo, err := cmdutil.ReadClearable(cmd, "reply-to", "clear-reply-to")
	if err != nil {
		return err
	}
	if err := validation.ValidateOptionalEmail("--reply-to", replyTo); err != nil {
		return err
	}
	if err := cmdutil.RejectTogether(cmd, "reply-to", "clear-reply-to"); err != nil {
		return err
	}
	if clearReplyTo {
		replyTo = new(string)
	}

	request := requests.UpdateCampaignRequest{
		Name:              name,
		Subject:           cmdutil.ChangedString(cmd, "subject"),
		Preheader:         cmdutil.ChangedString(cmd, "preheader"),
		From:              from,
		ReplyTo:           replyTo,
		ListIDs:           sets.listIDs,
		SegmentIDs:        sets.segmentIDs,
		ExcludeListIDs:    sets.excludeListIDs,
		ExcludeSegmentIDs: sets.excludeSegmentIDs,
		Content:           content,
	}
	if request.Name == nil && request.Subject == nil && request.Preheader == nil && request.From == nil &&
		request.ReplyTo == nil && request.ListIDs == nil && request.SegmentIDs == nil && request.ExcludeListIDs == nil &&
		request.ExcludeSegmentIDs == nil && request.Content == nil {
		return errors.NewValidationError("at least one of --name, --subject, --preheader, --markup-file, --mjml-file, "+
			"--html-file, --text-file, --regenerate-text, --from, --clear-from, --reply-to, --clear-reply-to, --list-id, "+
			"--clear-lists, --segment-id, --clear-segments, --exclude-list-id, --clear-exclude-lists, --exclude-segment-id "+
			"or --clear-exclude-segments must be provided", nil)
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
		"campaign_id": args[0],
	}).Debug("Executing campaigns update command")

	campaign, err := apiClient.UpdateCampaign(campaignID, request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleCampaign(campaign, printer.SingleConfig{
		SuccessMessage: "Campaign updated successfully",
		EmptyMessage:   "No campaign updated",
	})
}
