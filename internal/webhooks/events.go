package webhooks

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
)

// Event is a webhook event type. A webhook's settings name it by Name, as the
// API's on_<name> fields do; event payloads, the trigger endpoint and the
// listen filter name it by Type.
type Event struct {
	Name string
	Type string
	// Contact marks a contact event, which the API allows only for an
	// account with the marketing features and an API key with the contact
	// read scopes.
	Contact bool
}

// Events lists every webhook event type, in the order the CLI shows them.
var Events = []Event{
	{Name: "reception", Type: "message.reception"},
	{Name: "delivered", Type: "message.delivered"},
	{Name: "transient_error", Type: "message.transient_error"},
	{Name: "failed", Type: "message.failed"},
	{Name: "bounced", Type: "message.bounced"},
	{Name: "suppressed", Type: "message.suppressed"},
	{Name: "opened", Type: "message.opened"},
	{Name: "clicked", Type: "message.clicked"},
	{Name: "unsubscribed", Type: "message.unsubscribed"},
	{Name: "complained", Type: "message.complained"},
	{Name: "suppression_created", Type: "suppression.created"},
	{Name: "suppression_deleted", Type: "suppression.deleted"},
	{Name: "dns_error", Type: "domain.dns_error"},
	{Name: "sending_type_changed", Type: "domain.sending_type_changed"},
	{Name: "contact_created", Type: "contact.created", Contact: true},
	{Name: "contact_updated", Type: "contact.updated", Contact: true},
	{Name: "contact_deleted", Type: "contact.deleted", Contact: true},
	{Name: "contact_added_to_list", Type: "contact.added_to_list", Contact: true},
	{Name: "contact_removed_from_list", Type: "contact.removed_from_list", Contact: true},
	{Name: "contact_subscribed", Type: "contact.subscribed", Contact: true},
	{Name: "contact_unsubscribed", Type: "contact.unsubscribed", Contact: true},
	{Name: "contact_complained", Type: "contact.complained", Contact: true},
}

// SelectEvents returns the events the event flags select: events, the ones
// --events named, then, in the order of Events, every event but the contact
// events when all is set, and every contact event when allContact is set.
func SelectEvents(events []Event, all, allContact bool) []Event {
	selected := slices.Clone(events)
	for _, event := range Events {
		if (all && !event.Contact) || (allContact && event.Contact) {
			selected = append(selected, event)
		}
	}
	return selected
}

// ParseEvents returns the event each value names, by its Name or its Type in
// any case, in the order given. Values that name no event, which the error
// names before it lists the events, and a value that names an event an
// earlier value named, are validation errors of the --events flag.
func ParseEvents(values []string) ([]Event, error) {
	var unknown []string
	for _, value := range values {
		if _, ok := FindEvent(value); !ok {
			unknown = append(unknown, strconv.Quote(value))
		}
	}
	if len(unknown) > 0 {
		names := "is not an event"
		if len(unknown) > 1 {
			names = "are not events"
		}
		return nil, errors.NewValidationError(fmt.Sprintf("%s %s; --events must be one of: %s, or the same events as %s",
			strings.Join(unknown, ", "), names, strings.Join(EventNames(Events), ", "), strings.Join(EventTypes(Events), ", ")), nil)
	}

	var events []Event
	given := map[Event]string{}
	for _, value := range values {
		event, _ := FindEvent(value)
		if earlier, ok := given[event]; ok {
			return nil, validation.InvalidValueError("--events", value, fmt.Sprintf("names the same event as %q", earlier))
		}
		given[event] = value
		events = append(events, event)
	}
	return events, nil
}

// FindEvent returns the event value names by its Name or its Type, in any
// case.
func FindEvent(value string) (Event, bool) {
	for _, event := range Events {
		if strings.EqualFold(value, event.Name) || strings.EqualFold(value, event.Type) {
			return event, true
		}
	}
	return Event{}, false
}

// EventValues returns the values --events takes: the Name of each event,
// then its Type.
func EventValues() []string {
	return append(EventNames(Events), EventTypes(Events)...)
}

// EventNames returns the Name of each event, or nil when there are none.
func EventNames(events []Event) []string {
	var names []string
	for _, event := range events {
		names = append(names, event.Name)
	}
	return names
}

// EventTypes returns the Type of each event, or nil when there are none.
func EventTypes(events []Event) []string {
	var types []string
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}
