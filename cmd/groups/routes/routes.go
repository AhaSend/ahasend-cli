package routes

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the routes command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "Manage inbound email routes",
		Long: `Manage inbound email routes for processing incoming messages through webhooks.
Routes allow you to configure how incoming emails are processed and forwarded
to your application endpoints.

Routes enable you to:
- Process inbound emails through webhooks
- Filter emails by recipient patterns
- Control attachment handling and formatting
- Group messages by conversation threads
- Strip reply content for cleaner processing

Common workflow:
  1. Create a route: ahasend routes create
  2. Configure recipient filtering and webhook settings
  3. Test your route endpoint
  4. Monitor route activity: ahasend routes list`,
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewListenCommand())
	cmd.AddCommand(NewTriggerCommand())

	return cmd
}
