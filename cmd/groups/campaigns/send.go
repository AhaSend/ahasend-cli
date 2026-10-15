package campaigns

import (
	"fmt"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/timeparse"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewSendCommand creates the campaigns send command.
func NewSendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send <campaign-id>",
		Short: "Send or schedule a draft campaign",
		Long: `Send a draft campaign to its lists now, or at the time given with
--schedule: an RFC3339 time or a relative one such as "2h" from now. The API
takes a time more than a minute ahead; an earlier one fails before the
request. The campaign then goes through list
validation and review before it sends; follow its status with "campaigns get".

A campaign with issues is refused, and each issue is listed. To change the send
time of a scheduled campaign, unschedule it and send it again. Send asks for
confirmation unless --force is set.`,
		Example: `  # Send a campaign now
  ahasend campaigns send 123e4567-e89b-12d3-a456-426614174000

  # Schedule a campaign
  ahasend campaigns send 123e4567-e89b-12d3-a456-426614174000 --schedule 2026-11-01T09:00:00Z

  # Schedule a campaign for two hours from now
  ahasend campaigns send 123e4567-e89b-12d3-a456-426614174000 --schedule 2h

  # Send without confirmation, safe to retry
  ahasend campaigns send 123e4567-e89b-12d3-a456-426614174000 --force --idempotency-key send-october`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsSend,
		SilenceUsage: true,
	}

	cmd.Flags().String("schedule", "", "Schedule the send for an RFC3339 or relative time like '2h' (the API takes one more than a minute ahead); sends now when omitted")
	prompt.AddForceFlag(cmd)
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runCampaignsSend(cmd *cobra.Command, args []string) error {
	campaignID := args[0]
	campaignUUID, err := validation.ParseUUIDField("campaign ID", campaignID)
	if err != nil {
		return err
	}

	request := requests.SendCampaignRequest{}
	question := fmt.Sprintf("Are you sure you want to send campaign %s to its lists now?", campaignID)
	if schedule := cmdutil.ChangedString(cmd, "schedule"); schedule != nil {
		sendAt, err := timeparse.ParseTimeAhead("schedule", *schedule, "2h or 7d", time.Minute, 0)
		if err != nil {
			return err
		}
		request.SendAt = &sendAt
		question = fmt.Sprintf("Are you sure you want to send campaign %s to its lists at %s?",
			campaignID, printer.DisplayTime(sendAt))
	}

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, question); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"campaign_id": campaignID,
		"scheduled":   request.SendAt != nil,
	}).Debug("Executing campaigns send command")

	send, err := apiClient.SendCampaign(campaignUUID, request, idempotency...)
	if err != nil {
		return campaignNotReadyError(cmd, err, "send")
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleCampaignSend(send, printer.SingleConfig{
		SuccessMessage: "Campaign send accepted",
		EmptyMessage:   "No campaign sent",
	})
}
