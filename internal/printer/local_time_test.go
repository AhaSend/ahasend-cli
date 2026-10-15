package printer

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useLocalTimeZone sets time.Local to a fixed UTC+2 zone for the test, so
// local time output does not depend on the machine's zone.
func useLocalTimeZone(t *testing.T) {
	t.Helper()
	previous := time.Local
	time.Local = time.FixedZone("UTC+2", 2*60*60)
	t.Cleanup(func() { time.Local = previous })
}

// timelessViews are the views of renders that show no time: their result
// has none.
var timelessViews = map[string]bool{
	"auth result": true, "auth result without a profile": true, "empty": true,
	"sent messages": true, "smtp sent": true, "smtp test": true, "smtp test fails": true,
}

// timelessTables are the list views whose table has no time column.
var timelessTables = map[string]bool{
	"campaign recipients": true, "contact list": true, "domain list": true, "two domains": true,
	"route list": true, "webhook list": true,
}

func TestTableAndPlainShowLocalTimeWithZone(t *testing.T) {
	useLocalTimeZone(t)
	for _, format := range []string{"table", "plain"} {
		for name, r := range renders() {
			t.Run(format+"/"+name, func(t *testing.T) {
				output := renderWith(t, format, r.render)
				assert.NotRegexp(t, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}`, output)
				times := regexp.MustCompile(`\d{2}:\d{2}:\d{2}`).FindAllString(output, -1)
				zoned := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} UTC\+2`).FindAllString(output, -1)
				assert.Len(t, zoned, len(times), "every time ends with its zone")
				if !r.sparse && !timelessViews[name] && !(format == "table" && timelessTables[name]) {
					assert.NotEmpty(t, zoned)
				}
			})
		}
	}
}

func TestCSVWritesRFC3339Times(t *testing.T) {
	useLocalTimeZone(t)
	for name, r := range renders() {
		t.Run(name, func(t *testing.T) {
			output := renderWith(t, "csv", r.render)
			assert.NotRegexp(t, `\d{4}-\d{2}-\d{2} \d{2}:\d{2}`, output)
			if !r.sparse && !timelessViews[name] {
				assert.Regexp(t, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})`, output)
			}
		})
	}
}

func TestCSVTimesKeepTheAPIOffset(t *testing.T) {
	useLocalTimeZone(t)
	contact := contactFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleSingleContact(&contact, SingleConfig{}))

	_, rows := readCSV(t, output.String())
	row := rows[0]
	assert.Equal(t, "2026-09-10T08:30:00Z", row["created_at"])
	assert.Empty(t, row["unsubscribed_at"], "a nil time prints as empty")
}

func TestDisplayTimeShowsLocalTimeAndZone(t *testing.T) {
	useLocalTimeZone(t)
	assert.Equal(t, "2026-09-10 10:30:00 UTC+2", formatTime(time.Date(2026, 9, 10, 8, 30, 0, 0, time.UTC)))
	assert.Empty(t, formatTime(time.Time{}), "a zero time is missing")
	assert.Empty(t, formatCSVTime(time.Time{}), "a zero time is missing")
}

func TestPlainLeavesANilTimeOut(t *testing.T) {
	member := listMemberFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleSingleListMember(&member, SingleConfig{}))
	assert.NotContains(t, output.String(), "Unsubscribed At")
}

func TestTableAndPlainUseOneLabelForEachTimeAndNoObjectField(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		for name, r := range renders() {
			t.Run(format+"/"+name, func(t *testing.T) {
				// The port line of the SMTP settings is a sentence, not a list.
				text := strings.ReplaceAll(renderWith(t, format, r.render), "(STARTTLS, recommended); 25", "")
				assert.NotRegexp(t, `(?m)^Object:|│ Object `, text)
				assert.NotContains(t, text, "Created At")
				assert.NotContains(t, text, "Updated At")
				assert.NotContains(t, text, "; ")
			})
		}
	}
}
