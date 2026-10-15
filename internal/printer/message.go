package printer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// The fields of the views of a message, which the table, plain and CSV output
// of each view show in this order, except a table of messages, which shows
// the table columns of its list view.
var (
	// messageListFields are the fields of each message of a list.
	messageListFields = []string{
		"subject", "id", "sender", "recipient", "status", "created_at", "delivered_at", "opens", "clicks", "template_id",
	}
	// messageTableColumns are the fields a table of messages shows.
	messageTableColumns = []string{"subject", "id", "sender", "recipient", "status", "created_at"}
	// detailedMessageListFields are the fields of each message of a list
	// with details.
	detailedMessageListFields = slices.Concat(messageListFields, []string{
		"message_id", "direction", "domain_id", "attempts", "tags", "bounce_class", "retain_until",
	})
	// detailedMessageTableColumns are the fields a table of messages with
	// details shows.
	detailedMessageTableColumns = slices.Concat(messageTableColumns, []string{"direction", "attempts", "bounce_class"})
	// messageDetailFields are the fields of a message's detail view. The view
	// ends with the message's delivery attempts and content, which table and
	// plain output show in their own ways and CSV output as the
	// delivery_attempts, content_size and content columns.
	messageDetailFields = []string{
		"subject", "id", "account_id", "sender", "recipient", "status", "direction",
		"created_at", "updated_at", "sent_at", "delivered_at", "opens", "clicks", "attempts", "bounce_class",
		"is_bounce_notification", "message_id", "domain_id", "template_id", "tags", "retain_until",
	}
)

// messageHeaders names the table column or row of each message field.
var messageHeaders = map[string]string{
	"id":           "ID",
	"account_id":   "Account ID",
	"sender":       "From",
	"recipient":    "To",
	"subject":      "Subject",
	"status":       "Status",
	"created_at":   "Created",
	"delivered_at": "Delivered",
	"opens":        "Opens",
	"clicks":       "Clicks",
	"message_id":   "Message ID",
	"direction":    "Direction",
	"domain_id":    "Domain ID",
	"attempts":     "Attempts",
	"tags":         "Tags",
	"bounce_class": "Bounce Class",
	"retain_until": "Retain Until",
	"template_id":  "Template ID",
	"updated_at":   "Updated",
	"sent_at":      "Sent",

	"is_bounce_notification": "Bounce Notification",
}

// deliveryAttemptHeaders name the columns of a table of a message's delivery
// attempts, and the rows of each attempt in plain output.
var deliveryAttemptHeaders = []any{"Time", "Status", "Log"}

func (h *jsonHandler) HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleMessage(message *responses.Message, config SingleConfig) error {
	return h.printAPIResponse(message)
}

func (h *jsonHandler) HandleCreateMessage(response *responses.CreateMessageResponse, config ResultConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error {
	_, columns := messageListView(config)
	return writeTableList(h, config, pageOf(response), "messages", columns, messageHeaders, messageDisplayFields)
}

func (h *tableHandler) HandleSingleMessage(message *responses.Message, config SingleConfig) error {
	if message == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}

	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}

	rows := messageDetailRows(message)

	// The content shows as a one-line preview of at most 100 characters
	if content := formatOptionalString(message.Content); content != "" {
		preview := content
		if runes := []rune(preview); len(runes) > 100 {
			preview = string(runes[:97]) + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		preview = strings.ReplaceAll(preview, "\r", "")
		rows = append(rows,
			[]string{"Content Size", fmt.Sprintf("%d bytes", len(content))},
			[]string{"Content", preview},
		)
	}

	h.renderFieldTable(rows)

	if len(message.DeliveryAttempts) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(h.writer, "\nDelivery Attempts:\n"); err != nil {
		return err
	}
	h.renderTable(deliveryAttemptHeaders, deliveryAttemptRows(message)...)
	return nil
}

