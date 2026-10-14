package domains

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the update command
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <domain>",
		Short: "Update a domain's settings",
		Long: `Update domain settings such as custom subdomains, DKIM rotation interval,
DKIM selector and sending type.

Only provided fields are updated; omitted fields remain unchanged.
Subdomain fields that have been locked after DNS verification cannot be
changed. DKIM rotation interval is only available for managed DNS domains on
eligible plans. DKIM selector is only available on Platform Partner accounts;
--dkim-selector "" clears it so the default selector applies. A sending type
change applies to new messages within five minutes.`,
		Example: `  # Update tracking subdomain
  ahasend domains update example.com --tracking-subdomain click

  # Update multiple subdomains
  ahasend domains update example.com --tracking-subdomain click --return-path-subdomain mail

  # Set DKIM rotation interval (managed DNS only)
  ahasend domains update example.com --dkim-rotation-interval 45

  # Switch the domain to marketing sending
  ahasend domains update example.com --sending-type marketing

  # Clear the custom DKIM selector
  ahasend domains update example.com --dkim-selector ""

  # Update all settings at once
  ahasend domains update example.com \
    --tracking-subdomain click \
    --return-path-subdomain mail \
    --subscription-subdomain preferences \
    --media-subdomain media \
    --dkim-rotation-interval 60`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runDomainsUpdate,
		SilenceUsage: true,
	}

	addSettingsFlags(cmd)
	cmd.Flags().String("dkim-selector", "", "Custom DKIM selector; an empty value clears it (Platform Partner accounts only)")
	cmdutil.AddOneOfFlag(cmd, "sending-type", "", "Sending type: %s", sendingTypes)

	return cmd
}

// newEditCommand creates the hidden edit command, the deprecated name of
// update, which prints a deprecation notice on stderr and runs update.
func newEditCommand() *cobra.Command {
	cmd := NewUpdateCommand()
	cmd.Use = "edit <domain>"
	cmd.Hidden = true
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.ErrOrStderr(), `Command "edit" is deprecated, use "domains update" instead`)
		return runDomainsUpdate(cmd, args)
	}
	return cmd
}

func runDomainsUpdate(cmd *cobra.Command, args []string) error {
	handler := printer.GetResponseHandlerFromCommand(cmd)

	domain := args[0]
	if err := validation.ValidatePathSegment("domain", domain); err != nil {
		return err
	}

	// Build the request from provided flags only
	settings, err := readSettings(cmd)
	if err != nil {
		return err
	}
	if settings.empty() {
		return errors.NewValidationError("at least one of --tracking-subdomain, --return-path-subdomain, --subscription-subdomain, --media-subdomain, --dkim-rotation-interval, --dkim-selector or --sending-type must be provided", nil)
	}

	req := requests.UpdateDomainRequest{
		TrackingSubdomain:        settings.TrackingSubdomain,
		ReturnPathSubdomain:      settings.ReturnPathSubdomain,
		SubscriptionSubdomain:    settings.SubscriptionSubdomain,
		MediaSubdomain:           settings.MediaSubdomain,
		DKIMRotationIntervalDays: settings.DKIMRotationIntervalDays,
		DKIMSelector:             settings.DKIMSelector,
		SendingType:              settings.SendingType,
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"domain":  domain,
		"request": fmt.Sprintf("%+v", req),
	}).Debug("Executing domains update command")

	response, err := client.UpdateDomain(domain, req)
	if err != nil {
		return err
	}

	config := printer.SingleConfig{
		SuccessMessage: "Domain updated successfully",
		EmptyMessage:   "Domain not found",
	}

	return handler.HandleSingleDomain(response, config)
}
