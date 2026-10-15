package timeparse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTimePastNamesTheFlagAndTheValue(t *testing.T) {
	_, err := ParseTimePast("from-time", "Soon")

	require.Error(t, err)
	assert.Equal(t, `invalid --from-time value "Soon": use RFC 3339 or a relative time such as 24h or 7d`, err.Error())
}

func TestParseTimeFutureGivesOneMessageForEveryBadValue(t *testing.T) {
	for _, value := range []string{"soon", "-2h", "0d", "3 fortnights"} {
		t.Run(value, func(t *testing.T) {
			_, err := ParseTimeFuture("expires", value, "30d or 1y")

			require.Error(t, err)
			assert.Equal(t, `invalid --expires value "`+value+`": use RFC 3339 or a relative time such as 30d or 1y`, err.Error())
		})
	}
}

// TestParseTimeRelativeGrammar checks that both directions take exactly a
// positive whole number and one unit.
func TestParseTimeRelativeGrammar(t *testing.T) {
	now := time.Now()
	valid := map[string]time.Duration{
		"30m": 30 * time.Minute,
		"24h": 24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
		"1w":  7 * 24 * time.Hour,
	}
	for value, offset := range valid {
		t.Run(value, func(t *testing.T) {
			past, err := ParseTimePast("from-time", value)
			require.NoError(t, err)
			assert.WithinDuration(t, now.Add(-offset), past, time.Minute)

			future, err := ParseTimeFuture("expires", value, "30d or 1y")
			require.NoError(t, err)
			assert.WithinDuration(t, now.Add(offset), future, time.Minute)
		})
	}

	for _, value := range []string{"3mo", "1y"} {
		t.Run(value, func(t *testing.T) {
			past, err := ParseTimePast("from-time", value)
			require.NoError(t, err)
			assert.True(t, past.Before(now.AddDate(0, 0, -27)))

			future, err := ParseTimeFuture("expires", value, "30d or 1y")
			require.NoError(t, err)
			assert.True(t, future.After(now.AddDate(0, 0, 27)))
		})
	}

	invalid := []string{"3h30m", "1.5h", "7xd", "-2d", "+2d", "0d", "07d", "d", "7", "7D", " 7d", "7d ",
		"7 d", "1min", "2weeks", "99999999999h", "10000y"}
	for _, value := range invalid {
		t.Run("invalid "+value, func(t *testing.T) {
			_, err := ParseTimePast("from-time", value)
			require.Error(t, err)
			assert.Equal(t, `invalid --from-time value "`+value+`": use RFC 3339 or a relative time such as 24h or 7d`, err.Error())

			_, err = ParseTimeFuture("expires", value, "30d or 1y")
			require.Error(t, err)
			assert.Equal(t, `invalid --expires value "`+value+`": use RFC 3339 or a relative time such as 30d or 1y`, err.Error())
		})
	}
}

// TestParseTimeRejectsAmountsThatWouldOverflow checks the bound on the number
// of a relative time: a number too large fails, even where the date
// arithmetic would wrap around to a time that looks valid.
func TestParseTimeRejectsAmountsThatWouldOverflow(t *testing.T) {
	for _, value := range []string{"9223372036854775807d", "9223372036854775807w",
		"9223372036854775807mo", "9223372036854775807y"} {
		t.Run(value, func(t *testing.T) {
			_, err := ParseTimePast("from-time", value)
			require.Error(t, err)

			_, err = ParseTimeFuture("expires", value, "30d or 1y")
			require.Error(t, err)
		})
	}
}

// TestParseTimeFutureRejectsATimeInThePast checks that a future time flag
// refuses any time that is not after now, a recent one included.
func TestParseTimeFutureRejectsATimeInThePast(t *testing.T) {
	for _, ago := range []time.Duration{2 * time.Minute, 30 * time.Second} {
		past := time.Now().Add(-ago).UTC().Format(time.RFC3339)
		_, err := ParseTimeFuture("expires", past, "30d or 1y")

		require.Error(t, err)
		assert.Equal(t, `invalid --expires value "`+past+`": the time is in the past`, err.Error())
	}
}

func TestParseTimeAheadChecksItsWindow(t *testing.T) {
	_, err := ParseTimeAhead("schedule", "30s", "2h", time.Minute, 0)
	assert.EqualError(t, err, `invalid --schedule value "30s": use RFC 3339 or a relative time such as 2h`)

	soon := time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339)
	_, err = ParseTimeAhead("schedule", soon, "2h", time.Minute, 0)
	assert.EqualError(t, err, `invalid --schedule value "`+soon+`": the API takes a time more than a minute ahead`)

	_, err = ParseTimeAhead("schedule", "8d", "2h", 0, 7*24*time.Hour)
	assert.EqualError(t, err, `invalid --schedule value "8d": the API takes a time at most 7 days ahead`)

	_, err = ParseTimeAhead("schedule", "2h", "2h", time.Minute, 7*24*time.Hour)
	assert.NoError(t, err)
}

// TestMonthsAndYearsAreCalendarUnits checks that mo and y count calendar
// months and years, as time.AddDate does, from the end of a long month:
// a month after January 31 is March 3 in 2026, not February 28 or a
// 30-day step.
func TestMonthsAndYearsAreCalendarUnits(t *testing.T) {
	start := time.Date(2026, time.January, 31, 12, 0, 0, 0, time.UTC)
	clock = func() time.Time { return start }
	t.Cleanup(func() { clock = time.Now })

	tests := []struct {
		value        string
		future, past time.Time
	}{
		{"1mo", time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC), time.Date(2025, time.December, 31, 12, 0, 0, 0, time.UTC)},
		{"13mo", time.Date(2027, time.March, 3, 12, 0, 0, 0, time.UTC), time.Date(2024, time.December, 31, 12, 0, 0, 0, time.UTC)},
		{"1y", time.Date(2027, time.January, 31, 12, 0, 0, 0, time.UTC), time.Date(2025, time.January, 31, 12, 0, 0, 0, time.UTC)},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			future, err := ParseTimeFuture("expires", test.value, "30d or 1y")
			require.NoError(t, err)
			assert.True(t, test.future.Equal(future), "future: %s", future)

			past, err := ParseTimePast("from-time", test.value)
			require.NoError(t, err)
			assert.True(t, test.past.Equal(past), "past: %s", past)
		})
	}
}
