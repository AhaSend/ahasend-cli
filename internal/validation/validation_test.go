package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{
			name:    "valid email",
			email:   "test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			email:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "valid email with numbers",
			email:   "user123@example123.com",
			wantErr: false,
		},
		{
			name:    "empty email",
			email:   "",
			wantErr: true,
		},
		{
			name:    "invalid email without @",
			email:   "invalid.email.com",
			wantErr: true,
		},
		{
			name:    "invalid email without domain",
			email:   "user@",
			wantErr: true,
		},
		{
			name:    "invalid email without username",
			email:   "@example.com",
			wantErr: true,
		},
		{
			name:    "invalid email with spaces",
			email:   "user name@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEmails(t *testing.T) {
	tests := []struct {
		name    string
		emails  []string
		wantErr bool
	}{
		{
			name:    "valid emails",
			emails:  []string{"test1@example.com", "test2@example.com"},
			wantErr: false,
		},
		{
			name:    "empty slice",
			emails:  []string{},
			wantErr: true,
		},
		{
			name:    "contains invalid email",
			emails:  []string{"test@example.com", "invalid.email"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmails(tt.emails)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid UUID v4",
			id:      "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "valid UUID v1",
			id:      "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			wantErr: false,
		},
		{
			name:    "empty UUID",
			id:      "",
			wantErr: true,
		},
		{
			name:    "invalid UUID format",
			id:      "not-a-uuid",
			wantErr: true,
		},
		{
			name:    "invalid UUID with wrong length",
			id:      "550e8400-e29b-41d4-a716",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUUID(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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
			name:    "valid subdomain",
			domain:  "mail.example.com",
			wantErr: false,
		},
		{
			name:    "valid domain with numbers",
			domain:  "example123.com",
			wantErr: false,
		},
		{
			name:     "empty domain",
			domain:   "",
			wantErr:  true,
			errorMsg: "empty",
		},
		{
			name:     "domain starting with dash",
			domain:   "-invalid.com",
			wantErr:  true,
			errorMsg: "format",
		},
		{
			name:     "domain ending with dash",
			domain:   "invalid.com-",
			wantErr:  true,
			errorMsg: "format",
		},
		{
			name:     "domain with double dots",
			domain:   "invalid..domain.com",
			wantErr:  true,
			errorMsg: "format",
		},
		{
			name:     "domain too long",
			domain:   "this-is-a-very-long-domain-name-that-exceeds-the-maximum-allowed-length-for-a-domain-name-according-to-rfc-specifications-which-is-253-characters-and-this-domain-name-is-intentionally-made-very-long-to-test-the-validation-logic-for-domain-length.example.com",
			wantErr:  true,
			errorMsg: "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomainName(tt.domain)
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

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{
			name:    "valid table format",
			format:  "table",
			wantErr: false,
		},
		{
			name:    "valid json format",
			format:  "json",
			wantErr: false,
		},
		{
			name:    "valid csv format",
			format:  "csv",
			wantErr: false,
		},
		{
			name:    "valid plain format",
			format:  "plain",
			wantErr: false,
		},
		{
			name:    "invalid format",
			format:  "xml",
			wantErr: true,
		},
		{
			name:    "empty format",
			format:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutputFormat(tt.format)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr bool
	}{
		{
			name:    "valid debug level",
			level:   "debug",
			wantErr: false,
		},
		{
			name:    "valid info level",
			level:   "info",
			wantErr: false,
		},
		{
			name:    "valid warn level",
			level:   "warn",
			wantErr: false,
		},
		{
			name:    "valid error level",
			level:   "error",
			wantErr: false,
		},
		{
			name:    "invalid level",
			level:   "trace",
			wantErr: true,
		},
		{
			name:    "empty level",
			level:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLogLevel(tt.level)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateBooleanString(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid true",
			value:   "true",
			wantErr: false,
		},
		{
			name:    "valid false",
			value:   "false",
			wantErr: false,
		},
		{
			name:    "invalid value",
			value:   "yes",
			wantErr: true,
		},
		{
			name:    "empty value",
			value:   "",
			wantErr: true,
		},
		{
			name:    "capitalized true",
			value:   "True",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBooleanString(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateBatchConcurrency(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid concurrency 1",
			value:   "1",
			wantErr: false,
		},
		{
			name:    "valid concurrency 5",
			value:   "5",
			wantErr: false,
		},
		{
			name:    "valid concurrency 10",
			value:   "10",
			wantErr: false,
		},
		{
			name:    "invalid concurrency 0",
			value:   "0",
			wantErr: true,
		},
		{
			name:    "invalid concurrency 11",
			value:   "11",
			wantErr: true,
		},
		{
			name:    "invalid non-number",
			value:   "abc",
			wantErr: true,
		},
		{
			name:    "empty value",
			value:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBatchConcurrency(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEmailRequest(t *testing.T) {
	tests := []struct {
		name      string
		fromEmail string
		toEmails  []string
		subject   string
		content   string
		wantErr   bool
	}{
		{
			name:      "valid email request",
			fromEmail: "sender@example.com",
			toEmails:  []string{"recipient@example.com"},
			subject:   "Test Subject",
			content:   "Test content",
			wantErr:   false,
		},
		{
			name:      "invalid sender email",
			fromEmail: "invalid-email",
			toEmails:  []string{"recipient@example.com"},
			subject:   "Test Subject",
			content:   "Test content",
			wantErr:   true,
		},
		{
			name:      "no recipients",
			fromEmail: "sender@example.com",
			toEmails:  []string{},
			subject:   "Test Subject",
			content:   "Test content",
			wantErr:   true,
		},
		{
			name:      "invalid recipient email",
			fromEmail: "sender@example.com",
			toEmails:  []string{"invalid-email"},
			subject:   "Test Subject",
			content:   "Test content",
			wantErr:   true,
		},
		{
			name:      "empty subject",
			fromEmail: "sender@example.com",
			toEmails:  []string{"recipient@example.com"},
			subject:   "",
			content:   "Test content",
			wantErr:   true,
		},
		{
			name:      "empty content",
			fromEmail: "sender@example.com",
			toEmails:  []string{"recipient@example.com"},
			subject:   "Test Subject",
			content:   "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmailRequest(tt.fromEmail, tt.toEmails, tt.subject, tt.content)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
