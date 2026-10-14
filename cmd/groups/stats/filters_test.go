package stats

import (
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// deliverabilityParams runs stats deliverability with args and returns the
// params it sent.
func deliverabilityParams(t *testing.T, args ...string) requests.GetDeliverabilityStatisticsParams {
	t.Helper()
	var sent requests.GetDeliverabilityStatisticsParams
	mockClient := &mocks.MockClient{}
	mockClient.On("GetDeliverabilityStatistics", mock.Anything).Run(func(arguments mock.Arguments) {
		sent = arguments.Get(0).(requests.GetDeliverabilityStatisticsParams)
	}).Return(mocks.APIResponse[responses.DeliverabilityStatisticsResponse](t, `{"object":"list","data":[]}`), nil)
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewDeliverabilityCommand(), "plain", "", args...)
	require.NoError(t, err)
	return sent
}

func TestStatsSendsRepeatedTagsAndRecipientDomainsJoined(t *testing.T) {
	sent := deliverabilityParams(t, "--tag", "a", "--tag", "b,c", "--recipient-domain", "gmail.com", "--recipient-domain", "outlook.com")

	assert.Equal(t, "a,b,c", *sent.Tags)
	assert.Equal(t, "gmail.com,outlook.com", *sent.RecipientDomains)
	assert.Nil(t, sent.SenderDomain)
}

// TestStatsSendsRepeatedSenderDomainsJoined checks that --sender-domain
// takes several domains, which the API reads as a comma-separated list.
func TestStatsSendsRepeatedSenderDomainsJoined(t *testing.T) {
	sent := deliverabilityParams(t, "--sender-domain", "example.com", "--sender-domain", "example.org")

	require.NotNil(t, sent.SenderDomain)
	assert.Equal(t, "example.com,example.org", *sent.SenderDomain)
}

func TestStatsDefaultsToTheLastSevenDays(t *testing.T) {
	sent := deliverabilityParams(t)

	require.NotNil(t, sent.FromTime)
	assert.WithinDuration(t, time.Now().AddDate(0, 0, -7), *sent.FromTime, time.Minute)
	assert.Nil(t, sent.ToTime, "the API ends the range now")
}

func TestStatsEmptyFromTimeLeavesTheRangeToTheAPI(t *testing.T) {
	assert.Nil(t, deliverabilityParams(t, "--from-time", "").FromTime)
}

func TestStatsRejectBadFiltersBeforeAuthenticating(t *testing.T) {
	commands := []func() *cobra.Command{NewDeliverabilityCommand, NewBouncesCommand, NewDeliveryTimeCommand}
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--from-time", "soon"}, `invalid --from-time value "soon": use RFC 3339 or a relative time such as 24h or 7d`},
		{[]string{"--to-time", "soon"}, `invalid --to-time value "soon": use RFC 3339 or a relative time such as 24h or 7d`},
		{[]string{"--group-by", "year"}, "--group-by must be one of: hour, day, week, month"},
		{[]string{"--tag", ""}, "--tag cannot be empty"},
		{[]string{"--recipient-domain", ""}, "--recipient-domain cannot be empty"},
		{[]string{"--sender-domain", ""}, "--sender-domain cannot be empty"},
	}
	for _, command := range commands {
		for _, tt := range tests {
			t.Run(command().Name()+" "+tt.message, func(t *testing.T) {
				called := testutil.RefuseAuth(t)

				_, _, err := testutil.RunCommand(t, command(), "plain", "", tt.args...)

				require.Error(t, err)
				assert.Equal(t, tt.message, err.Error())
				assert.Equal(t, 4, clierrors.GetExitCode(err))
				assert.False(t, *called)
			})
		}
	}
}

func TestStatsNoEffectFlagsAreHiddenAndDeprecated(t *testing.T) {
	flags := map[string][]string{
		"deliverability": {"chart", "raw", "show-totals"},
		"bounces":        {"raw", "classification", "trends", "show-domains", "show-totals"},
		"delivery-time":  {"raw", "show-totals"},
	}
	for _, command := range []*cobra.Command{NewDeliverabilityCommand(), NewBouncesCommand(), NewDeliveryTimeCommand()} {
		for _, name := range flags[command.Name()] {
			flag := command.Flags().Lookup(name)
			require.NotNil(t, flag, "%s --%s", command.Name(), name)
			assert.True(t, flag.Hidden, "%s --%s", command.Name(), name)
			assert.Equal(t, "it has no effect", flag.Deprecated, "%s --%s", command.Name(), name)
			assert.NotContains(t, command.Long+command.Example, "--"+name+" ", "%s --%s", command.Name(), name)
		}
	}
}
