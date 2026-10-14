package campaigns

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates the campaigns delete command.
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <campaign-id>",
		Short: "Delete a draft campaign",
		Long: `Delete a draft campaign with its design and its list choices. Delete asks
for confirmation unless --force is set.`,
		Example: `  # Delete a draft campaign
  ahasend campaigns delete 123e4567-e89b-12d3-a456-426614174000

  # Delete a draft campaign without confirmation
  ahasend campaigns delete 123e4567-e89b-12d3-a456-426614174000 --force`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runCampaignsDelete,
		SilenceUsage: true,
	}
	prompt.AddForceFlag(cmd)
	return cmd
}

func runCampaignsDelete(cmd *cobra.Command, args []string) error {
	campaignID := args[0]
	campaignUUID, err := validation.ParseUUIDField("campaign ID", campaignID)
	if err != nil {
		return err
	}

	if err := prompt.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete campaign %s?", campaignID)); err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"campaign_id": campaignID,
	}).Debug("Executing campaigns delete command")

	response, err := apiClient.DeleteCampaign(campaignUUID)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: fmt.Sprintf("Campaign '%s' deleted successfully", campaignID),
	})
}
