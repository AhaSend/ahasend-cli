package webhooks

import (
	"encoding/json"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
)

// eventFields returns the on_<name> fields of a webhook request that are set
// to true.
func eventFields(t *testing.T, request any) []string {
	t.Helper()
	data, err := json.Marshal(request)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(data, &fields))
	var on []string
	for _, event := range webhooks.Events {
		if fields["on_"+event.Name] == true {
			on = append(on, "on_"+event.Name)
		}
	}
	return on
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

func TestCreateAndUpdateWithAllEventsSetEveryEvent(t *testing.T) {
	var all []string
	for _, event := range webhooks.Events {
		all = append(all, "on_"+event.Name)
	}

	assert.Equal(t, all, eventFields(t, createdRequest(t, "--all-events")))
	assert.Equal(t, all, eventFields(t, updatedRequest(t, "--all-events")))
	assert.Empty(t, eventFields(t, updatedRequest(t, "--clear-events")))
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
	assert.Equal(t, webhooks.EventTypes(webhooks.Events), triggeredEvents(t, "--all-events"))
}

func TestAllEventsHasOneDescription(t *testing.T) {
	for _, cmd := range []*cobra.Command{NewCreateCommand(), NewUpdateCommand(), NewTriggerCommand()} {
		assert.Equal(t, "Select every event type", cmd.Flags().Lookup("all-events").Usage, cmd.Name())
	}
}
