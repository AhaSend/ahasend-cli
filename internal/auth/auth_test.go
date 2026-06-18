package auth

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/client"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
)

func TestGetAuthenticatedClientUsesInjectedResolver(t *testing.T) {
	cmd := newAuthTestCommand()
	mockClient := &mocks.MockClient{}
	var resolvedCommand *cobra.Command

	restore := SetAuthenticatedClientResolverForTesting(func(cmd *cobra.Command) (client.AhaSendClient, error) {
		resolvedCommand = cmd
		return mockClient, nil
	})
	t.Cleanup(restore)

	got, err := GetAuthenticatedClient(cmd)

	require.NoError(t, err)
	assert.Same(t, mockClient, got)
	assert.Same(t, cmd, resolvedCommand)
}

func TestSetAuthenticatedClientResolverForTestingRestoreReturnsDefaultBehavior(t *testing.T) {
	cmd := newAuthTestCommand()
	require.NoError(t, cmd.Flags().Set("api-key", "test-api-key"))

	mockClient := &mocks.MockClient{}
	restore := SetAuthenticatedClientResolverForTesting(func(*cobra.Command) (client.AhaSendClient, error) {
		return mockClient, nil
	})
	t.Cleanup(restore)

	got, err := GetAuthenticatedClient(cmd)
	require.NoError(t, err)
	assert.Same(t, mockClient, got)

	restore()

	got, err = GetAuthenticatedClient(cmd)
	assert.Nil(t, got)
	require.Error(t, err)

	var cliErr *clierrors.CLIError
	require.ErrorAs(t, err, &cliErr)
	assert.Equal(t, clierrors.ErrCodeValidation, cliErr.Code)
	assert.Equal(t, "--account-id is required when using --api-key", cliErr.Message)
}

func newAuthTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("api-key", "", "")
	cmd.Flags().String("account-id", "", "")
	cmd.Flags().String("profile", "", "")
	return cmd
}
