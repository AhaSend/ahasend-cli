package smtp

import (
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gopkg.in/gomail.v2"
)

// NewSendCommand creates the smtp send command
func NewSendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send an email via SMTP protocol",
		Long: `Send an email using SMTP protocol with AhaSend's SMTP server.

This command tests SMTP sending directly from the CLI with options like the
regular 'messages send' command.

--from, a recipient (--to, --cc or --bcc), --subject, content (--text,
--html, --text-file or --html-file), --username and --password are required.
When one is missing and stdin is a terminal, the command asks for it;
--non-interactive never asks.

Other users of this machine can see --password in the process list, and
the shell may keep it in its history. --password-stdin reads the password
from the first line of stdin instead, without its line break; the command
then asks for nothing, since stdin holds the password.

Special headers can be used to control AhaSend features:
- AhaSend-Track-Opens: true/false
- AhaSend-Track-Clicks: true/false
- AhaSend-Tags: comma-separated tags
- AhaSend-Sandbox: true/false
- AhaSend-Sandbox-Result: deliver/bounce/defer/fail/suppress
The flags --track-opens, --track-clicks, --tag, --sandbox and --sandbox-result
set them; --header cannot set one whose flag is given.

--header cannot set the headers the command builds itself: From, Sender,
To, Cc, Bcc and Subject, which the flags set and the SMTP envelope is built
from, and Mime-Version, Content-Type and Content-Transfer-Encoding. It may
set Date and Message-ID. Without a Message-ID, the command sets one, a UUID
at the domain of --from, and prints it; messages get finds the message by
it. Header names ignore case, and each may be given once.

When the server refuses any recipient, nothing is sent, and the error names
the recipient.

The command starts TLS: at once on port 465, otherwise with STARTTLS when
the server offers it. It never sends --username, --password or the message
over a connection without TLS, except to localhost, 127.0.0.1 or ::1: when
the server offers no STARTTLS, it fails before logging in. It also fails
when the server offers no AUTH. The connection, TLS and the login must end
within 30 seconds, and sending the message within 5 minutes.

--test checks the connection without sending: it connects to --server, says
EHLO, starts TLS when the server offers it, logs in when --username and
--password are given, and quits. It needs only --server, and --username and
--password together to test the login; it sends no message, so the message
flags have no effect.`,
		Example: `  # Send an email, asking for every required value
  ahasend smtp send

  # Send simple text email via SMTP
  ahasend smtp send \
    --from sender@example.com \
    --to recipient@example.com \
    --subject "Test Email" \
    --text "This is a test email"

  # Send with HTML content
  ahasend smtp send \
    --from sender@example.com \
    --to recipient@example.com \
    --subject "HTML Email" \
    --html "<h1>Hello</h1><p>This is HTML content</p>"

  # Send with attachments
  ahasend smtp send \
    --from sender@example.com \
    --to recipient@example.com \
    --subject "Email with Attachment" \
    --text "Please find the attachment" \
    --attach document.pdf

  # Test the connection and the login without sending
  ahasend smtp send --test --username smtp-user --password smtp-pass

  # Send in sandbox mode simulating a bounce
  ahasend smtp send \
    --from sender@example.com \
    --to recipient@example.com \
    --subject "Test Bounce" \
    --text "This will simulate a bounce" \
    --sandbox \
    --sandbox-result bounce \
    --username smtp-user \
    --password smtp-pass

  # Use custom SMTP server, with the password from an environment variable
  printf '%s\n' "$SMTP_PASSWORD" | ahasend smtp send \
    --server mail.example.com:587 \
    --username user \
    --password-stdin \
    --from sender@example.com \
    --to recipient@example.com \
    --subject "Custom Server Test" \
    --text "Sent through a custom SMTP server"`,
		Args:         cmdutil.NoArgs,
		RunE:         runSMTPSend,
		SilenceUsage: true,
	}

	// Email content flags
	cmd.Flags().String("from", "", cmdutil.FromUsage+" (required)")
	cmdutil.AddRepeatableFlag(cmd, "to", cmdutil.ToUsage)
	cmdutil.AddRepeatableFlag(cmd, "cc", "CC recipient (can be used multiple times)")
	cmdutil.AddRepeatableFlag(cmd, "bcc", "BCC recipient (can be used multiple times)")
	cmd.Flags().String("subject", "", cmdutil.SubjectUsage+" (required)")
	cmd.Flags().String("text", "", cmdutil.TextUsage)
	cmd.Flags().String("html", "", cmdutil.HTMLUsage)
	cmd.Flags().String("text-file", "", cmdutil.TextFileUsage)
	cmd.Flags().String("html-file", "", cmdutil.HTMLFileUsage)
	cmdutil.AddRepeatableFlag(cmd, "attach", cmdutil.AttachUsage+" (can be used multiple times)")
	for _, name := range []string{"text-file", "html-file", "attach"} {
		cmdutil.MarkFileFlag(cmd, name)
	}

	// SMTP server flags
	cmd.Flags().String("server", "send.ahasend.com:587", "SMTP server address")
	cmd.Flags().String("username", "", "SMTP username (required, except by --test)")
	cmd.Flags().String("password", "", "SMTP password (required, except by --test); other users of this machine can see it in the process list, so prefer --password-stdin")
	prompt.AddStdinFlag(cmd, "password-stdin", "Read the SMTP password from the first line of stdin, in place of --password; the command then asks for nothing")
	cmd.Flags().String("credential-id", "", "")
	cmdutil.MarkNoEffect(cmd, "credential-id", "use --username and --password")

	// Special headers for AhaSend features
	cmd.Flags().Bool("track-opens", false, cmdutil.TrackOpensUsage)
	cmd.Flags().Bool("track-clicks", false, cmdutil.TrackClicksUsage)
	cmdutil.AddRepeatableFlag(cmd, "tag", cmdutil.TagUsage)
	cmdutil.AddFlagAlias(cmd, "tags", "tag")
	cmd.Flags().Bool("sandbox", false, cmdutil.SandboxUsage)
	cmdutil.AddOneOfFlag(cmd, "sandbox-result", "deliver", cmdutil.SandboxResultUsage, cmdutil.SandboxResults)
	cmdutil.AddRepeatableFlag(cmd, "header", cmdutil.HeaderUsage+
		"; not From, Sender, To, Cc, Bcc, Subject or the MIME headers (can be used multiple times)")

	// Test mode
	cmd.Flags().Bool("test", false, "Connect to --server, and log in when --username and --password are given, then quit without sending")
	prompt.AddNonInteractiveFlag(cmd)

	return cmd
}

