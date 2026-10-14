package cmdutil

import (
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/spf13/cobra"
)

// AddSenderNameFlag adds --from-name, for a command that declares --from
// itself. ReadSenderName reads it.
func AddSenderNameFlag(cmd *cobra.Command) {
	cmd.Flags().String("from-name", "", "Sender display name (needs --from)")
}

// ReadSenderName returns the value of --from-name, or nil when it was not
// set. --from-name without --from is a validation error.
func ReadSenderName(cmd *cobra.Command) (*string, error) {
	name := ChangedString(cmd, "from-name")
	if name != nil && !cmd.Flags().Changed("from") {
		return nil, errors.NewValidationError("--from-name needs --from", nil)
	}
	return name, nil
}
