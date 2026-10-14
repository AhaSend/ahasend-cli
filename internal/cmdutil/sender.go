package cmdutil

import (
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/spf13/cobra"
)

// AddSenderFlags adds --from and --from-name.
func AddSenderFlags(cmd *cobra.Command) {
	cmd.Flags().String("from", "", "Sender email address")
	AddSenderNameFlag(cmd)
}

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

// AddUpdateSenderFlags adds --from, --from-name and --clear-from, with
// clearFromUsage as the help of --clear-from.
func AddUpdateSenderFlags(cmd *cobra.Command, clearFromUsage string) {
	AddSenderFlags(cmd)
	cmd.Flags().Bool("clear-from", false, clearFromUsage)
}

// ReadSender builds the sender from the sender flags, or returns nil when none
// is set. --clear-from returns an empty address, which the SDK sends as null.
func ReadSender(cmd *cobra.Command) (*common.SenderAddress, error) {
	from, clearSender, err := ReadClearable(cmd, "from", "clear-from")
	if err != nil {
		return nil, err
	}
	if err := RejectTogether(cmd, "from", "clear-from"); err != nil {
		return nil, err
	}
	if err := validation.ValidateOptionalEmail("--from", from); err != nil {
		return nil, err
	}
	name, err := ReadSenderName(cmd)
	if err != nil {
		return nil, err
	}

	if clearSender {
		return &common.SenderAddress{}, nil
	}
	if from == nil {
		return nil, nil
	}
	return &common.SenderAddress{Email: *from, Name: name}, nil
}
