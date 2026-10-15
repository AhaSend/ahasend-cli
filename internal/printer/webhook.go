package printer

import (
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// The fields of the views of a webhook, which the table, plain and CSV output
// of each view show in this order, except a table of webhooks, which shows
// webhookTableColumns.
var (
	// webhookListFields are the fields of each webhook of a list, without
	// the secret.
	webhookListFields = slices.Concat([]string{
		"name", "id", "url", "enabled", "events", "scope", "domains", "created_at", "updated_at",
	}, callFields)
	// webhookTableColumns are the fields a table of webhooks shows.
	webhookTableColumns = []string{"name", "id", "url", "enabled", "events"}
	// webhookDetailFields are the fields of a webhook's detail view, without
	// the secret, which the API returns only when it creates the webhook.
	webhookDetailFields = slices.Concat([]string{
		"name", "id", "url", "enabled", "events", "scope", "domains", "created_at", "updated_at",
	}, callFields)
	// createdWebhookFields are the fields of a new webhook, which has no
	// calls yet.
	createdWebhookFields = []string{
		"name", "id", "url", "enabled", "events", "secret", "scope", "domains", "created_at", "updated_at",
	}
)

// webhookHeaders names the table column or row of each webhook field.
var webhookHeaders = withCallHeaders(map[string]string{
	"name":       "Name",
	"id":         "ID",
	"url":        "URL",
	"enabled":    "Enabled",
	"events":     "Events",
	"secret":     "Secret",
	"scope":      "Scope",
	"domains":    "Domains",
	"created_at": "Created",
	"updated_at": "Updated",
})

func (h *jsonHandler) HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error {
	return h.printAPIResponse(webhook)
}

func (h *jsonHandler) HandleCreateWebhook(webhook *responses.Webhook, config ResultConfig) error {
	return h.printAPIResponse(webhook)
}

func (h *jsonHandler) HandleUpdateWebhook(webhook *responses.Webhook, config ResultConfig) error {
	return h.printAPIResponse(webhook)
}

func (h *tableHandler) HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error {
	return writeTableList(h, config, pageOf(response), "webhooks", webhookTableColumns, webhookHeaders, func(webhook *responses.Webhook) map[string]string {
		return webhookDisplayFields(webhook, "")
	})
}

func (h *tableHandler) HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error {
	if webhook == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(webhookRows(webhookDetailFields, webhook, ""))
	return nil
}

// HandleCreateWebhook shows the webhook's secret, which the API shows only on
// creation.
func (h *tableHandler) HandleCreateWebhook(webhook *responses.Webhook, config ResultConfig) error {
	if webhook == nil {
		return h.HandleEmpty("No webhook data received")
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(webhookRows(createdWebhookFields, webhook, webhook.Secret))
	return writeWebhookSecretNotice(h.writer, webhook)
}

func (h *tableHandler) HandleUpdateWebhook(webhook *responses.Webhook, config ResultConfig) error {
	if webhook == nil {
		return h.HandleEmpty("No webhook data received")
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(webhookRows(webhookDetailFields, webhook, ""))
	return nil
}

func (h *plainHandler) HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error {
	return writePlainList(h, config, pageOf(response), "webhooks", func(webhook *responses.Webhook) error {
		return writeLabelLines(h.writer, "", webhookRows(webhookListFields, webhook, ""))
	})
}

func (h *plainHandler) HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error {
	if webhook == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", webhookRows(webhookDetailFields, webhook, ""))
}

// HandleCreateWebhook shows the webhook's secret, which the API shows only on
// creation.
func (h *plainHandler) HandleCreateWebhook(webhook *responses.Webhook, config ResultConfig) error {
	if webhook == nil {
		return h.HandleEmpty("No webhook data received")
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", webhookRows(createdWebhookFields, webhook, webhook.Secret)); err != nil {
		return err
	}
	return writeWebhookSecretNotice(h.writer, webhook)
}

func (h *plainHandler) HandleUpdateWebhook(webhook *responses.Webhook, config ResultConfig) error {
	if webhook == nil {
		return h.HandleEmpty("No webhook data received")
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", webhookRows(webhookDetailFields, webhook, ""))
}

func (h *csvHandler) HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error {
	return writeCSVList(h, pageOf(response), webhookListFields, func(webhook *responses.Webhook) map[string]string {
		return webhookCSVFields(webhook, "")
	})
}

func (h *csvHandler) HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error {
	if webhook == nil {
		return nil // No CSV output for empty data
	}
	return h.writeFieldMaps(webhookDetailFields, webhookCSVFields(webhook, ""))
}

// HandleCreateWebhook writes the webhook with its secret, which the API shows
// only on creation.
func (h *csvHandler) HandleCreateWebhook(webhook *responses.Webhook, config ResultConfig) error {
	if webhook == nil {
		return nil // No CSV output for empty data
	}
	return h.writeFieldMaps(createdWebhookFields, webhookCSVFields(webhook, webhook.Secret))
}

func (h *csvHandler) HandleUpdateWebhook(webhook *responses.Webhook, config ResultConfig) error {
	if webhook == nil {
		return nil // No CSV output for empty data
	}
	return h.writeFieldMaps(webhookDetailFields, webhookCSVFields(webhook, ""))
}

// webhookCSVFields maps a webhook to its csv values, with secret as the value
// of the secret column.
func webhookCSVFields(webhook *responses.Webhook, secret string) map[string]string {
	values := callValues(webhook.SuccessCount, webhook.ErrorCount, webhook.ErrorsSinceLastSuccess,
		formatOptionalCSVTime(webhook.LastRequestAt))
	maps.Copy(values, map[string]string{
		"name":       webhook.Name,
		"id":         formatUUID(webhook.ID),
		"url":        webhook.URL,
		"enabled":    formatBool(webhook.Enabled),
		"events":     formatWebhookEvents(webhook),
		"secret":     secret,
		"scope":      webhook.Scope,
		"domains":    formatStringSlice(webhook.Domains),
		"created_at": formatCSVTime(webhook.CreatedAt),
		"updated_at": formatCSVTime(webhook.UpdatedAt),
	})
	return values
}

// webhookDisplayFields maps each webhook field to its table value, with
// secret as the value of the secret field.
func webhookDisplayFields(webhook *responses.Webhook, secret string) map[string]string {
	values := callValues(webhook.SuccessCount, webhook.ErrorCount, webhook.ErrorsSinceLastSuccess,
		formatOptionalTime(webhook.LastRequestAt))
	maps.Copy(values, map[string]string{
		"name":       webhook.Name,
		"id":         formatUUID(webhook.ID),
		"url":        webhook.URL,
		"enabled":    formatBooleanStatus(webhook.Enabled),
		"events":     formatWebhookEvents(webhook),
		"secret":     secret,
		"scope":      webhook.Scope,
		"domains":    formatDomains(webhook.Domains),
		"created_at": formatTime(webhook.CreatedAt),
		"updated_at": formatTime(webhook.UpdatedAt),
	})
	return values
}

// webhookRows returns the rows of fields of a webhook, with secret as the
// value of the secret row.
func webhookRows(fields []string, webhook *responses.Webhook, secret string) [][]string {
	return fieldRows(fields, webhookHeaders, webhookDisplayFields(webhook, secret))
}

// writeWebhookSecretNotice tells to save the secret of a new webhook, when
// the API returned one.
func writeWebhookSecretNotice(w io.Writer, webhook *responses.Webhook) error {
	if webhook.Secret == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "\n%s\n", webhookSecretNotice)
	return err
}

// callFields are the call counters of a webhook or a route: how many calls
// succeeded and failed, the errors since the last success, and when it was
// last called.
var callFields = []string{"success_count", "error_count", "errors_since_last_success", "last_request_at"}

// withCallHeaders adds the header of each call counter to headers.
func withCallHeaders(headers map[string]string) map[string]string {
	maps.Copy(headers, map[string]string{
		"success_count":             "Successful Calls",
		"error_count":               "Failed Calls",
		"errors_since_last_success": "Errors Since Last Success",
		"last_request_at":           "Last Called",
	})
	return headers
}

// callValues maps the call counters of a webhook or a route to their values,
// with lastCalledAt as the formatted time of the last call.
func callValues(successCount, errorCount uint64, errorsSinceLastSuccess int, lastCalledAt string) map[string]string {
	return map[string]string{
		"success_count":             formatUint64(successCount),
		"error_count":               formatUint64(errorCount),
		"errors_since_last_success": formatInt(errorsSinceLastSuccess),
		"last_request_at":           lastCalledAt,
	}
}
