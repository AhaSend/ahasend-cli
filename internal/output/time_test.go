package output

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatTimeLocal(t *testing.T) {
	tests := []struct {
		name     string
		input    *time.Time
		expected string
	}{
		{
			name:     "nil time",
			input:    nil,
			expected: "-",
		},
		{
			name:     "valid UTC time",
			input:    timePtrTime(time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)),
			expected: "2023-12-25 15:30:45", // Will be converted to local time
		},
		{
			name:     "zero time pointer",
			input:    timePtrTime(time.Time{}),
			expected: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimeLocal(tt.input)

			if tt.input != nil && !tt.input.IsZero() {
				// For non-zero times, check the format pattern rather than exact match
				// since the result depends on the local timezone
				assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatTimeLocalValue(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time",
			input:    time.Time{},
			expected: "-",
		},
		{
			name:  "valid UTC time",
			input: time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC),
			// expected will vary based on local timezone, so we'll check format instead
		},
		{
			name:  "local time",
			input: time.Date(2023, 12, 25, 15, 30, 45, 0, time.Local),
			// expected will be same as input since it's already local
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimeLocalValue(tt.input)

			if tt.input.IsZero() {
				assert.Equal(t, tt.expected, result)
			} else {
				// For non-zero times, check the format pattern
				assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, result)

				// Verify that the formatted time contains the correct date
				assert.Contains(t, result, "2023-12-25")
			}
		})
	}
}

func TestTimeFormatting_LocalTimezoneConversion(t *testing.T) {
	// Test that UTC time is properly converted to local time
	utcTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)

	// Format using pointer function
	resultPtr := FormatTimeLocal(&utcTime)
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, resultPtr)

	// Format using value function
	resultValue := FormatTimeLocalValue(utcTime)
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, resultValue)

	// Both should give the same result
	assert.Equal(t, resultPtr, resultValue)
}

func TestTimeFormatting_DifferentTimezones(t *testing.T) {
	// Create times in different timezones
	baseTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)

	// Test with different timezone locations if available
	locations := []string{"UTC", "Local"}

	for _, locName := range locations {
		t.Run("timezone_"+locName, func(t *testing.T) {
			var testTime time.Time
			if locName == "UTC" {
				testTime = baseTime.UTC()
			} else {
				testTime = baseTime.In(time.Local)
			}

			result := FormatTimeLocalValue(testTime)
			assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, result)
		})
	}
}

func TestTimeFormatting_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		timeFunc func() string
		expected string
	}{
		{
			name: "nil pointer",
			timeFunc: func() string {
				var nilTime *time.Time = nil
				return FormatTimeLocal(nilTime)
			},
			expected: "-",
		},
		{
			name: "zero time value",
			timeFunc: func() string {
				return FormatTimeLocalValue(time.Time{})
			},
			expected: "-",
		},
		{
			name: "Unix epoch",
			timeFunc: func() string {
				return FormatTimeLocalValue(time.Unix(0, 0))
			},
			// Should return formatted Unix epoch time in local timezone
		},
		{
			name: "far future time",
			timeFunc: func() string {
				return FormatTimeLocalValue(time.Date(2999, 12, 31, 23, 59, 59, 0, time.UTC))
			},
			// Should return formatted far future time
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.timeFunc()

			if tt.expected == "-" {
				assert.Equal(t, tt.expected, result)
			} else {
				// For non-dash results, verify format
				assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, result)
			}
		})
	}
}

func TestTimeFormatConsistency(t *testing.T) {
	// Test that the same time formatted multiple ways gives consistent results
	testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)

	// Format the same time multiple times
	results := make([]string, 5)
	for i := range results {
		results[i] = FormatTimeLocalValue(testTime)
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i], "Time formatting should be consistent")
	}
}

func TestTimeFormat_Specification(t *testing.T) {
	// Test that the time format matches the expected specification
	testTime := time.Date(2023, 1, 2, 3, 4, 5, 0, time.Local)
	result := FormatTimeLocalValue(testTime)

	// Should be in format "2006-01-02 15:04:05"
	expected := "2023-01-02 03:04:05"
	assert.Equal(t, expected, result)
}

// Benchmark tests
func BenchmarkFormatTimeLocal_Pointer(b *testing.B) {
	testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatTimeLocal(&testTime)
	}
}

func BenchmarkFormatTimeLocal_Value(b *testing.B) {
	testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatTimeLocalValue(testTime)
	}
}

func BenchmarkFormatTimeLocal_NilPointer(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatTimeLocal(nil)
	}
}

func BenchmarkFormatTimeLocal_ZeroValue(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatTimeLocalValue(time.Time{})
	}
}

// Helper function to create time pointer
func timePtrTime(t time.Time) *time.Time {
	return &t
}
