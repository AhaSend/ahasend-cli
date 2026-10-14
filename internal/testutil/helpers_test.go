package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/config"
)

func TestWriteProfilesWritesAConfigTheCLILoads(t *testing.T) {
	home := UseNewHome(t)
	const accountID = "11111111-1111-4111-8111-111111111111"

	WriteProfiles(t, home, "http://127.0.0.1:9", accountID, "work", "personal")

	manager, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, manager.Load())
	assert.Equal(t, "work", manager.GetConfig().DefaultProfile)
	assert.Equal(t, map[string]config.Profile{
		"work":     {APIKey: "aha-sk-work", APIURL: "http://127.0.0.1:9", AccountID: accountID},
		"personal": {APIKey: "aha-sk-personal", APIURL: "http://127.0.0.1:9", AccountID: accountID},
	}, manager.GetConfig().Profiles)
}
