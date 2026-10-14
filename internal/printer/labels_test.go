package printer

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
)

// plainLabels returns the label of each "Label: value" line of plain output,
// in order, without the indent.
func plainLabels(output string) []string {
	var labels []string
	for _, line := range strings.Split(output, "\n") {
		if label, _, found := strings.Cut(strings.TrimSpace(line), ": "); found {
			labels = append(labels, label)
		}
	}
	return labels
}

// upper returns each label in upper case, as a table header shows it.
func upper(labels []string) []string {
	headers := make([]string, len(labels))
	for i, label := range labels {
		headers[i] = strings.ToUpper(label)
	}
	return headers
}

func TestDeliverabilityTableHeadersAreThePlainLabels(t *testing.T) {
	stats := &responses.DeliverabilityStatisticsResponse{Data: []responses.DeliverabilityStatistics{{
		FromTimestamp: styleTime, ToTimestamp: styleTime,
		ReceptionCount: 4, DeliveredCount: 2, DeferredCount: 1, BouncedCount: 1, FailedCount: 1,
		SuppressedCount: 1, OpenedCount: 1, ClickedCount: 1,
	}}}
	render := func(h ResponseHandler) error { return h.HandleDeliverabilityStats(stats, StatsConfig{}) }

	labels := plainLabels(renderWith(t, "plain", render))
	assert.Equal(t, []string{"Time Period", "Reception", "Delivered", "Deferred", "Bounced", "Failed",
		"Suppressed", "Opened", "Clicked", "Delivery Rate", "Open Rate"}, labels)
	assert.Equal(t, upper(labels), tableCells(renderWith(t, "table", render))[0])
}

func TestDeliveryTimeTableHeadersAreThePlainLabels(t *testing.T) {
	render := func(h ResponseHandler) error { return h.HandleDeliveryTimeStats(styleDeliveryTimes(), StatsConfig{}) }

	assert.Equal(t, []string{"Time Period", "Delivered", "Average Delivery Time"},
		plainLabels(renderWith(t, "plain", render))[:3])
	assert.Equal(t, []string{"TIME PERIOD", "DELIVERED", "AVERAGE DELIVERY TIME"},
		tableCells(renderWith(t, "table", render))[0][:3])
}

func TestSubAccountUsageTableHeadersAreThePlainLabels(t *testing.T) {
	render := func(h ResponseHandler) error {
		return h.HandleSubAccountUsage(mocks.NewSubAccountUsage(0), SingleConfig{})
	}

	plain := renderWith(t, "plain", render)
	assert.Contains(t, plain, "\n  Account ID: ")
	assert.Contains(t, plain, "\n  Reception: ")
	assert.Contains(t, plain, "\n  Allocated Cost: ")
	assert.Equal(t, []string{"ACCOUNT", "ACCOUNT ID", "RECEPTION", "ALLOCATED COST"},
		tableCells(renderWith(t, "table", render))[0])
}

func TestAuthStatusRetentionHasTheSameRowsInTableAndPlain(t *testing.T) {
	metadata, data := int32(30), int32(1)
	status := &AuthStatus{Profile: "default", Account: &responses.Account{
		Name: "Acme", MessageMetadataRetention: &metadata, MessageDataRetention: &data,
	}}
	render := func(h ResponseHandler) error { return h.HandleAuthStatus(status) }

	assert.Contains(t, renderWith(t, "plain", render), "\nData Retention:\n  Message Metadata: 30 days\n  Message Data: 1 day\n")
	cells := tableCells(renderWith(t, "table", render))
	assert.Contains(t, cells, []string{"Message Metadata", "30 days"})
	assert.Contains(t, cells, []string{"Message Data", "1 day"})
	assert.NotContains(t, cells, []string{"TYPE", "DAYS"})
}

func TestSMTPSettingsHaveTheSameRowsInTableAndPlain(t *testing.T) {
	credential := styleSMTP()
	render := func(h ResponseHandler) error { return h.HandleSingleSMTP(&credential, SingleConfig{}) }

	plain := renderWith(t, "plain", render)
	_, settings, _ := strings.Cut(plain, "\nSMTP Settings:\n")
	labels := plainLabels(settings)
	assert.Equal(t, []string{"Server", "Port", "Username", "Password", "Authentication"}, labels)

	table := renderWith(t, "table", render)
	_, settingsTable, _ := strings.Cut(table, "\nSMTP Settings:\n")
	var rowLabels []string
	for _, row := range tableCells(settingsTable)[1:] {
		rowLabels = append(rowLabels, row[0])
	}
	assert.Equal(t, labels, rowLabels)
}
