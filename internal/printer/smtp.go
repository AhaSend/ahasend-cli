package printer

import (
	"fmt"
	"io"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// smtpHeaders names the table column or row of each SMTP credential field.
var smtpHeaders = map[string]string{
	"name":       "Name",
	"id":         "ID",
	"username":   "Username",
	"password":   "Password",
	"scope":      "Scope",
	"domains":    "Domains",
	"sandbox":    "Sandbox",
	"created_at": "Created",
	"updated_at": "Updated",
}

// The fields of the views of an SMTP credential, which the table, plain and
// CSV output of each view show in this order, except a table of credentials,
// which shows smtpTableColumns.
var (
	// smtpListFields are the fields of each credential of a list.
	smtpListFields = []string{"name", "id", "username", "scope", "domains", "sandbox", "created_at", "updated_at"}
	// smtpTableColumns are the fields a table of credentials shows.
	smtpTableColumns = []string{"name", "id", "username", "scope", "sandbox", "created_at"}
	// smtpDetailFields are the fields of a credential's detail view. Table
	// and plain output show the password as [HIDDEN], and CSV output leaves
	// it out: the API returns it only on creation.
	smtpDetailFields = []string{"name", "id", "username", "password", "scope", "domains", "sandbox", "created_at", "updated_at"}
	// createdSMTPFields are the fields of a new credential, which hold its
	// password: the API shows it only on creation.
	createdSMTPFields = []string{"name", "id", "username", "password", "scope", "domains", "sandbox", "created_at"}
)

// smtpServer is the host mail clients send through with a credential.
const smtpServer = "send.ahasend.com"

// savedSMTPPasswordHint is the password of the settings of a credential
// shown after its creation, which the API no longer returns.
const savedSMTPPasswordHint = "[Use the password provided during creation]"

func (h *jsonHandler) HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error {
	return h.printAPIResponse(credential)
}

func (h *jsonHandler) HandleCreateSMTP(credential *responses.SMTPCredential, config ResultConfig) error {
	return h.printAPIResponse(credential)
}

// smtpSendJSON and smtpTestJSON are the JSON output of smtp send, a sent
// message or a test's result, with the keys of its CSV output: no API answers
// smtp send, so it prints its own result.
type smtpSendJSON struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id"`
}

type smtpTestJSON struct {
	TestMode      bool   `json:"test_mode"`
	Success       bool   `json:"success"`
	Connected     bool   `json:"connected"`
	Authenticated bool   `json:"authenticated"`
	Error         string `json:"error"`
}

func (h *jsonHandler) HandleSMTPSend(result *SMTPSendResult, config ResultConfig) error {
	if result == nil {
		return h.HandleEmpty("No send result available")
	}
	if result.TestMode {
		return h.printJSON(smtpTestJSON{
			TestMode:      true,
			Success:       result.Success,
			Connected:     result.Connected,
			Authenticated: result.Authenticated,
			Error:         result.Error,
		})
	}
	return h.printJSON(smtpSendJSON{Success: result.Success, MessageID: result.MessageID})
}

func (h *tableHandler) HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error {
	return writeTableList(h, config, pageOf(response), "SMTP credentials", smtpTableColumns, smtpHeaders, func(credential *responses.SMTPCredential) map[string]string {
		return smtpDisplayFields(credential, "")
	})
}

func (h *tableHandler) HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error {
	if credential == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(fieldRows(smtpDetailFields, smtpHeaders, smtpDisplayFields(credential, hiddenSecret)))

	return h.renderSMTPSettings(smtpSettingsRows(credential, savedSMTPPasswordHint))
}

// HandleCreateSMTP shows the credential's password, which the API shows only
// on creation.
func (h *tableHandler) HandleCreateSMTP(credential *responses.SMTPCredential, config ResultConfig) error {
	if credential == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(fieldRows(createdSMTPFields, smtpHeaders, smtpDisplayFields(credential, credential.Password)))
	if err := writeSMTPPasswordNotice(h.writer, credential); err != nil {
		return err
	}

	return h.renderSMTPSettings(smtpSettingsRows(credential, createdSMTPPasswordHint(credential)))
}

// renderSMTPSettings renders the settings a mail client needs to send with a
// credential.
func (h *tableHandler) renderSMTPSettings(rows [][]string) error {
	if _, err := fmt.Fprintf(h.writer, "\nSMTP Settings:\n\n"); err != nil {
		return err
	}
	h.renderTable([]any{"Setting", "Value"}, rows...)
	return nil
}

// smtpSettingsRows returns the connection settings of a credential, with
// password as the password to use.
func smtpSettingsRows(credential *responses.SMTPCredential, password string) [][]string {
	return [][]string{
		{"Server", smtpServer},
		{"Port", "587 (STARTTLS, recommended); 25 and 2525 also support STARTTLS"},
		{"Username", credential.Username},
		{"Password", password},
		{"Authentication", "Plain or Login"},
	}
}

func (h *tableHandler) HandleSMTPSend(result *SMTPSendResult, config ResultConfig) error {
	if result == nil {
		return h.HandleEmpty("No send result")
	}

	if result.TestMode {
		if _, err := fmt.Fprintf(h.writer, "SMTP connection test result\n\n"); err != nil {
			return err
		}
		h.renderTable([]any{"Test", "Result"}, smtpTestRows(result)...)
		return nil
	}

	// A send that fails returns its error without printing a result.
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable([][]string{
		{"Status", "Sent"},
		{"Message ID", result.MessageID},
		{"Delivery", "Message queued for delivery"},
	})
	return nil
}

