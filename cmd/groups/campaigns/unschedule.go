package campaigns

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewUnscheduleCommand creates the campaigns unschedule command.
func NewUnscheduleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unschedule <campaign-id>",
		Short: "Return a validating or scheduled campaign to draft",
		Long: `Return a validating or scheduled campaign to draft, cancelling its list
validation or its scheduled send. A campaign that has queued mail cannot be
unscheduled.`,
		Example: `  # Return a scheduled campaign to draft
  ahasend campaigns unschedule 123e4567-e89b-12d3-a456-426614174000`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsUnschedule,
		SilenceUsage: true,
	}
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runCampaignsUnschedule(cmd *cobra.Command, args []string) error {
	campaignID, err := validation.ParseUUIDField("campaign ID", args[0])
	if err != nil {
		return err
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
	}).Debug("Executing campaigns unschedule command")

	campaign, err := apiClient.UnscheduleCampaign(campaignID, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleCampaign(campaign, printer.SingleConfig{
		SuccessMessage: "Campaign unscheduled successfully",
		EmptyMessage:   "No campaign unscheduled",
	})
}
