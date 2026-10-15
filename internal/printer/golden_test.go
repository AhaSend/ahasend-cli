package printer

import (
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files of TestOutputMatchesTheGoldenFiles")

// nilLists renders each list command's view of no response at all.
func nilLists() map[string]func(ResponseHandler) error {
	config := ListConfig{SuccessMessage: "Retrieved", EmptyMessage: "None found"}
	return map[string]func(ResponseHandler) error{
		"domains":            func(h ResponseHandler) error { return h.HandleDomainList(nil, config) },
		"messages":           func(h ResponseHandler) error { return h.HandleMessageList(nil, config) },
		"webhooks":           func(h ResponseHandler) error { return h.HandleWebhookList(nil, config) },
		"routes":             func(h ResponseHandler) error { return h.HandleRouteList(nil, config) },
		"suppressions":       func(h ResponseHandler) error { return h.HandleSuppressionList(nil, config) },
		"SMTP credentials":   func(h ResponseHandler) error { return h.HandleSMTPList(nil, config) },
		"API keys":           func(h ResponseHandler) error { return h.HandleAPIKeyList(nil, config) },
		"sub-accounts":       func(h ResponseHandler) error { return h.HandleSubAccountList(nil, config) },
		"contacts":           func(h ResponseHandler) error { return h.HandleContactList(nil, config) },
		"lists":              func(h ResponseHandler) error { return h.HandleListsList(nil, config) },
		"members":            func(h ResponseHandler) error { return h.HandleListMemberList(nil, config) },
		"memberships":        func(h ResponseHandler) error { return h.HandleContactMembershipList(nil, config) },
		"templates":          func(h ResponseHandler) error { return h.HandleTemplateList(nil, config) },
		"campaigns":          func(h ResponseHandler) error { return h.HandleCampaignList(nil, config) },
		"recipients":         func(h ResponseHandler) error { return h.HandleCampaignRecipientList(nil, config) },
		"campaign templates": func(h ResponseHandler) error { return h.HandleCampaignTemplateList(nil, config) },
		"versions":           func(h ResponseHandler) error { return h.HandleTemplateVersionList(nil, config) },
	}
}

// TestOutputMatchesTheGoldenFiles pins the table, plain and CSV output of
// every render, of a page of one, two and no items of every list, and of
// every list without a response, byte for byte. Run the test with -update
// to rewrite testdata/<format>.golden after changing the output on purpose.
func TestOutputMatchesTheGoldenFiles(t *testing.T) {
	useLocalTimeZone(t)
	views := map[string]func(ResponseHandler) error{}
	for name, r := range renders() {
		views[name] = r.render
	}
	for _, count := range []int{0, 1, 2} {
		for name, render := range everyList(count) {
			views[fmt.Sprintf("page of %d %s", count, name)] = render
		}
	}
	for name, render := range nilLists() {
		views["no response for "+name] = render
	}

	for _, format := range []string{"table", "plain", "csv"} {
		t.Run(format, func(t *testing.T) {
			var output strings.Builder
			for _, name := range slices.Sorted(maps.Keys(views)) {
				fmt.Fprintf(&output, "=== %s\n%s", name, renderWith(t, format, views[name]))
			}
			path := filepath.Join("testdata", format+".golden")
			if *updateGolden {
				require.NoError(t, os.WriteFile(path, []byte(output.String()), 0o644))
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, string(want), output.String())
		})
	}
}
