package listen

import (
	"bytes"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parsed returns a command whose flags were added by addFlags and parsed from
// args.
func parsed(t *testing.T, addFlags func(*cobra.Command), args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	addFlags(cmd)
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

// confirmCommand returns a command with the --force flag, parsed from args,
// that reads stdin and writes to the returned stdout and stderr.
func confirmCommand(t *testing.T, stdin string, args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := parsed(t, prompt.AddForceFlag, args...)
	var stdout, stderr bytes.Buffer
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr
}

func assertValidationError(t *testing.T, err error, message string) {
	t.Helper()
	require.Error(t, err)
	var cliErr *errors.CLIError
	require.True(t, stderrors.As(err, &cliErr), "want a CLIError, got %T", err)
	assert.Equal(t, errors.ErrCodeValidation, cliErr.Code)
	assert.Contains(t, err.Error(), message)
}