func (h *plainHandler) HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error {
	return writePlainList(h, config, pageOf(response), "SMTP credentials", func(credential *responses.SMTPCredential) error {
		return h.writeFieldLines(smtpListFields, smtpHeaders, smtpDisplayFields(credential, ""))
	})
}

func (h *plainHandler) HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error {
	if credential == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", fieldRows(smtpDetailFields, smtpHeaders, smtpDisplayFields(credential, hiddenSecret))); err != nil {
		return err
	}

	return writeSMTPSettings(h.writer, credential, savedSMTPPasswordHint)
}

// HandleCreateSMTP shows the credential's password, which the API shows only
// on creation.
func (h *plainHandler) HandleCreateSMTP(credential *responses.SMTPCredential, config ResultConfig) error {
	if credential == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", fieldRows(createdSMTPFields, smtpHeaders, smtpDisplayFields(credential, credential.Password))); err != nil {
		return err
	}
	if err := writeSMTPPasswordNotice(h.writer, credential); err != nil {
		return err
	}

	return writeSMTPSettings(h.writer, credential, createdSMTPPasswordHint(credential))
}

// writeSMTPSettings writes the settings a mail client needs to send with the
// credential.
func writeSMTPSettings(w io.Writer, credential *responses.SMTPCredential, password string) error {
	if _, err := fmt.Fprintf(w, "\nSMTP Settings:\n"); err != nil {
		return err
	}
	return writeLabelLines(w, "  ", smtpSettingsRows(credential, password))
}

func (h *plainHandler) HandleSMTPSend(result *SMTPSendResult, config ResultConfig) error {
	if result == nil {
		return h.HandleEmpty("No send result")
	}

	if result.TestMode {
		if _, err := fmt.Fprintf(h.writer, "SMTP Test Result:\n"); err != nil {
			return err
		}
		return writeLabelLines(h.writer, "  ", smtpTestRows(result))
	}

	// A send that fails returns its error without printing a result.
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", [][]string{{"Message ID", result.MessageID}})
}

func (h *csvHandler) HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error {
	return writeCSVList(h, pageOf(response), smtpListFields, smtpCSVFields)
}

func (h *csvHandler) HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error {
	if credential == nil {
		return nil // No CSV output for nil credential
	}
	return h.writeFieldMaps(withoutField(smtpDetailFields, "password"), smtpCSVFields(credential))
}

// HandleCreateSMTP writes the credential with its password, which the API
// shows only on creation.
func (h *csvHandler) HandleCreateSMTP(credential *responses.SMTPCredential, config ResultConfig) error {
	if credential == nil {
		return nil
	}
	fields := smtpCSVFields(credential)
	fields["password"] = credential.Password
	return h.writeFieldMaps(createdSMTPFields, fields)
}

func smtpCSVFields(credential *responses.SMTPCredential) map[string]string {
	return map[string]string{
		"id":         formatUUID(credential.ID),
		"name":       credential.Name,
		"username":   credential.Username,
		"scope":      credential.Scope,
		"domains":    formatStringSlice(credential.Domains),
		"sandbox":    formatBool(credential.Sandbox),
		"created_at": formatCSVTime(credential.CreatedAt),
		"updated_at": formatCSVTime(credential.UpdatedAt),
	}
}

func (h *csvHandler) HandleSMTPSend(result *SMTPSendResult, config ResultConfig) error {
	if result == nil {
		return nil
	}

	if result.TestMode {
		return h.writeRows([]string{"test_mode", "success", "connected", "authenticated", "error"},
			[]string{"true", formatBool(result.Success), formatBool(result.Connected), formatBool(result.Authenticated), result.Error})
	}
	// A send that fails returns its error without printing a result.
	return h.writeRows([]string{"success", "message_id"},
		[]string{formatBool(result.Success), result.MessageID})
}

// smtpDisplayFields maps each SMTP credential field to its table and plain
// value, with password as the value of the password field.
func smtpDisplayFields(credential *responses.SMTPCredential, password string) map[string]string {
	return map[string]string{
		"name":       credential.Name,
		"id":         formatUUID(credential.ID),
		"username":   credential.Username,
		"password":   password,
		"scope":      credential.Scope,
		"domains":    formatDomains(credential.Domains),
		"sandbox":    formatBooleanStatus(credential.Sandbox),
		"created_at": formatTime(credential.CreatedAt),
		"updated_at": formatTime(credential.UpdatedAt),
	}
}

// createdSMTPPasswordHint tells which password a new credential's settings
// use: the one shown, or a saved one when the API returned none.
func createdSMTPPasswordHint(credential *responses.SMTPCredential) string {
	if credential.Password != "" {
		return "[Use the password shown above]"
	}
	return "[Use your saved password]"
}

// writeSMTPPasswordNotice tells to save the password of a new credential,
// when the API returned one.
func writeSMTPPasswordNotice(w io.Writer, credential *responses.SMTPCredential) error {
	if credential.Password == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "\n%s\n", smtpPasswordNotice)
	return err
}

// smtpTestRows returns the rows of an SMTP test's result: whether it
// connected and logged in, and the error of a failed one.
func smtpTestRows(result *SMTPSendResult) [][]string {
	switch {
	case result.Success:
		authentication := "Not tested (no --username and --password)"
		if result.Authenticated {
			authentication = "Passed"
		}
		return [][]string{
			{"Connection", "Successful"},
			{"Authentication", authentication},
			{"Status", "No message was sent"},
		}
	case result.Connected:
		return [][]string{
			{"Connection", "Successful"},
			{"Authentication", "Failed"},
			{"Error", result.Error},
			{"Status", "Please check your SMTP username and password"},
		}
	default:
		return [][]string{
			{"Connection", "Failed"},
			{"Error", result.Error},
			{"Status", "Please check your SMTP settings"},
		}
	}
}