func (h *tableHandler) HandleCreateMessage(response *responses.CreateMessageResponse, config ResultConfig) error {
	if response == nil {
		return h.HandleEmpty("No message data received")
	}

	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	rows := make([][]string, len(response.Data))
	for i := range response.Data {
		rows[i] = append([]string{formatInt(i + 1)}, fieldValues(sentMessageFields, sentMessageValues(&response.Data[i]))...)
	}
	h.renderTable(append([]any{"#"}, fieldHeaders(sentMessageFields, sentMessageHeaders)...), rows...)
	return nil
}

func (h *plainHandler) HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error {
	fields, _ := messageListView(config)
	return writePlainList(h, config, pageOf(response), "messages", func(message *responses.Message) error {
		return h.writeFieldLines(fields, messageHeaders, messageDisplayFields(message))
	})
}

func (h *plainHandler) HandleSingleMessage(message *responses.Message, config SingleConfig) error {
	if message == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}

	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", messageDetailRows(message)); err != nil {
		return err
	}
	if len(message.DeliveryAttempts) > 0 {
		if _, err := fmt.Fprintln(h.writer, "Delivery Attempts:"); err != nil {
			return err
		}
		for i, row := range deliveryAttemptRows(message) {
			if _, err := fmt.Fprintf(h.writer, "  Attempt %d:\n", i+1); err != nil {
				return err
			}
			labels := make([][]string, len(row))
			for j, value := range row {
				labels[j] = []string{deliveryAttemptHeaders[j].(string), value}
			}
			if err := writeLabelLines(h.writer, "    ", labels); err != nil {
				return err
			}
		}
	}

	// Plain output shows the full content
	content := formatOptionalString(message.Content)
	if content == "" {
		return nil
	}
	_, err := fmt.Fprintf(h.writer, "Content Size: %d bytes\nContent:\n%s\n", len(content), content)
	return err
}

func (h *plainHandler) HandleCreateMessage(response *responses.CreateMessageResponse, config ResultConfig) error {
	if response == nil {
		return h.HandleEmpty("No message data received")
	}

	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	for i, messageData := range response.Data {
		if _, err := fmt.Fprintf(h.writer, "\nMessage %d:\n", i+1); err != nil {
			return err
		}
		rows := fieldRows(sentMessageFields, sentMessageHeaders, sentMessageValues(&messageData))
		if err := writeLabelLines(h.writer, "  ", rows); err != nil {
			return err
		}
	}
	return nil
}

func (h *csvHandler) HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error {
	fields, _ := messageListView(config)
	return writeCSVList(h, pageOf(response), fields, messageCSVFields)
}

func (h *csvHandler) HandleSingleMessage(message *responses.Message, config SingleConfig) error {
	if message == nil {
		return nil // No CSV output for empty data
	}
	// The delivery attempts are a JSON array in one column, as the API sent
	// them
	fields := messageCSVFields(message)
	attempts, err := stableJSON(message.DeliveryAttempts)
	if err != nil {
		return err
	}
	fields["delivery_attempts"] = attempts
	columns := slices.Concat(messageDetailFields, []string{"delivery_attempts", "content_size", "content"})
	return h.writeFieldMaps(columns, fields)
}

func messageCSVFields(message *responses.Message) map[string]string {
	content := formatOptionalString(message.Content)
	return map[string]string{
		"id":           formatUUID(message.ID),
		"account_id":   formatUUID(message.AccountID),
		"sender":       message.Sender,
		"recipient":    message.Recipient,
		"subject":      message.Subject,
		"status":       message.Status,
		"direction":    message.Direction,
		"created_at":   formatCSVTime(message.CreatedAt),
		"updated_at":   formatCSVTime(message.UpdatedAt),
		"sent_at":      formatOptionalCSVTime(message.SentAt),
		"delivered_at": formatOptionalCSVTime(message.DeliveredAt),
		"opens":        formatInt(int(message.OpenCount)),
		"clicks":       formatInt(int(message.ClickCount)),
		"attempts":     formatInt(int(message.NumAttempts)),
		"bounce_class": formatOptionalString(message.BounceClassification),
		"message_id":   message.MessageID,
		"domain_id":    formatUUID(message.DomainID),
		"template_id":  formatOptionalUUID(message.TemplateID),
		"tags":         formatTags(message.Tags),
		"retain_until": formatCSVTime(message.RetainUntil),
		"content_size": formatInt(len(content)),

		"is_bounce_notification": formatBool(message.IsBounceNotification),
		"content":                content,
	}
}

