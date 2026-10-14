package webhooks

import (
	"fmt"
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
}

// Events lists every webhook event type.
var Events = []Event{
	{"reception", "message.reception"},
	{"delivered", "message.delivered"},
	{"transient_error", "message.transient_error"},
	{"failed", "message.failed"},
	{"bounced", "message.bounced"},
	{"suppressed", "message.suppressed"},
	{"opened", "message.opened"},
	{"clicked", "message.clicked"},
	{"suppression_created", "suppression.created"},
	{"dns_error", "domain.dns_error"},
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
			strings.Join(unknown, ", "), names, strings.Join(eventNames(Events), ", "), strings.Join(EventTypes(Events), ", ")), nil)
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
	return append(EventNames(), EventTypes(Events)...)
}

// EventNames returns the Name of every event.
func EventNames() []string {
	return eventNames(Events)
}

// eventNames returns the Name of each event.
func eventNames(events []Event) []string {
	names := make([]string, len(events))
	for i, event := range events {
		names[i] = event.Name
	}
	return names
}

// EventTypes returns the Type of each event.
func EventTypes(events []Event) []string {
	types := make([]string, len(events))
	for i, event := range events {
		types[i] = event.Type
	}
	return types
}
