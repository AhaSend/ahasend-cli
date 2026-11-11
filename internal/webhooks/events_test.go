package webhooks

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEventsTakesNamesAndTypes(t *testing.T) {
	events, err := ParseEvents([]string{"opened", "message.clicked", "suppression_created", "domain.dns_error"})

	require.NoError(t, err)
	assert.Equal(t, []string{"opened", "clicked", "suppression_created", "dns_error"}, EventNames(events))
	assert.Equal(t, []string{"message.opened", "message.clicked", "suppression.created", "domain.dns_error"}, EventTypes(events))
}

func TestParseEventsListsTheEventsForAnUnknownValue(t *testing.T) {
	_, err := ParseEvents([]string{"opened", "sent"})

	require.Error(t, err)
	assert.Equal(t, `"sent" is not an event; --events must be one of:`+" reception, delivered, transient_error, "+
		"failed, bounced, suppressed, opened, clicked, unsubscribed, complained, suppression_created, "+
		"suppression_deleted, dns_error, sending_type_changed, contact_created, contact_updated, contact_deleted, "+
		"contact_added_to_list, contact_removed_from_list, contact_subscribed, contact_unsubscribed, "+
		"contact_complained, or the same events as "+
		"message.reception, message.delivered, message.transient_error, message.failed, message.bounced, "+
		"message.suppressed, message.opened, message.clicked, message.unsubscribed, message.complained, "+
		"suppression.created, suppression.deleted, domain.dns_error, domain.sending_type_changed, contact.created, "+
		"contact.updated, contact.deleted, contact.added_to_list, contact.removed_from_list, contact.subscribed, "+
		"contact.unsubscribed, contact.complained", err.Error())
}

// TestParseEventsTakesTheSendingTypeChangedEvent checks that the event of a
// domain's sending type change parses by its Name and by its Type, in any
// case, sits after dns_error, and is not a contact event.
func TestParseEventsTakesTheSendingTypeChangedEvent(t *testing.T) {
	for _, value := range []string{"sending_type_changed", "domain.sending_type_changed", "Domain.Sending_Type_Changed"} {
		t.Run(value, func(t *testing.T) {
			events, err := ParseEvents([]string{value})

			require.NoError(t, err)
			assert.Equal(t, []Event{{Name: "sending_type_changed", Type: "domain.sending_type_changed"}}, events)
		})
	}
	names := EventNames(Events)
	assert.Equal(t, "sending_type_changed", names[slices.Index(names, "dns_error")+1])
}

func TestParseEventsTakesTheContactEvents(t *testing.T) {
	events, err := ParseEvents([]string{"contact_created", "contact.complained", "message.unsubscribed", "suppression_deleted"})

	require.NoError(t, err)
	assert.Equal(t, []string{"contact.created", "contact.complained", "message.unsubscribed", "suppression.deleted"}, EventTypes(events))
}

// TestEventsMarkTheEightContactEvents checks that the contact events, which
// need the marketing features, are the eight contact.* types, and that the
// other 14 are the ones any account can use.
func TestEventsMarkTheEightContactEvents(t *testing.T) {
	var contact, other []string
	for _, event := range Events {
		if event.Contact {
			contact = append(contact, event.Type)
		} else {
			other = append(other, event.Type)
		}
		assert.Equal(t, event.Contact, strings.HasPrefix(event.Type, "contact."), event.Type)
	}
	assert.Len(t, contact, 8)
	assert.Len(t, other, 14)
}

func TestSelectEventsAddsTheEventsOfTheAllFlagsInTheOrderOfEvents(t *testing.T) {
	named, err := ParseEvents([]string{"opened", "delivered"})
	require.NoError(t, err)
	var contact, other []Event
	for _, event := range Events {
		if event.Contact {
			contact = append(contact, event)
		} else {
			other = append(other, event)
		}
	}

	assert.Equal(t, named, SelectEvents(named, false, false))
	assert.Equal(t, other, SelectEvents(nil, true, false))
	assert.Equal(t, contact, SelectEvents(nil, false, true))
	assert.Equal(t, Events, SelectEvents(nil, true, true))
	assert.Empty(t, SelectEvents(nil, false, false))
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
