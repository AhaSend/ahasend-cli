package webhooks

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the webhooks command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhooks",
		Short: "Manage your webhook endpoints",
		Long: `Manage your webhook endpoints for receiving real-time event notifications
from AhaSend. Configure webhooks to receive notifications about message events,
domain verification, suppressions, and more.

Webhooks allow you to build reactive applications that respond to email events
in real-time, such as tracking deliveries, bounces, opens, and clicks.

Common workflow:
  1. Create a webhook: ahasend webhooks create
  2. Configure event types and URL
  3. Test your webhook endpoint
  4. Monitor webhook activity: ahasend webhooks list`,
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())

	return cmd
}
