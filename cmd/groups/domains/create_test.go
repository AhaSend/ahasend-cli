package domains

import (
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/stretchr/testify/assert"
)

func TestValidateDomainName(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		wantErr  bool
		errorMsg string
	}{
		{
			name:    "valid domain",
			domain:  "example.com",
			wantErr: false,
		},
		{
			name:     "empty domain",
			domain:   "",
			wantErr:  true,
			errorMsg: "domain cannot be empty",
		},
		{
			name:    "subdomain",
			domain:  "mail.example.com",
			wantErr: false,
		},
		{
			name:     "invalid characters",
			domain:   "example..com",
			wantErr:  true,
			errorMsg: "not a domain name",
		},
		{
			name:     "starts with hyphen",
			domain:   "-example.com",
			wantErr:  true,
			errorMsg: "not a domain name",
		},
		{
			name:     "ends with hyphen",
			domain:   "example.com-",
			wantErr:  true,
			errorMsg: "not a domain name",
		},
		{
			name:     "too long domain",
			domain:   generateLongDomain(254),
			wantErr:  true,
			errorMsg: "not a domain name",
		},
		{
			name:    "valid international domain",
			domain:  "example.co.uk",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateDomainName("domain", tt.domain)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function to generate long domain names for testing
func generateLongDomain(length int) string {
	domain := ""
	for i := 0; i < length; i++ {
		domain += "a"
	}
	return domain + ".com"
}

func TestCreateCommand_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "valid domain",
			args:      []string{"example.com"},
			expectErr: false,
		},
		{
			name:      "no arguments is allowed", // MaximumNArgs(1) allows 0 args
			args:      []string{},
			expectErr: false,
		},
		{
			name:      "too many arguments",
			args:      []string{"example.com", "test.com"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCreateCommand()
			err := cmd.Args(cmd, tt.args)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateCommand_DefaultValues(t *testing.T) {
	cmd := NewCreateCommand()

	// Test default values
	format, _ := cmd.Flags().GetString("format")
	assert.Empty(t, format, "Format should default to empty string")

	noDNSHelp, _ := cmd.Flags().GetBool("no-dns-help")
	assert.False(t, noDNSHelp, "No DNS help should default to false")
}

func TestCreateCommand_FormatOptions(t *testing.T) {
	validFormats := []string{"bind", "cloudflare", "terraform"}

	for _, format := range validFormats {
		t.Run("format_"+format, func(t *testing.T) {
			cmd := NewCreateCommand()
			cmd.SetArgs([]string{"--format", format, "example.com"})

			err := cmd.ParseFlags([]string{"--format", format})
			assert.NoError(t, err)

			actualFormat, _ := cmd.Flags().GetString("format")
			assert.Equal(t, format, actualFormat)
		})
	}
}

// Benchmark tests
func BenchmarkCreateCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCreateCommand()
	}
}

func BenchmarkCreateCommand_DomainValidation(b *testing.B) {
	domains := []string{
		"example.com",
		"mail.example.com",
		"test.co.uk",
		"subdomain.example.org",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domain := domains[i%len(domains)]
		_ = validation.ValidateDomainName("domain", domain)
	}
}