func runSMTPSend(cmd *cobra.Command, args []string) error {
	// Get printer instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	send, err := readSMTPSend(cmd)
	if err != nil {
		return err
	}
	if send.testMode {
		return testSMTPConnection(cmd, handler, send.host, send.port)
	}

	logger.Get().WithFields(map[string]interface{}{
		"server": send.server,
		"from":   send.message.from,
		"to":     send.message.to,
	}).Debug("Sending email via SMTP with gomail")

	// Send the email
	messageID := ensureMessageID(&send.message)
	session, err := connectSMTP(send.host, send.port)
	if err == nil {
		if err = session.logIn(send.username, send.password); err != nil {
			session.close()
		} else {
			err = session.send(buildGomailMessage(send.message))
		}
	}
	// A connection that failed after the message was sent is a network
	// failure, and the message may have been accepted, so it is not "failed".
	var unknown *unknownOutcomeError
	if stderrors.As(err, &unknown) {
		return errors.NewNetworkError(unknown.Error(), nil)
	}
	if err != nil {
		return &smtpError{action: "SMTP send failed", err: sessionFailure(err)}
	}

	return handler.HandleSMTPSend(&printer.SMTPSendResult{Success: true, MessageID: messageID}, printer.ResultConfig{
		SuccessMessage: "Email sent successfully via SMTP",
	})
}

// smtpSend is the input of smtp send: the server, the login and the
// message. With --test it holds only the server.
type smtpSend struct {
	server             string
	host               string
	port               int
	testMode           bool
	username, password string
	message            smtpMessage
}

// smtpMessage is the message smtp send sends, with the AhaSend options it
// sets as headers.
type smtpMessage struct {
	from          string
	to, cc, bcc   []string
	subject       string
	text, html    string
	attachments   []cmdutil.Attachment
	headers       map[string]string
	trackOpens    *bool
	trackClicks   *bool
	tags          []string
	sandbox       bool
	sandboxResult string
}

