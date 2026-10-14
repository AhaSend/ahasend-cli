package webhooks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEventsTakesNamesAndTypes(t *testing.T) {
	events, err := ParseEvents([]string{"opened", "message.clicked", "suppression_created", "domain.dns_error"})

	require.NoError(t, err)
	assert.Equal(t, []string{"opened", "clicked", "suppression_created", "dns_error"}, eventNames(events))
	assert.Equal(t, []string{"message.opened", "message.clicked", "suppression.created", "domain.dns_error"}, EventTypes(events))
}

func TestParseEventsListsTheEventsForAnUnknownValue(t *testing.T) {
	_, err := ParseEvents([]string{"opened", "sent"})

	require.Error(t, err)
	assert.Equal(t, `"sent" is not an event; --events must be one of:`+" reception, delivered, transient_error, "+
		"failed, bounced, suppressed, opened, clicked, suppression_created, dns_error, or the same events as "+
		"message.reception, message.delivered, message.transient_error, message.failed, message.bounced, "+
		"message.suppressed, message.opened, message.clicked, suppression.created, domain.dns_error", err.Error())
}

func TestParseEventsNamesEveryUnknownValue(t *testing.T) {
	_, err := ParseEvents([]string{"sent", "opened", "read"})

	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), `"sent", "read" are not events; --events must be one of: reception, `), err.Error())
}

// TestParseEventsRejectsAnEventGivenTwice checks that an event named twice,
// by its Name, its Type or both in any case, fails rather than being
// triggered twice.
func TestParseEventsRejectsAnEventGivenTwice(t *testing.T) {
	for _, values := range [][]string{
		{"delivered", "message.delivered"},
		{"delivered", "DELIVERED"},
	} {
		t.Run(values[1], func(t *testing.T) {
			_, err := ParseEvents(values)

			require.Error(t, err)
			assert.Equal(t, `invalid --events value "`+values[1]+`": names the same event as "delivered"`, err.Error())
		})
	}
}
