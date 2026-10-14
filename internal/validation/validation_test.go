package validation

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{
			name:    "valid email with an apostrophe",
			email:   "o'brien@example.com",
			wantErr: false,
		},
		{
			name:    "valid non-ASCII email",
			email:   "jörg@exämple.com",
			wantErr: false,
		},
		{
			name:    "valid email with a plus tag",
			email:   "user+tag@example.co.uk",
			wantErr: false,
		},
		{
			name:    "invalid email with a display name",
			email:   "Jane <jane@example.com>",
			wantErr: true,
		},
		{
			name:    "invalid email in angle brackets",
			email:   "<jane@example.com>",
			wantErr: true,
		},
		{
			name:    "invalid email with a leading space",
			email:   " jane@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail("--to", tt.email)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateUUIDField(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     string
		wantErr   bool
		errorMsg  string
	}{
		{
			name:      "valid sub-account ID",
			fieldName: "sub-account ID",
			value:     "550e8400-e29b-41d4-a716-446655440000",
			wantErr:   false,
		},
		{
			name:      "invalid sub-account ID format",
			fieldName: "sub-account ID",
			value:     "not-a-uuid",
			wantErr:   true,
			errorMsg:  `invalid sub-account ID value "not-a-uuid": not a UUID`,
		},
		{
			name:      "invalid API key ID format",
			fieldName: "API key ID",
			value:     "not-a-uuid",
			wantErr:   true,
			errorMsg:  `invalid API key ID value "not-a-uuid": not a UUID`,
		},
		{
			name:      "empty field value",
			fieldName: "sub-account ID",
			value:     "",
			wantErr:   true,
			errorMsg:  "sub-account ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUUIDField(tt.fieldName, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errorMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseUUIDFieldReturnsParsedUUID(t *testing.T) {
	id, err := ParseUUIDField("list ID", "550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	assert.Equal(t, uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), id)
}

func TestParseUUIDFieldRejectsInvalidFormat(t *testing.T) {
	_, err := ParseUUIDField("list ID", "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid list ID value "not-a-uuid": not a UUID`, err.Error())
}

func TestParseUUIDFieldRejectsEmptyValue(t *testing.T) {
	_, err := ParseUUIDField("list ID", "")
	require.Error(t, err)
	assert.Equal(t, "list ID cannot be empty", err.Error())
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
			errorMsg: "--domain cannot be empty",
		},
		{
			name:     "domain starting with dash",
			domain:   "-invalid.com",
			wantErr:  true,
			errorMsg: "not a domain name",
		},
		{
			name:     "domain ending with dash",
			domain:   "invalid.com-",
			wantErr:  true,
			errorMsg: "not a domain name",
		},
		{
			name:     "domain with double dots",
			domain:   "invalid..domain.com",
			wantErr:  true,
			errorMsg: "not a domain name",
		},
		{
			name:     "domain too long",
			domain:   "this-is-a-very-long-domain-name-that-exceeds-the-maximum-allowed-length-for-a-domain-name-according-to-rfc-specifications-which-is-253-characters-and-this-domain-name-is-intentionally-made-very-long-to-test-the-validation-logic-for-domain-length.example.com",
			wantErr:  true,
			errorMsg: "not a domain name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomainName("--domain", tt.domain)
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

func TestValidatePageLimit(t *testing.T) {
	tests := []struct {
		name    string
		limit   int32
		wantErr bool
	}{
		{
			name:    "negative limit",
			limit:   -1,
			wantErr: true,
		},
		{
			name:    "zero limit",
			limit:   0,
			wantErr: false,
		},
		{
			name:    "maximum limit",
			limit:   100,
			wantErr: false,
		},
		{
			name:    "over maximum limit",
			limit:   101,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePageLimit(tt.limit)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePathSegment(t *testing.T) {
	assert.NoError(t, ValidatePathSegment("contact ID or email", "person@example.com"))
	assert.NoError(t, ValidatePathSegment("contact ID or email", "11111111-1111-4111-8111-111111111111"))
	assert.NoError(t, ValidatePathSegment("contact ID or email", "not an email"), "the API checks the format")

	for _, value := range []string{"", "   ", "\t"} {
		assert.EqualError(t, ValidatePathSegment("contact ID or email", value), "contact ID or email cannot be empty", value)
	}

	for _, value := range []string{".", "..", "a\nb"} {
		assert.Error(t, ValidatePathSegment("contact ID or email", value), value)
	}
	assert.EqualError(t, ValidatePathSegment("contact ID or email", ".."), `invalid contact ID or email value "..": not a contact ID or email`,
		"the value is unquoted, like ValidateUUIDField's")
}

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost":        true,
		"LOCALHOST":        true,
		"127.0.0.1":        true,
		"127.1.2.3":        true,
		"::1":              true,
		"10.0.0.1":         false,
		"api.ahasend.com":  false,
		"localhost.evil.x": false,
		"":                 false,
	} {
		assert.Equal(t, want, IsLoopbackHost(host), host)
	}
}

func TestCheckIPAllowListTakesAddressesAndBlocks(t *testing.T) {
	assert.NoError(t, CheckIPAllowList([]string{"203.0.113.0/24", "198.51.100.7", "2001:db8::1", "2001:db8::/32", "10.0.0.5/8"}))
}

func TestCheckIPAllowListCountsDistinctBlocks(t *testing.T) {
	var entries []string
	for i := range 100 {
		entries = append(entries, fmt.Sprintf("10.0.%d.0/24", i), fmt.Sprintf("10.0.%d.1/24", i))
	}
	assert.NoError(t, CheckIPAllowList(entries), "an address inside a block is the same block")

	err := CheckIPAllowList(append(entries, "192.0.2.1"))
	require.Error(t, err)
	assert.Equal(t, "--ip-allow-list takes at most 100 distinct entries, got 101", err.Error())
}

func TestParseMessageIDKeepsAMessageIDAndCanonicalizesAUUID(t *testing.T) {
	id, err := ParseMessageID("message ID", "<8B1F2E8C-5D6A-4C3B-9E7F-1A2B3C4D5E6F@example.com>")
	require.NoError(t, err)
	assert.Equal(t, "<8B1F2E8C-5D6A-4C3B-9E7F-1A2B3C4D5E6F@example.com>", id)

	id, err = ParseMessageID("message ID", "8B1F2E8C-5D6A-4C3B-9E7F-1A2B3C4D5E6F")
	require.NoError(t, err)
	assert.Equal(t, "8b1f2e8c-5d6a-4c3b-9e7f-1a2b3c4d5e6f", id)
}

func TestValidateEmailNamesTheInput(t *testing.T) {
	assert.EqualError(t, ValidateEmail("--reply-to", "Jane <jane@example.com>"),
		`invalid --reply-to value "Jane <jane@example.com>": not an email address`)
	assert.EqualError(t, ValidateEmail("email", "not-an-email"), `invalid email value "not-an-email": not an email address`)
	assert.EqualError(t, ValidateEmail("--from", ""), "--from cannot be empty")
}