// readSMTPSend reads and checks the flags of smtp send, reads the content
// and attachment files, and asks for the missing required values when
// stdin is a terminal. With --test it stops once the server is read.
func readSMTPSend(cmd *cobra.Command) (*smtpSend, error) {
	for _, pair := range [][2]string{{"text", "text-file"}, {"html", "html-file"}, {"password", "password-stdin"}} {
		if err := cmdutil.RejectTogether(cmd, pair[0], pair[1]); err != nil {
			return nil, err
		}
	}
	repeatable := map[string][]string{}
	for _, name := range []string{"to", "cc", "bcc", "tag"} {
		values, err := cmdutil.ReadRepeatable(cmd, name, "")
		if err != nil {
			return nil, err
		}
		repeatable[name] = values
	}
	message := smtpMessage{to: repeatable["to"], cc: repeatable["cc"], bcc: repeatable["bcc"], tags: repeatable["tag"]}
	attachPaths, err := cmdutil.ReadAttachPaths(cmd)
	if err != nil {
		return nil, err
	}
	if message.headers, err = cmdutil.ReadHeaders(cmd, "header"); err != nil {
		return nil, err
	}
	if message.sandboxResult, err = cmdutil.ReadSandboxResult(cmd); err != nil {
		return nil, err
	}

	message.text, _ = cmd.Flags().GetString("text")
	message.html, _ = cmd.Flags().GetString("html")
	textFile, _ := cmd.Flags().GetString("text-file")
	htmlFile, _ := cmd.Flags().GetString("html-file")

	send := &smtpSend{}
	send.server, _ = cmd.Flags().GetString("server")

	message.trackOpens = cmdutil.ChangedBool(cmd, "track-opens")
	message.trackClicks = cmdutil.ChangedBool(cmd, "track-clicks")
	message.sandbox, _ = cmd.Flags().GetBool("sandbox")
	send.testMode, _ = cmd.Flags().GetBool("test")
	if err := checkFlagHeaders(cmd, message.headers, message.sandbox); err != nil {
		return nil, err
	}

	if send.host, send.port, err = parseServer(send.server); err != nil {
		return nil, err
	}
	if send.testMode {
		return send, nil
	}
	if textFile != "" {
		if message.text, err = cmdutil.ReadFlagFile("text-file", textFile); err != nil {
			return nil, err
		}
	}
	if htmlFile != "" {
		if message.html, err = cmdutil.ReadFlagFile("html-file", htmlFile); err != nil {
			return nil, err
		}
	}
	if message.attachments, err = cmdutil.ReadAttachments(attachPaths); err != nil {
		return nil, err
	}

	// Ask for missing required values when stdin is a terminal
	if message.from, err = cmdutil.ReadOrAsk(cmd, "from", "Sender email address"); err != nil {
		return nil, err
	}
	if err := validation.ValidateEmail("--from", message.from); err != nil {
		return nil, err
	}
	if message.to == nil && message.cc == nil && message.bcc == nil {
		recipient, err := cmdutil.AskIfEmpty(cmd, "", "Recipient email address",
			"at least one of --to, --cc or --bcc must be provided")
		if err != nil {
			return nil, err
		}
		message.to = []string{recipient}
	}
	for _, recipients := range []struct {
		flag   string
		emails []string
	}{{"--to", message.to}, {"--cc", message.cc}, {"--bcc", message.bcc}} {
		for _, email := range recipients.emails {
			if err := validation.ValidateEmail(recipients.flag, email); err != nil {
				return nil, err
			}
		}
	}
	if message.subject, err = cmdutil.ReadOrAsk(cmd, "subject", "Email subject"); err != nil {
		return nil, err
	}
	if message.text == "" && message.html == "" {
		if message.text, err = cmdutil.AskIfEmpty(cmd, "", "Email content (plain text)",
			"one of --text, --html, --text-file or --html-file must be provided"); err != nil {
			return nil, err
		}
	}
	if send.username, err = cmdutil.ReadOrAsk(cmd, "username", "SMTP username"); err != nil {
		return nil, err
	}
	if send.password, err = readPassword(cmd); err != nil {
		return nil, err
	}
	send.message = message
	return send, nil
}

