// Package validation provides centralized validation utilities for the AhaSend CLI.
//
// This package consolidates all validation logic used across CLI commands:
//
//   - Email address format validation with comprehensive regex
//   - UUID format validation for account IDs and message IDs
//   - Domain name validation following DNS standards
//   - Output format validation (table, json, csv, plain)
//   - Log level validation (debug, info, warn, error)
//   - Preference value validation with type checking
//   - Batch concurrency limits and safety constraints
//
// All validation functions return structured errors from the errors package
// for consistent error handling and user feedback across the CLI.
package validation

import (
	"regexp"
	"strconv"

	"github.com/google/uuid"

	"github.com/AhaSend/ahasend-cli/internal/errors"
)

// Email validation regex pattern
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Domain validation regex pattern
var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)

// ValidateEmail validates an email address format
func ValidateEmail(email string) error {
	if email == "" {
		return errors.NewValidationError("email address cannot be empty", nil)
	}
	if !emailRegex.MatchString(email) {
		return errors.NewValidationError("invalid email address format: "+email, nil)
	}
	return nil
}

// ValidateEmails validates multiple email addresses
func ValidateEmails(emails []string) error {
	if len(emails) == 0 {
		return errors.NewValidationError("at least one email address is required", nil)
	}

	for _, email := range emails {
		if err := ValidateEmail(email); err != nil {
			return err
		}
	}
	return nil
}

// ValidateUUID validates that a string is a valid UUID format
func ValidateUUID(id string) error {
	if id == "" {
		return errors.NewValidationError("UUID cannot be empty", nil)
	}
	if _, err := uuid.Parse(id); err != nil {
		return errors.NewValidationError("invalid UUID format: "+id, err)
	}
	return nil
}

// ValidateDomainName validates a domain name format
func ValidateDomainName(domain string) error {
	if domain == "" {
		return errors.NewValidationError("domain name cannot be empty", nil)
	}

	if len(domain) > 253 {
		return errors.NewValidationError("domain name too long (max 253 characters)", nil)
	}

	if !domainRegex.MatchString(domain) {
		return errors.NewValidationError("invalid domain name format", nil)
	}

	return nil
}

// ValidateOutputFormat validates output format values
func ValidateOutputFormat(value string) error {
	validFormats := []string{"table", "json", "csv", "plain"}
	for _, format := range validFormats {
		if value == format {
			return nil
		}
	}
	return errors.NewValidationError("invalid output format: "+value+" (must be one of: table, json, csv, plain)", nil)
}

// ValidateLogLevel validates log level values
func ValidateLogLevel(value string) error {
	validLevels := []string{"debug", "info", "warn", "error"}
	for _, level := range validLevels {
		if value == level {
			return nil
		}
	}
	return errors.NewValidationError("invalid log level: "+value+" (must be one of: debug, info, warn, error)", nil)
}

// ValidateBooleanString validates boolean string values
func ValidateBooleanString(value string) error {
	if value != "true" && value != "false" {
		return errors.NewValidationError("invalid boolean value: "+value+" (must be 'true' or 'false')", nil)
	}
	return nil
}

// ValidateBatchConcurrency validates batch concurrency values
func ValidateBatchConcurrency(value string) error {
	concurrency, err := strconv.Atoi(value)
	if err != nil {
		return errors.NewValidationError("invalid batch concurrency: "+value+" (must be a number)", nil)
	}
	if concurrency < 1 || concurrency > 10 {
		return errors.NewValidationError("invalid batch concurrency: "+strconv.Itoa(concurrency)+" (must be between 1 and 10)", nil)
	}
	return nil
}

// ValidateEmailRequest validates a complete email sending request
func ValidateEmailRequest(fromEmail string, toEmails []string, subject, content string) error {
	// Validate sender
	if err := ValidateEmail(fromEmail); err != nil {
		return errors.NewValidationError("invalid sender email: "+err.Error(), nil)
	}

	// Validate recipients
	if err := ValidateEmails(toEmails); err != nil {
		return errors.NewValidationError("invalid recipient emails: "+err.Error(), nil)
	}

	// Validate subject
	if subject == "" {
		return errors.NewValidationError("subject cannot be empty", nil)
	}

	// Validate content (either text or HTML content is required)
	if content == "" {
		return errors.NewValidationError("email content cannot be empty", nil)
	}

	return nil
}
