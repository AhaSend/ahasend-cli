package printer

import (
	"fmt"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
)

// Common formatting utilities for all output formats

// formatTime formats a time.Time value consistently
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// formatTimePtr handles optional time fields safely
func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "N/A"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// formatBooleanStatus formats boolean values as Yes/No for human readability
func formatBooleanStatus(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// formatDNSStatus formats DNS validation status
func formatDNSStatus(valid bool) string {
	if valid {
		return "Valid"
	}
	return "Invalid"
}

// formatOptionalString handles nil string pointers safely
func formatOptionalString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// formatStringSlice joins string slices with commas
func formatStringSlice(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	return strings.Join(slice, ", ")
}

// formatUUID formats UUID values consistently
func formatUUID(id uuid.UUID) string {
	return id.String()
}

// formatInt formats integer values, handling zero values appropriately
func formatInt(i int) string {
	return fmt.Sprintf("%d", i)
}

// formatUint64 formats integer values, handling zero values appropriately
func formatUint64(i uint64) string {
	return fmt.Sprintf("%d", i)
}

// formatFloat64 formats float64 values with appropriate precision
func formatFloat64(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

// Field ordering and selection utilities

// orderFields reorders fields according to the specified order
// If fieldOrder is empty, returns fields in their natural order
func orderFields(fieldMap map[string]string, fieldOrder []string) [][]string {
	var result [][]string

	if len(fieldOrder) == 0 {
		// Natural order - use reflection order or alphabetical
		for key, value := range fieldMap {
			result = append(result, []string{key, value})
		}
		return result
	}

	// Use specified order
	for _, field := range fieldOrder {
		if value, exists := fieldMap[field]; exists {
			result = append(result, []string{field, value})
		}
	}

	// Add any remaining fields not in the specified order
	for key, value := range fieldMap {
		found := false
		for _, orderedField := range fieldOrder {
			if key == orderedField {
				found = true
				break
			}
		}
		if !found {
			result = append(result, []string{key, value})
		}
	}

	return result
}

// Webhook-specific utility functions

// formatWebhookEvents formats webhook event subscriptions as a comma-separated list
func formatWebhookEvents(webhook *responses.Webhook) string {
	var events []string

	if webhook.OnReception {
		events = append(events, "reception")
	}
	if webhook.OnDelivered {
		events = append(events, "delivered")
	}
	if webhook.OnTransientError {
		events = append(events, "transient_error")
	}
	if webhook.OnFailed {
		events = append(events, "failed")
	}
	if webhook.OnBounced {
		events = append(events, "bounced")
	}
	if webhook.OnSuppressed {
		events = append(events, "suppressed")
	}
	if webhook.OnOpened {
		events = append(events, "opened")
	}
	if webhook.OnClicked {
		events = append(events, "clicked")
	}
	if webhook.OnSuppressionCreated {
		events = append(events, "suppression_created")
	}
	if webhook.OnDNSError {
		events = append(events, "dns_error")
	}

	if len(events) == 0 {
		return "none"
	}
	return strings.Join(events, ", ")
}

// formatWebhookSecret formats webhook secret for display (masked)
func formatWebhookSecret(secret string) string {
	if secret == "" {
		return "Not configured"
	}
	return "Configured"
}

// formatWebhookSecretCreation formats webhook secret for creation response (shows actual secret)
func formatWebhookSecretCreation(secret string) string {
	if secret == "" {
		return "Not generated"
	}
	return secret
}