func (h *csvHandler) HandleCreateMessage(response *responses.CreateMessageResponse, config ResultConfig) error {
	if response == nil {
		return nil
	}

	rows := make([]map[string]string, len(response.Data))
	for i := range response.Data {
		rows[i] = sentMessageValues(&response.Data[i])
	}
	return h.writeFieldMaps(sentMessageFields, rows...)
}

// messageDisplayFields maps each message field, except its content, to its
// table and plain value.
func messageDisplayFields(message *responses.Message) map[string]string {
	return map[string]string{
		"id":           formatUUID(message.ID),
		"account_id":   formatUUID(message.AccountID),
		"updated_at":   formatTime(message.UpdatedAt),
		"sent_at":      formatOptionalTime(message.SentAt),
		"sender":       message.Sender,
		"recipient":    message.Recipient,
		"subject":      message.Subject,
		"status":       message.Status,
		"created_at":   formatTime(message.CreatedAt),
		"delivered_at": formatOptionalTime(message.DeliveredAt),
		"opens":        formatInt(int(message.OpenCount)),
		"clicks":       formatInt(int(message.ClickCount)),
		"message_id":   message.MessageID,
		"direction":    message.Direction,
		"domain_id":    formatUUID(message.DomainID),
		"attempts":     formatInt(int(message.NumAttempts)),
		"tags":         formatTags(message.Tags),
		"bounce_class": formatOptionalString(message.BounceClassification),
		"retain_until": formatTime(message.RetainUntil),
		"template_id":  formatOptionalUUID(message.TemplateID),

		"is_bounce_notification": formatBooleanStatus(message.IsBounceNotification),
	}
}

// deliveryAttemptRows returns the time, status and log of each delivery
// attempt of message, the columns of deliveryAttemptHeaders.
func deliveryAttemptRows(message *responses.Message) [][]string {
	rows := make([][]string, len(message.DeliveryAttempts))
	for i, attempt := range message.DeliveryAttempts {
		rows[i] = []string{formatTime(attempt.Time), attempt.Status, attempt.Log}
	}
	return rows
}

// messageDetailRows returns the rows of a message's detail view, without its
// content.
func messageDetailRows(message *responses.Message) [][]string {
	return fieldRows(messageDetailFields, messageHeaders, messageDisplayFields(message))
}

// messageListView returns the fields and the table columns of each message
// of a list: the detailed ones when config asks for them.
func messageListView(config ListConfig) (fields, columns []string) {
	if config.Detailed {
		return detailedMessageListFields, detailedMessageTableColumns
	}
	return messageListFields, messageTableColumns
}

// sentMessageFields are the fields of each message of a send, which the
// table, plain and CSV output show in this order.
var sentMessageFields = []string{"recipient", "id", "status", "error"}

// sentMessageHeaders names the table column or plain row of each field of a
// message of a send.
var sentMessageHeaders = map[string]string{
	"recipient": "Recipient",
	"id":        "ID",
	"status":    "Status",
	"error":     "Error",
}

// sentMessageValues maps each field of one message of a send to its value.
func sentMessageValues(message *responses.CreateSingleMessageResponse) map[string]string {
	return map[string]string{
		"recipient": message.Recipient.Email,
		"id":        formatOptionalString(message.ID),
		"status":    message.Status,
		"error":     formatOptionalString(message.Error),
	}
}
