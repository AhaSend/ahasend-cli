package printer

import (
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deliveredMessage is a message sent and delivered at the second attempt.
func deliveredMessage() responses.Message {
	message := styleMessage()
	sent := styleTime
	message.SentAt = &sent
	message.IsBounceNotification = true
	message.DeliveryAttempts = []responses.DeliveryEvent{
		{Time: styleTime, Status: "deferred", Log: "421 try again\nlater"},
		{Time: styleTime, Status: "delivered", Log: "250 OK"},
	}
	return message
}

func renderMessage(t *testing.T, format string, message responses.Message) string {
	t.Helper()
	return renderWith(t, format, func(h ResponseHandler) error { return h.HandleSingleMessage(&message, SingleConfig{}) })
}

func TestMessageTableShowsSentAtBounceNotificationAndDeliveryAttempts(t *testing.T) {
	useLocalTimeZone(t)
	out := renderMessage(t, "table", deliveredMessage())

	assert.Regexp(t, `│ Sent +│ 2026-09-10 10:30:00 UTC\+2 +│`, out)
	assert.Regexp(t, `│ Bounce Notification +│ Yes +│`, out)
	assert.Contains(t, out, "\nDelivery Attempts:\n")
	assert.Regexp(t, `│ +TIME +│ +STATUS +│ +LOG +│`, out)
	assert.Regexp(t, `│ 2026-09-10 10:30:00 UTC\+2 │ deferred +│ 421 try again │`, out)
	assert.Regexp(t, `│ +│ +│ later +│`, out)
	assert.Regexp(t, `│ 2026-09-10 10:30:00 UTC\+2 │ delivered +│ 250 OK +│`, out)
}

func TestMessagePlainShowsSentAtBounceNotificationAndDeliveryAttempts(t *testing.T) {
	useLocalTimeZone(t)
	out := renderMessage(t, "plain", deliveredMessage())

	assert.Contains(t, out, "Sent: 2026-09-10 10:30:00 UTC+2\n")
	assert.Contains(t, out, "Bounce Notification: Yes\n")
	assert.Contains(t, out, "Delivery Attempts:\n"+
		"  Attempt 1:\n    Time: 2026-09-10 10:30:00 UTC+2\n    Status: deferred\n    Log: 421 try again\n         later\n"+
		"  Attempt 2:\n    Time: 2026-09-10 10:30:00 UTC+2\n    Status: delivered\n    Log: 250 OK\n")
}

func TestMessageCSVHasTheDeliveryAttemptsAsJSON(t *testing.T) {
	_, rows := readCSV(t, renderMessage(t, "csv", deliveredMessage()))

	require.Len(t, rows, 1)
	assert.Equal(t, "2026-09-10T08:30:00Z", rows[0]["sent_at"])
	assert.Equal(t, "true", rows[0]["is_bounce_notification"])
	assert.Equal(t, `[{"time":"2026-09-10T08:30:00Z","log":"421 try again\nlater","status":"deferred"},`+
		`{"time":"2026-09-10T08:30:00Z","log":"250 OK","status":"delivered"}]`, rows[0]["delivery_attempts"])
}

func TestMessageWithoutDeliveryAttemptsShowsNone(t *testing.T) {
	message := styleMessage()

	assert.NotContains(t, renderMessage(t, "table", message), "Delivery Attempts")
	assert.NotContains(t, renderMessage(t, "plain", message), "Delivery Attempts")
	_, rows := readCSV(t, renderMessage(t, "csv", message))
	assert.Equal(t, "", rows[0]["delivery_attempts"])
}
