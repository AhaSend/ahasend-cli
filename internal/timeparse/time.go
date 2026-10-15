package timeparse

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/errors"
)

// relativeTimePattern is the grammar of a relative time: a positive whole
// number and a unit, m (minutes), h (hours), d (days), w (weeks), mo (months)
// or y (years).
var relativeTimePattern = regexp.MustCompile(`^([1-9][0-9]*)(m|h|d|w|mo|y)$`)

// clock returns the time relative times count from; a test sets it.
var clock = time.Now

// maxRelativeAmount bounds the number of a relative time, so the date
// arithmetic cannot overflow; any time this far away is invalid anyway.
const maxRelativeAmount = 1_000_000_000

// invalidTimeError is the validation error for value, a bad value of the time
// flag name; examples names relative times the flag takes, such as "24h or 7d".
func invalidTimeError(name, value, examples string) error {
	return errors.NewValidationError(fmt.Sprintf("invalid --%s value %q: use RFC 3339 or a relative time such as %s", name, value, examples), nil)
}

// ParseTimePast parses value, the value of the time flag name: an RFC 3339
// time, or a relative time in the past, such as "24h" for 24 hours ago.
func ParseTimePast(name, value string) (time.Time, error) {
	return parseTime(name, value, "24h or 7d", -1)
}

// ParseTimeFuture parses value, the value of the time flag name: an RFC 3339
// time, or a relative time in the future, such as "30d" for 30 days from now.
// examples names relative times the flag takes, such as "2h or 30d", for the
// error a bad value gets. A time that is not after now is a validation error.
func ParseTimeFuture(name, value, examples string) (time.Time, error) {
	return ParseTimeAhead(name, value, examples, 0, 0)
}

// ParseTimeAhead parses value like ParseTimeFuture, for a time the API takes
// only in a window ahead of now, such as a send time: more than minAhead
// from now, when minAhead is not 0, and at most maxAhead from now, when
// maxAhead is not 0. A time that is not after now, or is outside the window,
// is a validation error; the API judges the time again by its own clock.
func ParseTimeAhead(name, value, examples string, minAhead, maxAhead time.Duration) (time.Time, error) {
	t, err := parseTime(name, value, examples, 1)
	if err != nil {
		return time.Time{}, err
	}
	now := clock()
	switch {
	case !t.After(now):
		return time.Time{}, errors.NewValidationError(fmt.Sprintf("invalid --%s value %q: the time is in the past", name, value), nil)
	case minAhead > 0 && !t.After(now.Add(minAhead)):
		return time.Time{}, errors.NewValidationError(fmt.Sprintf(
			"invalid --%s value %q: the API takes a time more than %s ahead", name, value, describeDuration(minAhead)), nil)
	case maxAhead > 0 && t.After(now.Add(maxAhead)):
		return time.Time{}, errors.NewValidationError(fmt.Sprintf(
			"invalid --%s value %q: the API takes a time at most %s ahead", name, value, describeDuration(maxAhead)), nil)
	}
	return t, nil
}

// describeDuration names d, a whole number of days or minutes, such as
// "7 days" or "a minute".
func describeDuration(d time.Duration) string {
	if days := d / (24 * time.Hour); days > 0 && d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%d days", days)
	}
	if d == time.Minute {
		return "a minute"
	}
	return fmt.Sprintf("%d minutes", d/time.Minute)
}

// parseTime parses value as an RFC 3339 time, or as a relative time that sign
// turns into the past (-1) or the future (1) from now.
func parseTime(name, value, examples string, sign int) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}

	match := relativeTimePattern.FindStringSubmatch(value)
	if match == nil {
		return time.Time{}, invalidTimeError(name, value, examples)
	}
	amount, err := strconv.Atoi(match[1])
	if err != nil || amount > maxRelativeAmount {
		return time.Time{}, invalidTimeError(name, value, examples)
	}

	now := clock()
	var t time.Time
	switch match[2] {
	case "m", "h":
		unit := time.Minute
		if match[2] == "h" {
			unit = time.Hour
		}
		if int64(amount) > math.MaxInt64/int64(unit) {
			return time.Time{}, invalidTimeError(name, value, examples)
		}
		t = now.Add(time.Duration(sign*amount) * unit)
	case "d":
		t = now.AddDate(0, 0, sign*amount)
	case "w":
		t = now.AddDate(0, 0, sign*amount*7)
	case "mo":
		t = now.AddDate(0, sign*amount, 0)
	case "y":
		t = now.AddDate(sign*amount, 0, 0)
	}
	// RFC 3339 has four-digit years only, so the API cannot take a time
	// outside them.
	if t.Year() < 0 || t.Year() > 9999 {
		return time.Time{}, invalidTimeError(name, value, examples)
	}
	return t, nil
}
