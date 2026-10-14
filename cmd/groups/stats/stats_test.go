package stats

import (
	"fmt"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/timeparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeliverabilityHelpNamesTheCountsTheOutputShows checks that the help
// names what each period of the output holds, and nothing it lacks.
func TestDeliverabilityHelpNamesTheCountsTheOutputShows(t *testing.T) {
	long := NewDeliverabilityCommand().Long

	for _, count := range []string{"received", "delivered", "deferred", "bounced", "failed", "suppressed", "opened", "clicked",
		"delivery rate", "open rate"} {
		assert.Contains(t, long, count)
	}
	assert.NotContains(t, long, "rejected")
	assert.NotContains(t, long, "sent")
}

func TestDeliverabilityCommand_FlagDefaults(t *testing.T) {
	cmd := NewDeliverabilityCommand()

	// Parse flags with no arguments to get defaults
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err)

	fromTime, _ := cmd.Flags().GetString("from-time")
	assert.Equal(t, "7d", fromTime)

	groupBy, _ := cmd.Flags().GetString("group-by")
	assert.Equal(t, "day", groupBy)

	chart, _ := cmd.Flags().GetBool("chart")
	assert.False(t, chart)
}

func TestBouncesCommand_FlagDefaults(t *testing.T) {
	cmd := NewBouncesCommand()

	// Parse flags with no arguments to get defaults
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err)

	fromTime, _ := cmd.Flags().GetString("from-time")
	assert.Equal(t, "7d", fromTime)

	groupBy, _ := cmd.Flags().GetString("group-by")
	assert.Equal(t, "day", groupBy)

	showClassification, _ := cmd.Flags().GetBool("show-classification")
	assert.False(t, showClassification)
}

func TestDeliveryTimeCommand_FlagDefaults(t *testing.T) {
	cmd := NewDeliveryTimeCommand()

	// Parse flags with no arguments to get defaults
	err := cmd.ParseFlags([]string{})
	require.NoError(t, err)

	fromTime, _ := cmd.Flags().GetString("from-time")
	assert.Equal(t, "7d", fromTime)

	groupBy, _ := cmd.Flags().GetString("group-by")
	assert.Equal(t, "day", groupBy)

	raw, _ := cmd.Flags().GetBool("raw")
	assert.False(t, raw)
}

// Test time parsing functionality
func TestParseTimeFlag(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		description string
	}{
		{
			name:        "RFC3339 format",
			input:       "2024-01-15T10:30:00Z",
			expectError: false,
			description: "Should parse valid RFC3339 timestamp",
		},
		{
			name:        "relative minutes",
			input:       "30m",
			expectError: false,
			description: "Should parse relative minutes",
		},
		{
			name:        "relative hours",
			input:       "24h",
			expectError: false,
			description: "Should parse relative hours",
		},
		{
			name:        "relative days",
			input:       "7d",
			expectError: false,
			description: "Should parse relative days",
		},
		{
			name:        "relative weeks",
			input:       "2w",
			expectError: false,
			description: "Should parse relative weeks",
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
			description: "Should reject empty string",
		},
		{
			name:        "invalid format",
			input:       "invalid",
			expectError: true,
			description: "Should reject invalid format",
		},
		{
			name:        "invalid unit",
			input:       "5x",
			expectError: true,
			description: "Should reject invalid time unit",
		},
		{
			name:        "zero value",
			input:       "0h",
			expectError: true,
			description: "Should reject a zero amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := timeparse.ParseTimePast("from-time", tt.input)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.True(t, result.IsZero())
			} else {
				assert.NoError(t, err, tt.description)
				assert.False(t, result.IsZero(), "Result should not be zero time for valid input")
			}
		})
	}
}

func TestParseTimeFlag_RelativeCalculation(t *testing.T) {
	// Test that relative time calculation is correct
	before := time.Now()
	result, err := timeparse.ParseTimePast("from-time", "1h")

	require.NoError(t, err)
	require.False(t, result.IsZero())

	// Should be approximately 1 hour ago (within test execution time)
	expectedTime := before.Add(-1 * time.Hour)
	timeDiff := result.Sub(expectedTime)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	// Allow up to 1 second difference for test execution time
	assert.True(t, timeDiff < time.Second, "Relative time calculation should be accurate")
}

// Test parameter validation
func TestDeliverabilityCommand_GroupByValidation(t *testing.T) {
	tests := []struct {
		groupBy     string
		shouldError bool
	}{
		{"hour", false},
		{"day", false},
		{"week", false},
		{"month", false},
		{"invalid", true},
		{"", true},
		{"HOUR", false}, // any case
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("group-by_%s", tt.groupBy), func(t *testing.T) {
			cmd := NewDeliverabilityCommand()
			require.NoError(t, cmd.ParseFlags([]string{"--group-by", tt.groupBy}))
			_, err := readFilters(cmd)

			if tt.shouldError {
				assert.EqualError(t, err, "--group-by must be one of: hour, day, week, month")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkStatsCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}

func BenchmarkDeliverabilityCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewDeliverabilityCommand()
	}
}

func BenchmarkParseTimeFlag_Relative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = timeparse.ParseTimePast("from-time", "7d")
	}
}

func BenchmarkParseTimeFlag_RFC3339(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = timeparse.ParseTimePast("from-time", "2024-01-15T10:30:00Z")
	}
}

// Benchmark tests removed during ResponseHandler migration

// Helper functions for tests
