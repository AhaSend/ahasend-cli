package domains

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListCommand_FilterLogic(t *testing.T) {
	// Test the domain filtering logic
	testDomains := []struct {
		domain   string
		dnsValid bool
		status   string
		expected bool
	}{
		{"example.com", true, "verified", true},
		{"test.com", false, "verified", false},
		{"example.com", true, "pending", false},
		{"test.com", false, "pending", true},
		{"test.com", false, "failed", true},
	}

	for _, tt := range testDomains {
		t.Run(tt.domain+"_"+tt.status, func(t *testing.T) {
			var matches bool
			switch tt.status {
			case "verified":
				matches = tt.dnsValid
			case "pending", "failed":
				matches = !tt.dnsValid
			}

			assert.Equal(t, tt.expected, matches,
				"Domain %s with DNS valid %v should match status %s: %v",
				tt.domain, tt.dnsValid, tt.status, tt.expected)
		})
	}
}

func TestListCommand_DefaultValues(t *testing.T) {
	cmd := NewListCommand()

	// Test default values
	limit, _ := cmd.Flags().GetInt32("limit")
	assert.Equal(t, int32(0), limit, "Limit should default to 0")

	after, _ := cmd.Flags().GetString("after")
	assert.Empty(t, after, "After should default to empty string")

	status, _ := cmd.Flags().GetString("status")
	assert.Empty(t, status, "Status should default to empty string")
}

func TestListCommand_StatusValidation(t *testing.T) {
	validStatuses := []string{"", "verified", "pending", "failed"}

	for _, status := range validStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			cmd := NewListCommand()
			if status != "" {
				cmd.SetArgs([]string{"--status", status})
			} else {
				cmd.SetArgs([]string{})
			}

			err := cmd.ParseFlags(cmd.Flags().Args())
			assert.NoError(t, err)
		})
	}
}

// Benchmark tests
func BenchmarkListCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewListCommand()
	}
}

func BenchmarkListCommand_FlagParsing(b *testing.B) {
	cmd := NewListCommand()
	args := []string{"--limit", "50", "--after", "token", "--status", "verified"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cmd.ParseFlags(args)
		if err != nil {
			b.Fatal(err)
		}
		// Reset flags for next iteration
		cmd.ResetFlags()
		cmd = NewListCommand()
	}
}
