package printer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// NoProfilesMessage is what the auth commands print when the configuration
// file holds no profile.
const NoProfilesMessage = "No authentication profiles found"

// authResultJSON is the JSON object auth login, switch and logout print.
type authResultJSON struct {
	Profile string `json:"profile"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (h *jsonHandler) HandleAuthResult(result AuthResult) error {
	return h.printJSON(authResultJSON{Profile: result.Profile, Success: true, Message: result.Message})
}

// authStatusJSON is the JSON object auth status prints. Valid is null when
// the check could not tell whether the API accepts the credentials, Error
// says why the check failed, Note why a valid key has no account, and
// Account is the API's account response, or null when it could not be
// fetched.
type authStatusJSON struct {
	Profile   string          `json:"profile"`
	AccountID string          `json:"account_id"`
	Valid     *bool           `json:"valid"`
	Error     string          `json:"error,omitempty"`
	Note      string          `json:"note,omitempty"`
	Account   json.RawMessage `json:"account"`
}

func (h *jsonHandler) HandleAuthStatus(status *AuthStatus) error {
	if status == nil {
		return h.HandleEmpty("No authentication status available")
	}
	output := authStatusJSON{
		Profile:   status.Profile,
		AccountID: status.AccountID,
		Valid:     status.Valid,
		Error:     status.Error,
		Note:      status.Note,
		Account:   json.RawMessage("null"),
	}
	if status.Account != nil {
		if body, ok := apiresponse.Body(status.Account); ok && len(bytes.TrimSpace(body)) > 0 {
			output.Account = body
		} else {
			encoded, err := encodeJSON(status.Account, "")
			if err != nil {
				return err
			}
			output.Account = encoded
		}
	}
	return h.printJSON(output)
}

// HandleAuthResult prints the result's message, which names the profile.
func (h *tableHandler) HandleAuthResult(result AuthResult) error {
	_, err := fmt.Fprintln(h.writer, result.Message)
	return err
}

func (h *tableHandler) HandleAuthStatus(status *AuthStatus) error {
	if status == nil {
		return h.HandleEmpty("No authentication status available")
	}

	if _, err := fmt.Fprintf(h.writer, "Authentication Status\n\n"); err != nil {
		return err
	}
	h.renderFieldTable(authStatusRows(status))

	account := status.Account
	if account == nil {
		return nil
	}
	if _, err := fmt.Fprintf(h.writer, "\nAccount Details:\n\n"); err != nil {
		return err
	}
	h.renderFieldTable(accountDetailRows(account))

	if rows := accountEmailSettingRows(account); len(rows) > 0 {
		if _, err := fmt.Fprintf(h.writer, "\nEmail Settings:\n\n"); err != nil {
			return err
		}
		h.renderTable([]any{"Setting", "Value"}, rows...)
	}

	if rows := accountRetentionRows(account); len(rows) > 0 {
		if _, err := fmt.Fprintf(h.writer, "\nData Retention:\n\n"); err != nil {
			return err
		}
		h.renderFieldTable(rows)
	}
	return nil
}

// HandleAuthResult prints the result's message, which names the profile.
func (h *plainHandler) HandleAuthResult(result AuthResult) error {
	_, err := fmt.Fprintln(h.writer, result.Message)
	return err
}

func (h *plainHandler) HandleAuthStatus(status *AuthStatus) error {
	if status == nil {
		return h.HandleEmpty("No authentication status available")
	}

	if _, err := fmt.Fprintf(h.writer, "Authentication Status\n"); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", authStatusRows(status)); err != nil {
		return err
	}

	account := status.Account
	if account == nil {
		return nil
	}
	if _, err := fmt.Fprintf(h.writer, "\nAccount Details:\n"); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "  ", accountDetailRows(account)); err != nil {
		return err
	}

	if rows := accountEmailSettingRows(account); len(rows) > 0 {
		if _, err := fmt.Fprintf(h.writer, "\nEmail Settings:\n"); err != nil {
			return err
		}
		if err := writeLabelLines(h.writer, "  ", rows); err != nil {
			return err
		}
	}

	if rows := accountRetentionRows(account); len(rows) > 0 {
		if _, err := fmt.Fprintf(h.writer, "\nData Retention:\n"); err != nil {
			return err
		}
		if err := writeLabelLines(h.writer, "  ", rows); err != nil {
			return err
		}
	}
	return nil
}

func (h *csvHandler) HandleAuthResult(result AuthResult) error {
	return h.writeRows([]string{"profile", "success", "message"}, []string{result.Profile, "true", result.Message})
}

func (h *csvHandler) HandleAuthStatus(status *AuthStatus) error {
	if status == nil {
		return nil // No CSV output for empty status
	}

	fieldMap := map[string]string{
		"profile":    status.Profile,
		"account_id": status.AccountID,
		"valid":      formatOptionalBool(status.Valid),
		"error":      status.Error,
		"note":       status.Note,
	}

	if account := status.Account; account != nil {
		fieldMap["account_id"] = formatUUID(account.ID)
		fieldMap["parent_account_id"] = formatOptionalUUID(account.ParentAccountID)
		fieldMap["account_name"] = account.Name
		fieldMap["website"] = formatOptionalString(account.Website)
		fieldMap["about"] = formatOptionalString(account.About)
		fieldMap["created_at"] = formatCSVTime(account.CreatedAt)
		fieldMap["updated_at"] = formatCSVTime(account.UpdatedAt)
		fieldMap["track_opens"] = formatOptionalBool(account.TrackOpens)
		fieldMap["track_clicks"] = formatOptionalBool(account.TrackClicks)
		fieldMap["reject_bad_recipients"] = formatOptionalBool(account.RejectBadRecipients)
		fieldMap["reject_mistyped_recipients"] = formatOptionalBool(account.RejectMistypedRecipients)
		if account.MessageMetadataRetention != nil {
			fieldMap["message_metadata_retention"] = formatInt(int(*account.MessageMetadataRetention))
		}
		if account.MessageDataRetention != nil {
			fieldMap["message_data_retention"] = formatInt(int(*account.MessageDataRetention))
		}
	}

	return h.writeFieldMaps([]string{
		"profile", "account_id", "valid", "error", "note", "parent_account_id", "account_name",
		"website", "about", "created_at", "updated_at", "track_opens",
		"track_clicks", "reject_bad_recipients", "reject_mistyped_recipients",
		"message_metadata_retention", "message_data_retention",
	}, fieldMap)
}

// HandleAuthProfiles prints the profile names as {"profiles": [...]}.
func (h *jsonHandler) HandleAuthProfiles(profiles []string) error {
	if profiles == nil {
		profiles = []string{}
	}
	return h.printJSON(map[string][]string{"profiles": profiles})
}

// HandleAuthProfiles prints the line that lists the profiles, such as
// "Found 2 profiles: default, production".

func (h *tableHandler) HandleAuthProfiles(profiles []string) error {
	_, err := fmt.Fprintln(h.writer, profileSummary(profiles))
	return err
}

func (h *plainHandler) HandleAuthProfiles(profiles []string) error {
	_, err := fmt.Fprintln(h.writer, profileSummary(profiles))
	return err
}

// HandleAuthProfiles writes a profile column with one row per profile.
func (h *csvHandler) HandleAuthProfiles(profiles []string) error {
	rows := make([]map[string]string, len(profiles))
	for i, profile := range profiles {
		rows[i] = map[string]string{"profile": profile}
	}
	return h.writeFieldMaps([]string{"profile"}, rows...)
}

// profileSummary lists the profile names, for example
// "Found 2 profiles: default, production", or says there are none.
func profileSummary(profiles []string) string {
	if len(profiles) == 0 {
		return NoProfilesMessage
	}
	noun := "profiles"
	if len(profiles) == 1 {
		noun = "profile"
	}
	return fmt.Sprintf("Found %d %s: %s", len(profiles), noun, strings.Join(profiles, ", "))
}

// authStatusRows returns the profile, account ID and validity of a status,
// and its note. A failed check is not a row: auth status prints it as its
// error line on stderr, as every command prints its error.
func authStatusRows(status *AuthStatus) [][]string {
	// Unknown validity is a missing value
	valid := ""
	if status.Valid != nil {
		valid = formatBooleanStatus(*status.Valid)
	}
	rows := [][]string{
		{"Profile", status.Profile},
		{"Account ID", status.AccountID},
		{"Valid", valid},
	}
	if status.Note != "" {
		rows = append(rows, []string{"Note", status.Note})
	}
	return rows
}

// accountDetailRows returns the rows of an account's details. An optional
// field the API left out has no row.
func accountDetailRows(account *responses.Account) [][]string {
	rows := [][]string{{"ID", formatUUID(account.ID)}}
	if account.ParentAccountID != nil {
		rows = append(rows, []string{"Parent Account ID", formatUUID(*account.ParentAccountID)})
	}
	rows = append(rows, []string{"Name", account.Name})
	if account.Website != nil {
		rows = append(rows, []string{"Website", *account.Website})
	}
	if account.About != nil {
		rows = append(rows, []string{"About", *account.About})
	}
	return append(rows,
		[]string{"Created", formatTime(account.CreatedAt)},
		[]string{"Updated", formatTime(account.UpdatedAt)},
	)
}

// accountEmailSettingRows returns a row for each email setting the API
// returned for an account.
func accountEmailSettingRows(account *responses.Account) [][]string {
	var rows [][]string
	for _, setting := range []struct {
		label string
		value *bool
	}{
		{"Track Opens", account.TrackOpens},
		{"Track Clicks", account.TrackClicks},
		{"Reject Bad Recipients", account.RejectBadRecipients},
		{"Reject Mistyped Recipients", account.RejectMistypedRecipients},
	} {
		if setting.value != nil {
			rows = append(rows, []string{setting.label, formatBooleanStatus(*setting.value)})
		}
	}
	return rows
}

// accountRetentionRows returns a row with the number of days, such as
// "30 days", for each retention period the API returned for an account.
func accountRetentionRows(account *responses.Account) [][]string {
	var rows [][]string
	if account.MessageMetadataRetention != nil {
		rows = append(rows, []string{"Message Metadata", formatCount(int(*account.MessageMetadataRetention), "days")})
	}
	if account.MessageDataRetention != nil {
		rows = append(rows, []string{"Message Data", formatCount(int(*account.MessageDataRetention), "days")})
	}
	return rows
}
