package printer

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
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

// bouncedMessage is a message delivered and then bounced out of band: its
// last delivery attempt names the bounce report message.
func bouncedMessage() responses.Message {
	message := deliveredMessage()
	message.IsBounceNotification = false
	report := uuid.MustParse(styleLongID)
	message.DeliveryAttempts = append(message.DeliveryAttempts,
		responses.DeliveryEvent{Time: styleTime, Status: "bounced", Log: "550 5.1.1 unknown user", ReportMessageID: &report})
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

func TestMessageTableShowsTheReportMessageIDOfABounce(t *testing.T) {
	useLocalTimeZone(t)
	cells := tableCells(renderMessage(t, "table", bouncedMessage()))

	assert.Contains(t, cells, []string{"TIME", "STATUS", "LOG", "REPORT MESSAGE ID"})
	assert.Contains(t, cells, []string{"2026-09-10 10:30:00 UTC+2", "delivered", "250 OK", "-"})
	assert.Contains(t, cells, []string{"2026-09-10 10:30:00 UTC+2", "bounced", "550 5.1.1 unknown user", styleLongID})
}

func TestMessagePlainShowsTheReportMessageIDOfABounce(t *testing.T) {
	useLocalTimeZone(t)
	out := renderMessage(t, "plain", bouncedMessage())

	assert.Contains(t, out, "  Attempt 2:\n    Time: 2026-09-10 10:30:00 UTC+2\n    Status: delivered\n    Log: 250 OK\n"+
		"  Attempt 3:\n    Time: 2026-09-10 10:30:00 UTC+2\n    Status: bounced\n    Log: 550 5.1.1 unknown user\n"+
		"    Report Message ID: "+styleLongID+"\n")
	assert.Equal(t, 1, strings.Count(out, "Report Message ID"))
}

func TestMessageCSVHasTheReportMessageIDInTheDeliveryAttempts(t *testing.T) {
	_, rows := readCSV(t, renderMessage(t, "csv", bouncedMessage()))

	assert.Contains(t, rows[0]["delivery_attempts"], `"report_message_id":"`+styleLongID+`"`)
}

func TestMessageWithoutAReportMessageIDShowsNoReportColumn(t *testing.T) {
	assert.NotContains(t, renderMessage(t, "table", deliveredMessage()), "REPORT MESSAGE ID")
	assert.NotContains(t, renderMessage(t, "plain", deliveredMessage()), "Report Message ID")
}

// sentMessage renders a one-request send of one message with id and
// messageIDHeader, nil for one the API did not return.
func sentMessage(t *testing.T, format string, id, messageIDHeader *string) string {
	t.Helper()
	return renderWith(t, format, func(h ResponseHandler) error {
		return h.HandleCreateMessage(&responses.CreateMessageResponse{Data: []responses.CreateSingleMessageResponse{{
			Recipient: common.Recipient{Email: "to@example.com"}, ID: id, MessageIDHeader: messageIDHeader, Status: "queued",
		}}}, ResultConfig{SuccessMessage: "Message sent successfully"})
	})
}

func TestSentMessageShowsTheMessageIDHeaderAfterTheID(t *testing.T) {
	id, header := styleID.String(), "<"+styleID.String()+"@example.com>"

	cells := tableCells(sentMessage(t, "table", &id, &header))
	assert.Equal(t, [][]string{
		{"#", "RECIPIENT", "ID", "MESSAGE-ID", "STATUS", "ERROR"},
		{"1", "to@example.com", id, header, "queued", "-"},
	}, cells)

	assert.Contains(t, sentMessage(t, "plain", &id, &header),
		"\nMessage 1:\n  Recipient: to@example.com\n  ID: "+id+"\n  Message-ID: "+header+"\n  Status: queued\n")

	headers, rows := readCSV(t, sentMessage(t, "csv", &id, &header))
	assert.Equal(t, []string{"recipient", "id", "message_id_header", "status", "error"}, headers)
	assert.Equal(t, header, rows[0]["message_id_header"])
}

func TestSentMessageWithoutAMessageIDHeaderShowsItEmpty(t *testing.T) {
	assert.Equal(t, []string{"1", "to@example.com", "-", "-", "queued", "-"}, tableCells(sentMessage(t, "table", nil, nil))[1])
	assert.NotContains(t, sentMessage(t, "plain", nil, nil), "Message-ID")
	_, rows := readCSV(t, sentMessage(t, "csv", nil, nil))
	assert.Equal(t, "", rows[0]["message_id_header"])
}
