package webhooks

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
)

// eventSettingsSent returns each on_<name> field of a webhook request, by
// its JSON name, with the value the request sends.
func eventSettingsSent(t *testing.T, request any) map[string]bool {
	t.Helper()
	data, err := json.Marshal(request)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(data, &fields))
	settings := map[string]bool{}
	for name, value := range fields {
		if strings.HasPrefix(name, "on_") {
			on, ok := value.(bool)
			require.True(t, ok, "%s is %v", name, value)
			settings[name] = on
		}
	}
	return settings
}

// eventFields returns the on_<name> fields of a webhook request that are set
// to true, sorted.
func eventFields(t *testing.T, request any) []string {
	t.Helper()
	var on []string
	for name, value := range eventSettingsSent(t, request) {
		if value {
			on = append(on, name)
		}
	}
	slices.Sort(on)
	return on
}

// onFields returns the on_<name> field of each event of events, sorted.
func onFields(events []webhooks.Event) []string {
	var fields []string
	for _, event := range events {
		fields = append(fields, "on_"+event.Name)
	}
	slices.Sort(fields)
	return fields
}

// contactEvents returns the 8 contact events of webhooks.Events.
func contactEvents() []webhooks.Event {
	return slices.DeleteFunc(slices.Clone(webhooks.Events), func(event webhooks.Event) bool { return !event.Contact })
}

// otherEvents returns the 13 events of webhooks.Events that any account can
// use.
func otherEvents() []webhooks.Event {
	return slices.DeleteFunc(slices.Clone(webhooks.Events), func(event webhooks.Event) bool { return event.Contact })
}

// TestEventSettersSetOneFieldForEachEvent checks setCreateEvents and
// setUpdateEvents over every event: given one event alone, each turns on
// exactly that event's on_<name> field and sends every other one as false,
// and no two events turn on the same field.
func TestEventSettersSetOneFieldForEachEvent(t *testing.T) {
	setters := map[string]func(settings map[string]bool) any{
		"setCreateEvents": func(settings map[string]bool) any {
			var req requests.CreateWebhookRequest
			setCreateEvents(&req, settings)
			return req
		},
		"setUpdateEvents": func(settings map[string]bool) any {
			var req requests.UpdateWebhookRequest
			setUpdateEvents(&req, settings)
			return req
		},
	}
	for name, set := range setters {
		t.Run(name, func(t *testing.T) {
			setBy := map[string]string{}
			for _, event := range webhooks.Events {
				sent := set(eventSettings([]webhooks.Event{event}))

				on := eventFields(t, sent)
				require.Equal(t, []string{"on_" + event.Name}, on, event.Name)
				assert.NotContains(t, setBy, on[0], "%s and %s set the same field", setBy[on[0]], event.Name)
				setBy[on[0]] = event.Name
				assert.Len(t, eventSettingsSent(t, sent), len(webhooks.Events), event.Name)
			}
		})
	}
}

func TestCreateAndUpdateTakeEachEventByNameOrType(t *testing.T) {
	for _, event := range webhooks.Events {
		for _, value := range []string{event.Name, event.Type} {
			t.Run(value, func(t *testing.T) {
				want := []string{"on_" + event.Name}
				assert.Equal(t, want, eventFields(t, createdRequest(t, "--events", value)))
				assert.Equal(t, want, eventFields(t, updatedRequest(t, "--events", value)))
			})
		}
	}
}

// TestCreateAndUpdateWithAllEventsSetEveryNonContactEvent checks that
// --all-events turns on the 13 events any account can use and sends the
// contact events as false, that --all-contact-events does the reverse, and
// that the two together turn on all 21.
func TestCreateAndUpdateWithAllEventsSetEveryNonContactEvent(t *testing.T) {
	tests := []struct {
		args  []string
		want  []webhooks.Event
		count int
	}{
		{[]string{"--all-events"}, otherEvents(), 13},
		{[]string{"--all-contact-events"}, contactEvents(), 8},
		{[]string{"--all-events", "--all-contact-events"}, webhooks.Events, 21},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			require.Len(t, tt.want, tt.count)
			for name, sent := range map[string]any{
				"create": createdRequest(t, tt.args...),
				"update": updatedRequest(t, tt.args...),
			} {
				assert.Equal(t, onFields(tt.want), eventFields(t, sent), name)
				assert.Len(t, eventSettingsSent(t, sent), len(webhooks.Events), "%s sends every event setting", name)
			}
		})
	}
}

