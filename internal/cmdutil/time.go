package cmdutil

import (
	"time"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/timeparse"
	"github.com/spf13/cobra"
)

// readPastTime returns the time the string flag name gives, an RFC 3339 time
// or a relative time in the past such as "24h", or nil when it is empty.
func readPastTime(cmd *cobra.Command, name string) (*time.Time, error) {
	value, _ := cmd.Flags().GetString(name)
	if value == "" {
		return nil, nil
	}
	parsed, err := timeparse.ParseTimePast(name, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// ReadPastTimeRange returns the times of the --from-time and --to-time flags,
// each nil when it is empty. A --from-time that is not before --to-time is a
// validation error.
func ReadPastTimeRange(cmd *cobra.Command) (from, to *time.Time, err error) {
	if from, err = readPastTime(cmd, "from-time"); err != nil {
		return nil, nil, err
	}
	if to, err = readPastTime(cmd, "to-time"); err != nil {
		return nil, nil, err
	}
	if from != nil && to != nil && !from.Before(*to) {
		return nil, nil, errors.NewValidationError("--from-time must be before --to-time", nil)
	}
	return from, to, nil
}
