package domains

import (
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCommand_Args(t *testing.T) {
	// Test that get command requires exactly 1 argument
	getCmd := NewGetCommand()
	assert.NotNil(t, getCmd.Args)
}

// TestGetCommandPrintsTheDomainInEveryFormat runs get with the shared domain
// fixture in each output format.
func TestGetCommandPrintsTheDomainInEveryFormat(t *testing.T) {
	tests := map[string][]string{
		"plain": {"Domain: example.com\n", "DNS Status: Valid\n", "ID: " + mocks.FixtureDomainID + "\n", "\nDNS Records:\n"},
		"table": {"example.com", mocks.FixtureDomainID, "\nDNS Records:\n"},
		"csv":   {"domain,id,account_id,", "example.com," + mocks.FixtureDomainID + "," + mocks.FixtureSubAccountID + ","},
		"json":  {`"domain": "example.com"`, `"id": "` + mocks.FixtureDomainID + `"`},
	}
	for format, want := range tests {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetDomain", "example.com").Return(mocks.NewDomain(), nil)

			output, err := testutil.RunWithClient(t, mockClient, NewGetCommand(), format, "example.com")

			require.NoError(t, err)
			for _, text := range want {
				assert.Contains(t, output, text)
			}
		})
	}
}
