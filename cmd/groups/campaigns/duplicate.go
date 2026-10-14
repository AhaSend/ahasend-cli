package campaigns

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewDuplicateCommand creates the campaigns duplicate command.
func NewDuplicateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "duplicate <campaign-id>",
		Short: "Copy a campaign into a new draft",
		Long: `Make a new draft from a campaign in any status. The copy gets the campaign's
design, subject, preview text, sender and the lists that still exist. Without
--name it is named "Copy of" and the campaign's name.`,
		Example: `  # Duplicate a campaign
  ahasend campaigns duplicate 123e4567-e89b-12d3-a456-426614174000

  # Duplicate a campaign under a new name
  ahasend campaigns duplicate 123e4567-e89b-12d3-a456-426614174000 --name "November news"`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsDuplicate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Name of the copy")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runCampaignsDuplicate(cmd *cobra.Command, args []string) error {
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
	request := requests.DuplicateCampaignRequest{
		Name: name,
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
	}).Debug("Executing campaigns duplicate command")

	campaign, err := apiClient.DuplicateCampaign(campaignID, request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleCampaign(campaign, printer.SingleConfig{
		SuccessMessage: "Campaign duplicated successfully",
		EmptyMessage:   "No campaign duplicated",
	})
}