// ownHeaders are the headers the command always builds itself, with how to
// set each instead. The sender and recipient headers make the SMTP
// envelope, so a --header for one would send the message to addresses no
// flag gave; the MIME headers describe the body and attachments. Date and
// Message-ID are not here: --header may set them.
var ownHeaders = []cmdutil.ReservedHeader{
	{Name: "From", Hint: "set it with --from"},
	{Name: "Sender", Hint: "the sender is --from"},
	{Name: "To", Hint: "set it with --to"},
	{Name: "Cc", Hint: "set it with --cc"},
	{Name: "Bcc", Hint: "set it with --bcc"},
	{Name: "Subject", Hint: "set it with --subject"},
	{Name: "Mime-Version", Hint: "the command sets it"},
	{Name: "Content-Type", Hint: "the command sets it from the content and attachments"},
	{Name: "Content-Transfer-Encoding", Hint: "the command sets it from the content and attachments"},
}

// checkFlagHeaders rejects a --header that names, in any case, a header the
// command sets itself, which it would replace or send twice: one of
// ownHeaders, or one it sets from a flag that was given, the AhaSend headers
// of --track-opens, --track-clicks, --tag, --sandbox and --sandbox-result,
// which --sandbox sets with its default.
func checkFlagHeaders(cmd *cobra.Command, headers map[string]string, sandbox bool) error {
	if err := cmdutil.CheckReservedHeaders(headers, ownHeaders); err != nil {
		return err
	}
	sandboxResultFlag := "sandbox"
	if cmd.Flags().Changed("sandbox-result") {
		sandboxResultFlag = "sandbox-result"
	}
	flagHeaders := []struct {
		header, flag string
		set          bool
	}{
		{"AhaSend-Track-Opens", "track-opens", cmd.Flags().Changed("track-opens")},
		{"AhaSend-Track-Clicks", "track-clicks", cmd.Flags().Changed("track-clicks")},
		{"AhaSend-Tags", cmdutil.TypedName(cmd, "tag"), cmd.Flags().Changed("tag")},
		{"AhaSend-Sandbox", "sandbox", sandbox},
		{"AhaSend-Sandbox-Result", sandboxResultFlag, sandbox},
	}
	for _, flagHeader := range flagHeaders {
		if name, ok := cmdutil.FindHeader(headers, flagHeader.header); ok && flagHeader.set {
			return errors.NewValidationError(fmt.Sprintf("--header %s and --%s cannot be used together", name, flagHeader.flag), nil)
		}
	}
	return nil
}

// parseServer splits --server into a host and a port; a server without a
// port uses 587. An empty server or host, and a port outside 1-65535, are
// validation errors.
func parseServer(server string) (string, int, error) {
	if strings.TrimSpace(server) == "" {
		return "", 0, errors.NewValidationError("--server cannot be empty", nil)
	}
	host, portStr, err := net.SplitHostPort(server)
	if err != nil {
		host, portStr = server, "587"
	}
	if host == "" {
		return "", 0, validation.InvalidValueError("--server", server, "no host; use a host and port such as send.ahasend.com:587")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, validation.InvalidValueError("--server", server, "the port must be a number from 1 to 65535")
	}
	return host, port, nil
}

// ensureMessageID returns the Message-ID of m: the one --header gave, or a
// new one, <uuid@the domain of --from>, which it adds to m's headers. AhaSend
// keeps the Message-ID a message has, so messages get finds the message by
// it.
func ensureMessageID(m *smtpMessage) string {
	for name, value := range m.headers {
		if strings.EqualFold(name, "Message-ID") {
			return value
		}
	}
	id := fmt.Sprintf("<%s@%s>", uuid.NewString(), m.from[strings.LastIndex(m.from, "@")+1:])
	if m.headers == nil {
		m.headers = map[string]string{}
	}
	m.headers["Message-ID"] = id
	return id
}

