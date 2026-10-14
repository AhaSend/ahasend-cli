package messages

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	ahasend "github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/common"
)

// readRecipients loads the recipients from the recipients file, or from the
// --to addresses
func readRecipients(recipientsFile string, toEmails []string) ([]common.Recipient, error) {
	if recipientsFile != "" {
		return loadRecipientsFromFile(recipientsFile)
	}

	if len(toEmails) == 0 {
		return nil, errors.NewValidationError("at least one recipient is required: use --to or --recipients", nil)
	}
	return createRecipientsFromEmails(toEmails)
}

// loadRecipientsFromFile loads recipients from JSON or CSV file
func loadRecipientsFromFile(filePath string) ([]common.Recipient, error) {
	data, err := cmdutil.ReadInputFile("recipients", filePath)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		return loadRecipientsFromJSON(bytes.NewReader(data))
	case ".csv":
		return loadRecipientsFromCSV(bytes.NewReader(data))
	default:
		return nil, errors.NewValidationError(fmt.Sprintf("unsupported recipients file format %s (supported: .json, .csv)", ext), nil)
	}
}

// errNoRecipients is the error of a recipients file that holds no recipient.
var errNoRecipients = errors.NewValidationError("--recipients file has no recipients", nil)

// recipientData represents a single recipient with their email, name and substitution data
type recipientData struct {
	Email         string                 `json:"email"`
	Name          string                 `json:"name,omitempty"`
	Substitutions map[string]interface{} `json:"substitution_data,omitempty"`
	// LegacySubstitutions reads the "substitutions" key, which earlier
	// releases read instead of the documented "substitution_data".
	LegacySubstitutions map[string]interface{} `json:"substitutions,omitempty"`
	// The keys a failed recipients file adds to say why a recipient failed,
	// which a send of the file again ignores.
	FailedError     json.RawMessage `json:"_error,omitempty"`
	FailedErrorCode json.RawMessage `json:"_error_code,omitempty"`
	FailedRetryable json.RawMessage `json:"_retryable,omitempty"`
	// The key the file of recipients whose outcome is unknown adds to name
	// the idempotency key of their request.
	FailedIdempotencyKey json.RawMessage `json:"_idempotency_key,omitempty"`
}

// loadRecipientsFromJSON parses recipients from JSON file. A key that
// recipientData does not name is an error, so a misspelt key does not drop
// what it holds. Each recipient is decoded on its own, so an error names its
// index the same way whatever the Go version's decoder puts in its errors.
func loadRecipientsFromJSON(file io.Reader) ([]common.Recipient, error) {
	var rawRecipients []json.RawMessage
	if err := cmdutil.DecodeJSON(file, &rawRecipients, "--recipients", "JSON array"); err != nil {
		return nil, err
	}
	if len(rawRecipients) == 0 {
		return nil, errNoRecipients
	}
	recipientData := make([]recipientData, len(rawRecipients))
	for i, raw := range rawRecipients {
		subject := fmt.Sprintf("--recipients index %d", i)
		if err := cmdutil.DecodeJSONStrict(bytes.NewReader(raw), &recipientData[i], subject, "JSON object"); err != nil {
			return nil, err
		}
	}

	var recipients []common.Recipient
	for i, data := range recipientData {
		if !validation.IsEmailAddress(data.Email) {
			return nil, errors.NewValidationError(fmt.Sprintf("--recipients index %d: %q is not an email address", i, data.Email), nil)
		}

		recipient := common.Recipient{
			Email: data.Email,
		}
		if data.Name != "" {
			recipient.Name = ahasend.String(data.Name)
		}
		if len(data.Substitutions) > 0 && len(data.LegacySubstitutions) > 0 {
			return nil, errors.NewValidationError(fmt.Sprintf("recipient at index %d sets both substitution_data and substitutions; use substitution_data", i), nil)
		}
		if len(data.Substitutions) > 0 {
			recipient.Substitutions = data.Substitutions
		} else if len(data.LegacySubstitutions) > 0 {
			recipient.Substitutions = data.LegacySubstitutions
		}
		recipients = append(recipients, recipient)
	}

	return recipients, nil
}

