package domains

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the domains command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "Manage your email sending domains",
		Long: `Manage your email sending domains including DNS verification, monitoring,
and configuration. Domains must be verified before you can send emails from them.

Common workflow:
  1. Create a domain: ahasend domains create example.com
  2. Configure DNS records as shown
  3. Verify the domain: ahasend domains verify example.com
  4. Check status: ahasend domains get example.com`,
	}

	// Add subcommands
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewVerifyCommand())
	cmd.AddCommand(NewDeleteCommand())

	return cmd
}
