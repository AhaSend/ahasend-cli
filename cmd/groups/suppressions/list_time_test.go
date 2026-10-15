package suppressions

import (
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

func TestListSendsTheCreationTimeRange(t *testing.T) {
	mockClient := &mocks.MockClient{}
	var sent requests.GetSuppressionsParams
	mockClient.On("ListSuppressions", mock.Anything).Run(func(args mock.Arguments) {
		sent = args.Get(0).(requests.GetSuppressionsParams)
	}).Return(&responses.PaginatedSuppressionsResponse{}, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "plain",
		"--from-time", "7d", "--to-time", "2099-01-01T00:00:00Z")

	require.NoError(t, err)
	require.NotNil(t, sent.FromTime)
	assert.WithinDuration(t, time.Now().AddDate(0, 0, -7), *sent.FromTime, time.Minute)
	require.NotNil(t, sent.ToTime)
	assert.Equal(t, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), sent.ToTime.UTC())
}

func TestListRejectsABackwardTimeRangeBeforeAuthenticating(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewListCommand(), "--from-time", "1d", "--to-time", "7d")

	require.Error(t, err)
	assert.Equal(t, "--from-time must be before --to-time", err.Error())
	assert.False(t, *called)
}
