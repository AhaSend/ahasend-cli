package stats

import (
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// statsPeriod is the time range of the periods the output tests return.
const statsPeriod = `"from_timestamp":"2026-01-01T00:00:00Z","to_timestamp":"2026-01-02T00:00:00Z"`

// TestStatsOutputFormats runs each stats command in table, plain and CSV
// output and checks the values of the period the API returned.
func TestStatsOutputFormats(t *testing.T) {
	tests := []struct {
		name    string
		command func() *cobra.Command
		method  string
		result  func(t *testing.T) any
		want    map[string][]string
	}{
		{
			name: "deliverability", command: NewDeliverabilityCommand, method: "GetDeliverabilityStatistics",
			result: func(t *testing.T) any {
				return mocks.APIResponse[responses.DeliverabilityStatisticsResponse](t, `{"object":"list","data":[{`+statsPeriod+
					`,"reception_count":4,"delivered_count":2,"opened_count":1}]}`)
			},
			want: map[string][]string{
				"table": {"RECEPTION", "DELIVERED", "50.00%"},
				"plain": {"  Reception: 4\n", "  Delivered: 2\n", "  Delivery Rate: 50.00%\n", "  Open Rate: 50.00%\n"},
				"csv":   {"2026-01-01T00:00:00Z,2026-01-02T00:00:00Z,4,2,0,0,0,0,1,0,50.00,50.00\n"},
			},
		},
		{
			name: "bounces", command: NewBouncesCommand, method: "GetBounceStatistics",
			result: func(t *testing.T) any {
				return mocks.APIResponse[responses.BounceStatisticsResponse](t, `{"object":"list","data":[{`+statsPeriod+
					`,"bounces":[{"classification":"BadMailbox","count":3},{"classification":"SpamBlock","count":1}]}]}`)
			},
			want: map[string][]string{
				"table": {"BadMailbox", "75.00%", "2 types"},
				"plain": {"  Total Bounces: 4\n", "    BadMailbox: 3 (75.00%)\n", "    SpamBlock: 1 (25.00%)\n"},
				"csv":   {"2026-01-01T00:00:00Z,2026-01-02T00:00:00Z,BadMailbox,3,75.00\n"},
			},
		},
		{
			name: "delivery-time", command: NewDeliveryTimeCommand, method: "GetDeliveryTimeStatistics",
			result: func(t *testing.T) any {
				return mocks.APIResponse[responses.DeliveryTimeStatisticsResponse](t, `{"object":"list","data":[{`+statsPeriod+
					`,"delivered_count":2,"avg_delivery_time":1.5,"delivery_times":[{"recipient_domain":"gmail.com","delivery_time":2.126,"count":2}]}]}`)
			},
			want: map[string][]string{
				"table": {"AVERAGE DELIVERY TIME", "1.50s", "│ RECIPIENT DOMAIN │ DELIVERED │ DELIVERY TIME │", "│ gmail.com        │ 2         │ 2.13s         │"},
				"plain": {"  Delivered: 2\n", "  Average Delivery Time: 1.50s\n", "    gmail.com: 2.13s (2 delivered)\n"},
				"csv":   {"2026-01-01T00:00:00Z,2026-01-02T00:00:00Z,2,1.5,gmail.com,2,2.126\n"},
			},
		},
	}
	for _, tt := range tests {
		for format, want := range tt.want {
			t.Run(tt.name+"/"+format, func(t *testing.T) {
				mockClient := &mocks.MockClient{}
				mockClient.On(tt.method, mock.Anything).Return(tt.result(t), nil)

				stdout, err := testutil.RunWithClient(t, mockClient, tt.command(), format)

				require.NoError(t, err)
				for _, text := range want {
					assert.Contains(t, stdout, text)
				}
			})
		}
	}
}