// loadRecipientsFromCSV parses recipients from CSV file
func loadRecipientsFromCSV(file io.Reader) ([]common.Recipient, error) {
	reader := csv.NewReader(file)
	// A row may leave out empty optional columns at its end
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, errors.NewValidationError("failed to parse --recipients CSV", err)
	}

	if len(records) < 2 { // Need header + at least one data row
		return nil, errNoRecipients
	}

	headers := records[0]
	var emailIndex, nameIndex = -1, -1

	// Find required columns
	for i, header := range headers {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "email":
			emailIndex = i
		case "name":
			nameIndex = i
		}
	}

	if emailIndex == -1 {
		return nil, errors.NewValidationError("CSV file must have an 'email' column", nil)
	}

	var recipients []common.Recipient
	for i, record := range records[1:] { // Skip header row
		if len(record) <= emailIndex {
			return nil, errors.NewValidationError(fmt.Sprintf("--recipients row %d has no email column", i+2), nil)
		}
		if len(record) > len(headers) {
			return nil, errors.NewValidationError(fmt.Sprintf("--recipients row %d has %d columns, more than the %d of the header",
				i+2, len(record), len(headers)), nil)
		}

		email := strings.TrimSpace(record[emailIndex])
		if !validation.IsEmailAddress(email) {
			return nil, errors.NewValidationError(fmt.Sprintf("--recipients row %d: %q is not an email address", i+2, email), nil)
		}

		recipient := common.Recipient{
			Email: email,
		}

		// Set name if column exists
		if nameIndex >= 0 && len(record) > nameIndex {
			name := strings.TrimSpace(record[nameIndex])
			if name != "" {
				recipient.Name = ahasend.String(name)
			}
		}

		// Create substitution data from remaining columns
		substitutionData := make(map[string]interface{})
		for j, header := range headers {
			if j != emailIndex && j != nameIndex && j < len(record) {
				key := strings.TrimSpace(header)
				value := strings.TrimSpace(record[j])
				if key != "" && value != "" {
					substitutionData[key] = value
				}
			}
		}

		if len(substitutionData) > 0 {
			recipient.Substitutions = substitutionData
		}

		recipients = append(recipients, recipient)
	}

	return recipients, nil
}

// createRecipientsFromEmails creates basic recipients from email addresses
func createRecipientsFromEmails(emails []string) ([]common.Recipient, error) {
	var recipients []common.Recipient
	for _, email := range emails {
		if err := validation.ValidateEmail("--to", email); err != nil {
			return nil, err
		}
		recipients = append(recipients, common.Recipient{
			Email: email,
		})
	}
	return recipients, nil
}

// loadGlobalSubstitutions loads global substitution variables from JSON file
func loadGlobalSubstitutions(filePath string) (map[string]interface{}, error) {
	data, err := cmdutil.ReadInputFile("global-substitutions", filePath)
	if err != nil {
		return nil, err
	}

	var substitutions map[string]interface{}
	if err := cmdutil.DecodeJSON(bytes.NewReader(data), &substitutions, "--global-substitutions", "JSON object"); err != nil {
		return nil, err
	}

	return substitutions, nil
}

// mergeGlobalSubstitutions copies the global substitutions into every
// recipient's substitutions, since the template endpoint has no request-level
// substitutions. A recipient's own value for the same key wins.
func mergeGlobalSubstitutions(recipients []common.Recipient, global map[string]interface{}) []common.Recipient {
	if len(global) == 0 {
		return recipients
	}

	merged := make([]common.Recipient, len(recipients))
	for i, recipient := range recipients {
		substitutions := make(map[string]interface{}, len(global)+len(recipient.Substitutions))
		for key, value := range global {
			substitutions[key] = value
		}
		for key, value := range recipient.Substitutions {
			substitutions[key] = value
		}
		recipient.Substitutions = substitutions
		merged[i] = recipient
	}
	return merged
}

// maxBatchSize is the number of recipients each send request holds
const maxBatchSize = 100

// recipientBatches splits recipients into consecutive batches of up to
// maxBatchSize, one per send request.
func recipientBatches(recipients []common.Recipient) [][]common.Recipient {
	var batches [][]common.Recipient
	for i := 0; i < len(recipients); i += maxBatchSize {
		batches = append(batches, recipients[i:min(i+maxBatchSize, len(recipients))])
	}
	return batches
}