// TestUpdateEventFlagsSendTheWholeSetOfEvents checks that the event flags of
// webhooks update send all 21 settings, so every event they do not name is
// turned off, and that a change without them sends none.
func TestUpdateEventFlagsSendTheWholeSetOfEvents(t *testing.T) {
	delivered := updatedRequest(t, "--events", "delivered")
	assert.Len(t, eventSettingsSent(t, delivered), len(webhooks.Events))
	assert.Equal(t, []string{"on_delivered"}, eventFields(t, delivered))

	cleared := updatedRequest(t, "--clear-events")
	assert.Len(t, eventSettingsSent(t, cleared), len(webhooks.Events))
	assert.Empty(t, eventFields(t, cleared), "--clear-events turns every event off")

	assert.Empty(t, eventSettingsSent(t, updatedRequest(t, "--name", "Events")))
}

// triggeredEvents runs webhooks trigger with args and returns the events it
// sent.
func triggeredEvents(t *testing.T, args ...string) []string {
	t.Helper()
	var sent []string
	mockClient := &mocks.MockClient{}
	mockClient.On("TriggerWebhook", uuid.MustParse(testWebhookID), mock.Anything).Run(func(arguments mock.Arguments) {
		sent = arguments.Get(1).([]string)
	}).Return(&common.SuccessResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	_, _, err := testutil.RunCommand(t, NewTriggerCommand(), "plain", "", append([]string{testWebhookID}, args...)...)
	require.NoError(t, err)
	return sent
}

func TestTriggerSendsTheEventTypesTheEndpointTakes(t *testing.T) {
	assert.Equal(t, []string{"message.delivered", "message.opened"},
		triggeredEvents(t, "--events", "delivered", "--events", "message.opened"))
	assert.Equal(t, []string{"message.reception", "message.delivered", "message.transient_error", "message.failed",
		"message.bounced", "message.suppressed", "message.opened", "message.clicked", "message.unsubscribed",
		"message.complained", "suppression.created", "suppression.deleted", "domain.dns_error"},
		triggeredEvents(t, "--all-events"))
	assert.Equal(t, []string{"contact.created", "contact.updated", "contact.deleted", "contact.added_to_list",
		"contact.removed_from_list", "contact.subscribed", "contact.unsubscribed", "contact.complained"},
		triggeredEvents(t, "--all-contact-events"))
	assert.Equal(t, webhooks.EventTypes(webhooks.Events), triggeredEvents(t, "--all-events", "--all-contact-events"))
}

func TestAllEventsHasOneDescription(t *testing.T) {
	for _, cmd := range []*cobra.Command{NewCreateCommand(), NewUpdateCommand(), NewTriggerCommand()} {
		assert.Equal(t, "Select every event except the contact events", cmd.Flags().Lookup("all-events").Usage, cmd.Name())
		assert.Equal(t, "Select every contact event", cmd.Flags().Lookup("all-contact-events").Usage, cmd.Name())
	}
}

// TestEventCommandsSayWhatTheContactEventsNeed checks that the help of each
// command that selects events says what the API needs for the contact
// events.
func TestEventCommandsSayWhatTheContactEventsNeed(t *testing.T) {
	for _, cmd := range []*cobra.Command{NewCreateCommand(), NewUpdateCommand(), NewTriggerCommand()} {
		long := strings.Join(strings.Fields(cmd.Long), " ")
		assert.Contains(t, long, "The contact events need an account with the marketing features, and an API key "+
			"with the contacts:read scope, and lists:read as well for the list, subscribe, unsubscribe and "+
			"complaint contact events. The API checks the key for every event an enabled webhook has on, so a "+
			"change to such a webhook needs these scopes even when it names no event.", cmd.Name())
	}
}
