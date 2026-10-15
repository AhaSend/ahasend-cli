package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
)

// TestSendCommandsDescribeTheirSharedFlagsAlike checks that a flag messages
// send and smtp send share has one help text in both, apart from what one
// command adds, such as when the flag is required.
func TestSendCommandsDescribeTheirSharedFlagsAlike(t *testing.T) {
	// The flags whose help a command adds to
	qualified := map[string]string{
		"from":    cmdutil.FromUsage,
		"subject": cmdutil.SubjectUsage,
		"attach":  cmdutil.AttachUsage,
		"header":  cmdutil.HeaderUsage,
	}
	messagesSend, smtpSend := findCommand(t, "messages", "send"), findCommand(t, "smtp", "send")
	shared := 0
	messagesSend.Flags().VisitAll(func(flag *pflag.Flag) {
		other := smtpSend.Flags().Lookup(flag.Name)
		if other == nil || flag.Hidden {
			return
		}
		shared++
		if base, ok := qualified[flag.Name]; ok {
			assert.True(t, strings.HasPrefix(flag.Usage, base), "messages send --%s: %s", flag.Name, flag.Usage)
			assert.True(t, strings.HasPrefix(other.Usage, base), "smtp send --%s: %s", flag.Name, other.Usage)
			return
		}
		assert.Equal(t, flag.Usage, other.Usage, "--%s", flag.Name)
	})
	assert.GreaterOrEqual(t, shared, 14, "messages send and smtp send share their message flags")
}

// findCommand returns the command at path in a new command tree.
func findCommand(t *testing.T, path ...string) *cobra.Command {
	t.Helper()
	cmd, _, err := NewRootCmdForTesting().Find(path)
	if err != nil {
		t.Fatalf("command %v: %v", path, err)
	}
	return cmd
}