// buildGomailMessage builds the MIME message of m.
func buildGomailMessage(m smtpMessage) *gomail.Message {
	message := gomail.NewMessage()

	// Set basic headers
	message.SetHeader("From", m.from)

	if len(m.to) > 0 {
		message.SetHeader("To", m.to...)
	}
	if len(m.cc) > 0 {
		message.SetHeader("Cc", m.cc...)
	}
	if len(m.bcc) > 0 {
		message.SetHeader("Bcc", m.bcc...)
	}

	message.SetHeader("Subject", m.subject)

	// Set AhaSend special headers
	if m.trackOpens != nil {
		message.SetHeader("AhaSend-Track-Opens", strconv.FormatBool(*m.trackOpens))
	}
	if m.trackClicks != nil {
		message.SetHeader("AhaSend-Track-Clicks", strconv.FormatBool(*m.trackClicks))
	}
	if len(m.tags) > 0 {
		message.SetHeader("AhaSend-Tags", strings.Join(m.tags, ","))
	}
	if m.sandbox {
		message.SetHeader("AhaSend-Sandbox", "true")
		if m.sandboxResult != "" {
			message.SetHeader("AhaSend-Sandbox-Result", m.sandboxResult)
		}
	}

	// Set custom headers. gomail adds a Date header unless one is set under
	// that exact name, so a date given in another case is set as Date.
	for name, value := range m.headers {
		if strings.EqualFold(name, "Date") {
			name = "Date"
		}
		message.SetHeader(name, value)
	}

	// Set body content
	if m.text != "" && m.html != "" {
		// Multipart alternative
		message.SetBody("text/plain", m.text)
		message.AddAlternative("text/html", m.html)
	} else if m.html != "" {
		// HTML only
		message.SetBody("text/html", m.html)
	} else {
		// Text only
		message.SetBody("text/plain", m.text)
	}

	// Add attachments
	for _, attachment := range m.attachments {
		data := attachment.Data
		message.Attach(attachment.Path, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		}))
	}

	return message
}

// testSMTPConnection connects to the server, logs in when --username and
// --password are given, and quits without sending.
// Giving one of --username and --password asks for the other.
func testSMTPConnection(cmd *cobra.Command, handler printer.ResponseHandler, host string, port int) error {
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	passwordStdin, _ := cmd.Flags().GetBool("password-stdin")
	if username != "" || password != "" || passwordStdin {
		var err error
		if username, err = cmdutil.ReadOrAsk(cmd, "username", "SMTP username"); err != nil {
			return err
		}
		if password, err = readPassword(cmd); err != nil {
			return err
		}
	}
	logger.Get().WithField("server", net.JoinHostPort(host, strconv.Itoa(port))).Debug("Testing the SMTP connection")

	result := &printer.SMTPSendResult{TestMode: true}
	config := printer.ResultConfig{SuccessMessage: "SMTP test completed successfully"}

	session, err := connectSMTP(host, port)
	if err != nil {
		return printSMTPTestFailure(handler, result, config, "SMTP connection test failed", err)
	}
	defer session.quit()
	result.Connected = true
	if username != "" {
		if err := session.logIn(username, password); err != nil {
			return printSMTPTestFailure(handler, result, config, "SMTP login failed", err)
		}
		result.Authenticated = true
	}
	result.Success = true
	return handler.HandleSMTPSend(result, config)
}

// readPassword returns the SMTP password: the first line of stdin with
// --password-stdin, or else --password, which the command asks for when it
// is missing and it can ask.
func readPassword(cmd *cobra.Command) (string, error) {
	if passwordStdin, _ := cmd.Flags().GetBool("password-stdin"); passwordStdin {
		return prompt.ReadStdinLine(cmd, "password-stdin", "password")
	}
	return cmdutil.ReadSecretOrAsk(cmd, "password", "SMTP password")
}

// printSMTPTestFailure prints result with the failure err of action, such as
// "SMTP login failed", and returns the error the command exits with: a
// network failure exits 7.
func printSMTPTestFailure(handler printer.ResponseHandler, result *printer.SMTPSendResult, config printer.ResultConfig, action string, err error) error {
	err = sessionFailure(err)
	result.Error = smtpErrorText(err)
	if printErr := handler.HandleSMTPSend(result, config); printErr != nil {
		return printErr
	}
	return errors.NewResultPrintedError((&smtpError{action: action, err: err}).Error(), errors.GetExitCode(err))
}
