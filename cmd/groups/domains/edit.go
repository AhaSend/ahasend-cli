package domains

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	ahasend "github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewEditCommand creates the edit command
func NewEditCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <domain>",
		Short: "Update domain DNS settings",
		Long: `Update DNS domain settings such as custom subdomains and DKIM rotation interval.

Only provided fields are updated; omitted fields remain unchanged.
Subdomain fields that have been locked after DNS verification cannot be changed.
DKIM rotation interval is only available for managed DNS domains on eligible plans.`,
		Example: `  # Update tracking subdomain
  ahasend domains edit example.com --tracking-subdomain click

  # Update multiple subdomains
  ahasend domains edit example.com --tracking-subdomain click --return-path-subdomain mail

  # Set DKIM rotation interval (managed DNS only)
  ahasend domains edit example.com --dkim-rotation-interval 45

  # Update all settings at once
  ahasend domains edit example.com \
    --tracking-subdomain click \
    --return-path-subdomain mail \
    --subscription-subdomain preferences \
    --media-subdomain media \
    --dkim-rotation-interval 60`,
		Args:         cobra.ExactArgs(1),
		RunE:         runDomainsEdit,
		SilenceUsage: true,
	}

	cmd.Flags().String("tracking-subdomain", "", "Custom tracking subdomain")
	cmd.Flags().String("return-path-subdomain", "", "Custom return-path subdomain")
	cmd.Flags().String("subscription-subdomain", "", "Custom subscription management subdomain")
	cmd.Flags().String("media-subdomain", "", "Custom media subdomain")
	cmd.Flags().Int("dkim-rotation-interval", 0, "DKIM rotation interval in days (managed DNS only, 30-180)")

	return cmd
}

func runDomainsEdit(cmd *cobra.Command, args []string) error {
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	domain := args[0]

	// Build the request from provided flags only
	req := requests.UpdateDomainRequest{}
	hasUpdates := false

	if cmd.Flags().Changed("tracking-subdomain") {
		v, _ := cmd.Flags().GetString("tracking-subdomain")
		req.TrackingSubdomain = ahasend.String(v)
		hasUpdates = true
	}
	if cmd.Flags().Changed("return-path-subdomain") {
		v, _ := cmd.Flags().GetString("return-path-subdomain")
		req.ReturnPathSubdomain = ahasend.String(v)
		hasUpdates = true
	}
	if cmd.Flags().Changed("subscription-subdomain") {
		v, _ := cmd.Flags().GetString("subscription-subdomain")
		req.SubscriptionSubdomain = ahasend.String(v)
		hasUpdates = true
	}
	if cmd.Flags().Changed("media-subdomain") {
		v, _ := cmd.Flags().GetString("media-subdomain")
		req.MediaSubdomain = ahasend.String(v)
		hasUpdates = true
	}
	if cmd.Flags().Changed("dkim-rotation-interval") {
		v, _ := cmd.Flags().GetInt("dkim-rotation-interval")
		if v < 30 || v > 180 {
			return errors.NewValidationError("DKIM rotation interval must be between 30 and 180 days", nil)
		}
		req.DKIMRotationIntervalDays = ahasend.Int(v)
		hasUpdates = true
	}

	if !hasUpdates {
		return errors.NewValidationError("at least one setting must be provided to update", nil)
	}

	logger.Get().WithFields(map[string]interface{}{
		"domain":  domain,
		"request": fmt.Sprintf("%+v", req),
	}).Debug("Executing domain edit command")

	response, err := client.UpdateDomain(domain, req)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewNotFoundError(fmt.Sprintf("domain '%s' not found", domain), nil)
	}

	config := printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Domain '%s' updated successfully", domain),
		EmptyMessage:   "Domain not found",
		FieldOrder:     []string{"domain", "dns_valid", "tracking_subdomain", "return_path_subdomain", "subscription_subdomain", "media_subdomain", "dkim_rotation_interval_days", "id", "created_at", "updated_at"},
	}

	return handler.HandleSingleDomain(response, config)
}
