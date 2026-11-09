package webhooks

import (
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/webhooks"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/spf13/cobra"
)

// Usages of the flags that select every event of a kind, which create,
// update and trigger share.
const (
	allEventsUsage        = "Select every event except the contact events"
	allContactEventsUsage = "Select every contact event"
)

// origins are the values of --origins: who made the change that a contact or
// suppression event reports.
var origins = []string{
	common.WebhookOriginRecipient,
	common.WebhookOriginAPI,
	common.WebhookOriginDashboard,
	common.WebhookOriginSystem,
}

// contactEventsHelp is the paragraph of Long help that says what the contact
// events need.
const contactEventsHelp = `The contact events need an account with the marketing features, and an API
key with the contacts:read scope, and lists:read as well for the list,
subscribe, unsubscribe and complaint contact events. The API checks the key
for every event an enabled webhook has on, so a change to such a webhook
needs these scopes even when it names no event.`

// addEventFlags adds the flags that select events: --events, with usage,
// --all-events and --all-contact-events.
func addEventFlags(cmd *cobra.Command, eventsUsage string) {
	cmdutil.AddRepeatableFlag(cmd, "events", eventsUsage)
	cmdutil.CompleteValues(cmd, "events", webhooks.EventValues())
	cmd.Flags().Bool("all-events", false, allEventsUsage)
	cmd.Flags().Bool("all-contact-events", false, allContactEventsUsage)
}

// readEventFlags returns the events that the flags of addEventFlags select,
// as webhooks.SelectEvents orders them, and whether any of them selects
// events. --events with --all-events or --all-contact-events is a validation
// error, and so is an empty or unknown --events value; clearFlag names the
// flag that the error of an empty value points to, or is "".
func readEventFlags(cmd *cobra.Command, clearFlag string) ([]webhooks.Event, bool, error) {
	values, err := cmdutil.ReadRepeatable(cmd, "events", clearFlag)
	if err != nil {
		return nil, false, err
	}
	for _, flag := range []string{"all-events", "all-contact-events"} {
		if err := cmdutil.RejectTogether(cmd, "events", flag); err != nil {
			return nil, false, err
		}
	}
	events, err := webhooks.ParseEvents(values)
	if err != nil {
		return nil, false, err
	}
	all, _ := cmd.Flags().GetBool("all-events")
	allContact, _ := cmd.Flags().GetBool("all-contact-events")
	return webhooks.SelectEvents(events, all, allContact), values != nil || all || allContact, nil
}

// eventSettings returns, for the Name of each webhook event, whether events
// holds it.
func eventSettings(events []webhooks.Event) map[string]bool {
	settings := make(map[string]bool, len(webhooks.Events))
	for _, event := range webhooks.Events {
		settings[event.Name] = false
	}
	for _, event := range events {
		settings[event.Name] = true
	}
	return settings
}

// readOrigins returns the --origins values in the API's spelling, or nil
// when the flag was not set. An empty value, or one that is not an origin,
// is a validation error.
func readOrigins(cmd *cobra.Command) ([]string, error) {
	values, err := cmdutil.ReadRepeatable(cmd, "origins", "")
	if err != nil {
		return nil, err
	}
	for i, value := range values {
		if values[i], err = cmdutil.MatchOneOf(cmd, "origins", value); err != nil {
			return nil, err
		}
	}
	return values, nil
}
