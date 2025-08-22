package domains

import (
	"bytes"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCommand_Flags(t *testing.T) {
	// Test that create command has required flags
	createCmd := NewCreateCommand()
	flags := createCmd.Flags()

	formatFlag := flags.Lookup("format")
	assert.NotNil(t, formatFlag)
	assert.Equal(t, "string", formatFlag.Value.Type())

	noDNSHelpFlag := flags.Lookup("no-dns-help")
	assert.NotNil(t, noDNSHelpFlag)
	assert.Equal(t, "bool", noDNSHelpFlag.Value.Type())
}

func TestCreateCommand_Structure(t *testing.T) {
	createCmd := NewCreateCommand()
	assert.Equal(t, "create", createCmd.Name())
	assert.Equal(t, "Create a new domain for email sending", createCmd.Short)
	assert.NotEmpty(t, createCmd.Long)
	assert.NotEmpty(t, createCmd.Example)
}

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
			errorMsg: "domain name cannot be empty",
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
			errorMsg: "invalid domain name format",
		},
		{
			name:     "starts with hyphen",
			domain:   "-example.com",
			wantErr:  true,
			errorMsg: "invalid domain name format",
		},
		{
			name:     "ends with hyphen",
			domain:   "example.com-",
			wantErr:  true,
			errorMsg: "invalid domain name format",
		},
		{
			name:     "too long domain",
			domain:   generateLongDomain(254),
			wantErr:  true,
			errorMsg: "too long",
		},
		{
			name:    "valid international domain",
			domain:  "example.co.uk",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateDomainName(tt.domain)
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

func TestCreateCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "all flags provided",
			args: []string{
				"--format", "cloudflare",
				"--no-dns-help",
			},
			expected: map[string]interface{}{
				"format":      "cloudflare",
				"no-dns-help": true,
			},
		},
		{
			name: "only format flag",
			args: []string{"--format", "bind"},
			expected: map[string]interface{}{
				"format":      "bind",
				"no-dns-help": false,
			},
		},
		{
			name: "only no-dns-help flag",
			args: []string{"--no-dns-help"},
			expected: map[string]interface{}{
				"format":      "", // default is empty
				"no-dns-help": true,
			},
		},
		{
			name: "no flags",
			args: []string{},
			expected: map[string]interface{}{
				"format":      "",
				"no-dns-help": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCreateCommand()
			cmd.SetArgs(tt.args)

			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			for flag, expected := range tt.expected {
				switch expected := expected.(type) {
				case bool:
					value, err := cmd.Flags().GetBool(flag)
					require.NoError(t, err)
					assert.Equal(t, expected, value, "Flag %s should have correct value", flag)
				case string:
					value, err := cmd.Flags().GetString(flag)
					require.NoError(t, err)
					assert.Equal(t, expected, value, "Flag %s should have correct value", flag)
				}
			}
		})
	}
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

func TestCreateCommand_Help(t *testing.T) {
	cmd := NewCreateCommand()

	// Capture help output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()
	assert.Contains(t, helpOutput, "Create a new domain")
	assert.Contains(t, helpOutput, "--format")
	assert.Contains(t, helpOutput, "--no-dns-help")
	assert.Contains(t, helpOutput, "domain")
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
		_ = validation.ValidateDomainName(domain)
	}
}
