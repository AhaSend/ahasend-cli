package campaigns

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewRecipientsCommand creates the campaigns recipients command.
func NewRecipientsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipients <campaign-id>",
		Short: "List a campaign's recipients",
		Long: `List a campaign's recipients, with cursor pagination. Give exactly one of
--status, a status that "campaigns stats" counts by status, in any case:
Processing, Queued, Deferred, Delivered, Bounced, Failed, "Administratively
Bounced", Suppressed, Rejected, Complained or Unsubscribed; or
--bounce-classification, a classification that "campaigns stats" counts by
bounce classification, other than Unknown.`,
		Example: `  # List the recipients a campaign delivered to
  ahasend campaigns recipients 123e4567-e89b-12d3-a456-426614174000 --status Delivered

  # List administratively bounced recipients, 50 at a time
  ahasend campaigns recipients 123e4567-e89b-12d3-a456-426614174000 --status "Administratively Bounced" --limit 50`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsRecipients,
		SilenceUsage: true,
	}

	cmdutil.AddOneOfFlag(cmd, "status", "", "Recipient status to list: %s", recipientStatuses)
	cmd.Flags().String("bounce-classification", "", "Bounce classification to list")
	cmdutil.AddPaginationFlags(cmd)
	return cmd
}

// recipientStatuses are the values of --status: the names "campaigns stats"
// counts recipients by, as the API spells them.
var recipientStatuses = []string{"Processing", "Queued", "Deferred", "Delivered", "Bounced", "Failed",
	"Administratively Bounced", "Suppressed", "Rejected", "Complained", "Unsubscribed"}

func runCampaignsRecipients(cmd *cobra.Command, args []string) error {
	campaignID, err := validation.ParseUUIDField("campaign ID", args[0])
	if err != nil {
		return err
	}
	if err := cmdutil.RejectTogether(cmd, "status", "bounce-classification"); err != nil {
		return err
	}
	status, err := cmdutil.ReadOneOf(cmd, "status")
	if err != nil {
		return err
	}
	bounceClassification, err := cmdutil.ReadNonEmpty(cmd, "bounce-classification", "")
	if err != nil {
		return err
	}
	if status == nil && bounceClassification == nil {
		return errors.NewValidationError("one of --status or --bounce-classification must be provided", nil)
	}
	limit, after, before, err := cmdutil.ReadPagination(cmd)
	if err != nil {
		return err
	}

	params := requests.ListCampaignRecipientsParams{
		Status:               status,
		BounceClassification: bounceClassification,
		Limit:                limit,
		After:                after,
		Before:               before,
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"campaign_id":           args[0],
		"status":                params.Status,
		"bounce_classification": params.BounceClassification,
	}).Debug("Executing campaigns recipients command")

	response, err := apiClient.ListCampaignRecipients(campaignID, params)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleCampaignRecipientList(response, printer.ListConfig{
		SuccessMessage: "Campaign recipients retrieved successfully",
		EmptyMessage:   "No recipients found",
	})
}
