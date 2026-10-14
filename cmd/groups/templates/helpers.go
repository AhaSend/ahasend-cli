package templates

import (
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/spf13/cobra"
)

// readReplyTo builds the template reply-to, which is an email only, from
// --reply-to, or returns nil when it is not set. --clear-reply-to, on update,
// returns an empty address, which the SDK sends as null. A reply-to needs a
// sender, so --reply-to cannot go with --clear-from.
func readReplyTo(cmd *cobra.Command) (*common.SenderAddress, error) {
	replyTo, clearReplyTo, err := cmdutil.ReadClearable(cmd, "reply-to", "clear-reply-to")
	if err != nil {
		return nil, err
	}
	if err := validation.ValidateOptionalEmail("--reply-to", replyTo); err != nil {
		return nil, err
	}
	for _, conflict := range []string{"clear-reply-to", "clear-from"} {
		if err := cmdutil.RejectTogether(cmd, "reply-to", conflict); err != nil {
			return nil, err
		}
	}

	if clearReplyTo {
		return &common.SenderAddress{}, nil
	}
	if replyTo == nil {
		return nil, nil
	}
	return &common.SenderAddress{Email: *replyTo}, nil
}
