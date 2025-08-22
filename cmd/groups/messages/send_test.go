package messages

import (
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/stretchr/testify/assert"
)

func TestSendCommand_Structure(t *testing.T) {
	sendCmd := NewSendCommand()
	assert.Equal(t, "send", sendCmd.Name())
	assert.Equal(t, "Send an email message", sendCmd.Short)
	assert.NotEmpty(t, sendCmd.Long)
	assert.NotEmpty(t, sendCmd.Example)
}

func TestSendCommand_Flags(t *testing.T) {
	// Test that send command has required flags
	sendCmd := NewSendCommand()
	flags := sendCmd.Flags()

	// Required email parameters
	fromFlag := flags.Lookup("from")
	assert.NotNil(t, fromFlag)
	assert.Equal(t, "string", fromFlag.Value.Type())

	toFlag := flags.Lookup("to")
	assert.NotNil(t, toFlag)
	assert.Equal(t, "stringSlice", toFlag.Value.Type())

	subjectFlag := flags.Lookup("subject")
	assert.NotNil(t, subjectFlag)
	assert.Equal(t, "string", subjectFlag.Value.Type())

	// Content options
	textFlag := flags.Lookup("text")
	assert.NotNil(t, textFlag)
	assert.Equal(t, "string", textFlag.Value.Type())

	htmlFlag := flags.Lookup("html")
	assert.NotNil(t, htmlFlag)
	assert.Equal(t, "string", htmlFlag.Value.Type())

	textTemplateFlag := flags.Lookup("text-template")
	assert.NotNil(t, textTemplateFlag)
	assert.Equal(t, "string", textTemplateFlag.Value.Type())

	htmlTemplateFlag := flags.Lookup("html-template")
	assert.NotNil(t, htmlTemplateFlag)
	assert.Equal(t, "string", htmlTemplateFlag.Value.Type())

	ampTemplateFlag := flags.Lookup("amp-template")
	assert.NotNil(t, ampTemplateFlag)
	assert.Equal(t, "string", ampTemplateFlag.Value.Type())

	ampFlag := flags.Lookup("amp")
	assert.NotNil(t, ampFlag)
	assert.Equal(t, "string", ampFlag.Value.Type())

	recipientsFlag := flags.Lookup("recipients")
	assert.NotNil(t, recipientsFlag)
	assert.Equal(t, "string", recipientsFlag.Value.Type())

	globalSubstitutionsFlag := flags.Lookup("global-substitutions")
	assert.NotNil(t, globalSubstitutionsFlag)
	assert.Equal(t, "string", globalSubstitutionsFlag.Value.Type())

	// Advanced options
	sandboxFlag := flags.Lookup("sandbox")
	assert.NotNil(t, sandboxFlag)
	assert.Equal(t, "bool", sandboxFlag.Value.Type())

	scheduleFlag := flags.Lookup("schedule")
	assert.NotNil(t, scheduleFlag)
	assert.Equal(t, "string", scheduleFlag.Value.Type())
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{
			name:    "valid email",
			email:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			email:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "valid email with plus",
			email:   "user+tag@example.com",
			wantErr: false,
		},
		{
			name:    "empty email",
			email:   "",
			wantErr: true,
		},
		{
			name:    "invalid format - no @",
			email:   "userexample.com",
			wantErr: true,
		},
		{
			name:    "invalid format - no domain",
			email:   "user@",
			wantErr: true,
		},
		{
			name:    "invalid format - no TLD",
			email:   "user@example",
			wantErr: true,
		},
		{
			name:    "invalid format - multiple @",
			email:   "user@@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateEmail(tt.email)
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
			name:      "valid request",
			fromEmail: "sender@example.com",
			toEmails:  []string{"recipient@example.com"},
			subject:   "Test Subject",
			content:   "Test content",
			wantErr:   false,
		},
		{
			name:      "multiple recipients",
			fromEmail: "sender@example.com",
			toEmails:  []string{"user1@example.com", "user2@example.com"},
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
			name:      "empty recipients",
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
			err := validation.ValidateEmailRequest(tt.fromEmail, tt.toEmails, tt.subject, tt.content)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
