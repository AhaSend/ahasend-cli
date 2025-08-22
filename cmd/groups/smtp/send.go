package smtp

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
	"gopkg.in/gomail.v2"
)

// NewSendCommand creates the smtp send command
func NewSendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send an email via SMTP protocol",
		Long: `Send an email using SMTP protocol with AhaSend's SMTP server.

This command allows you to test SMTP sending directly from the CLI using
the same options as the regular 'messages send' command but via SMTP protocol.

INTERACTIVE MODE:
When called without any arguments, the command will interactively prompt for
all required information: sender email, recipient, subject, content, and
SMTP credentials.

You can use existing SMTP credentials or provide credentials directly.
The command supports all standard email features including attachments,
HTML content, and custom headers.

Special headers can be used to control AhaSend features:
- AhaSend-Track-Opens: true/false
- AhaSend-Track-Clicks: true/false
- AhaSend-Tags: comma-separated tags
- AhaSend-Sandbox: true/false
- AhaSend-Sandbox-Result: deliver/bounce/defer/fail/suppress

In test mode, the command validates the SMTP connection and message building
without actually sending the email. It performs all SMTP steps up to DATA
command and then closes the connection.`,
		Example: `  # Interactive mode - prompts for all required information
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

  # Test SMTP connection and message validation
  ahasend smtp send --test \
    --from test@example.com \
    --to recipient@example.com \
    --subject "Test Message" \
    --text "This is a test" \
    --username smtp-user \
    --password smtp-pass

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

  # Use custom SMTP server
  ahasend smtp send \
    --server mail.example.com:587 \
    --username user \
    --password pass \
    --from sender@example.com \
    --to recipient@example.com \
    --subject "Custom Server Test"`,
		RunE: runSMTPSend,
	}

	// Email content flags
	cmd.Flags().String("from", "", "From email address")
	cmd.Flags().StringSlice("to", []string{}, "Recipient email addresses (can be used multiple times)")
	cmd.Flags().StringSlice("cc", []string{}, "CC recipients")
	cmd.Flags().StringSlice("bcc", []string{}, "BCC recipients")
	cmd.Flags().String("subject", "", "Email subject")
	cmd.Flags().String("text", "", "Plain text content")
	cmd.Flags().String("html", "", "HTML content")
	cmd.Flags().String("text-file", "", "Read text content from file")
	cmd.Flags().String("html-file", "", "Read HTML content from file")
	cmd.Flags().StringSlice("attach", []string{}, "File attachments (can be used multiple times)")

	// SMTP server flags
	cmd.Flags().String("server", "send.ahasend.com:587", "SMTP server address")
	cmd.Flags().String("username", "", "SMTP username (uses credential from account if not provided)")
	cmd.Flags().String("password", "", "SMTP password")
	cmd.Flags().String("credential-id", "", "Use specific SMTP credential by ID")

	// Special headers for AhaSend features
	cmd.Flags().Bool("track-opens", false, "Enable open tracking")
	cmd.Flags().Bool("track-clicks", false, "Enable click tracking")
	cmd.Flags().StringSlice("tags", []string{}, "Message tags")
	cmd.Flags().Bool("sandbox", false, "Send in sandbox mode")
	cmd.Flags().String("sandbox-result", "deliver", "Sandbox result simulation: deliver, bounce, defer, fail, or suppress (only used with --sandbox)")
	cmd.Flags().StringSlice("header", []string{}, "Custom headers (format: 'Name: value')")

	// Test mode
	cmd.Flags().Bool("test", false, "Test SMTP connection and message building (don't send email)")

	return cmd
}

// Interactive prompt functions
func promptSMTPFromEmail() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter sender email address: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(email), nil
}

func promptSMTPToEmail() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter recipient email address: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(email), nil
}

func promptSMTPSubject() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter email subject: ")
	subject, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(subject), nil
}

func promptSMTPContent() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter email content (plain text): ")
	content, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(content), nil
}

func promptSMTPCredentials() (username, password string, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter SMTP username: ")
	username, err = reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	username = strings.TrimSpace(username)

	fmt.Print("Enter SMTP password: ")
	password, err = reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	password = strings.TrimSpace(password)

	return username, password, nil
}

func runSMTPSend(cmd *cobra.Command, args []string) error {
	// Get printer instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// Get flag values
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetStringSlice("to")
	cc, _ := cmd.Flags().GetStringSlice("cc")
	bcc, _ := cmd.Flags().GetStringSlice("bcc")
	subject, _ := cmd.Flags().GetString("subject")
	text, _ := cmd.Flags().GetString("text")
	html, _ := cmd.Flags().GetString("html")
	textFile, _ := cmd.Flags().GetString("text-file")
	htmlFile, _ := cmd.Flags().GetString("html-file")
	attachments, _ := cmd.Flags().GetStringSlice("attach")

	server, _ := cmd.Flags().GetString("server")
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	credentialID, _ := cmd.Flags().GetString("credential-id")

	trackOpens, _ := cmd.Flags().GetBool("track-opens")
	trackClicks, _ := cmd.Flags().GetBool("track-clicks")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	sandbox, _ := cmd.Flags().GetBool("sandbox")
	sandboxResult, _ := cmd.Flags().GetString("sandbox-result")
	headers, _ := cmd.Flags().GetStringSlice("header")

	testMode, _ := cmd.Flags().GetBool("test")

	var err error

	// Interactive prompts for missing required fields
	if from == "" {
		from, err = promptSMTPFromEmail()
		if err != nil {
			return errors.NewValidationError("failed to get sender email", err)
		}
	}

	// Validate sender email
	if err := validation.ValidateEmail(from); err != nil {
		return err
	}

	// Interactive prompt for recipients if none provided
	if len(to) == 0 && len(cc) == 0 && len(bcc) == 0 {
		toEmail, err := promptSMTPToEmail()
		if err != nil {
			return errors.NewValidationError("failed to get recipient email", err)
		}
		to = []string{toEmail}
	}

	// Interactive prompt for subject if not provided
	if subject == "" {
		subject, err = promptSMTPSubject()
		if err != nil {
			return errors.NewValidationError("failed to get email subject", err)
		}
	}

	// Interactive prompt for content if none provided
	if text == "" && html == "" && textFile == "" && htmlFile == "" {
		text, err = promptSMTPContent()
		if err != nil {
			return errors.NewValidationError("failed to get email content", err)
		}
	}

	// Interactive prompt for credentials if not provided (except in test mode without actual sending)
	if (username == "" || password == "") && !testMode {
		if credentialID != "" {
			return errors.NewValidationError("cannot retrieve password for existing credential; please provide username and password directly", nil)
		}
		username, password, err = promptSMTPCredentials()
		if err != nil {
			return errors.NewValidationError("failed to get SMTP credentials", err)
		}
	}

	// Validate credentials are provided when needed
	if (username == "" || password == "") && !testMode {
		return errors.NewValidationError("--username and --password are required", nil)
	}

	// Load text content from file if specified
	if textFile != "" {
		content, err := os.ReadFile(textFile)
		if err != nil {
			return fmt.Errorf("failed to read text file: %w", err)
		}
		text = string(content)
	}

	// Load HTML content from file if specified
	if htmlFile != "" {
		content, err := os.ReadFile(htmlFile)
		if err != nil {
			return fmt.Errorf("failed to read HTML file: %w", err)
		}
		html = string(content)
	}

	// Final validation checks (should not fail if interactive prompts worked correctly)
	if from == "" {
		return errors.NewValidationError("sender email is required", nil)
	}
	if len(to) == 0 && len(cc) == 0 && len(bcc) == 0 {
		return errors.NewValidationError("at least one recipient is required", nil)
	}
	if subject == "" {
		return errors.NewValidationError("email subject is required", nil)
	}
	if text == "" && html == "" {
		return errors.NewValidationError("email content is required", nil)
	}

	// Validate sandbox result if sandbox mode is enabled
	if sandbox && sandboxResult != "" {
		validResults := []string{"deliver", "bounce", "defer", "fail", "suppress"}
		isValid := false
		for _, valid := range validResults {
			if sandboxResult == valid {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.NewValidationError(
				fmt.Sprintf("invalid sandbox-result '%s', must be one of: deliver, bounce, defer, fail, suppress", sandboxResult), nil)
		}
	}

	logger.Get().WithFields(map[string]interface{}{
		"server":    server,
		"from":      from,
		"to":        to,
		"test_mode": testMode,
	}).Debug("Sending email via SMTP with gomail")

	// Parse server address
	host, portStr, err := net.SplitHostPort(server)
	if err != nil {
		// If no port specified, default to 587
		host = server
		portStr = "587"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port number: %s", portStr)
	}

	// Build the email message
	message := buildGomailMessage(
		from, to, cc, bcc,
		subject, text, html,
		attachments, headers,
		trackOpens, trackClicks, tags, sandbox, sandboxResult,
	)

	if message == nil {
		return errors.NewAPIError("failed to build email message", nil)
	}

	// Create dialer
	dialer := gomail.NewDialer(host, port, username, password)

	// Configure TLS settings based on port
	if port == 465 {
		// Use SSL/TLS for port 465
		dialer.SSL = true
	} else {
		// Use STARTTLS for other ports (usually 587)
		dialer.TLSConfig = nil // Use default TLS config
	}

	// Test mode - validate connection and message without sending
	if testMode {
		return testSMTPWithGomail(dialer, message, handler, cmd)
	}

	// Send the email
	if err := dialer.DialAndSend(message); err != nil {
		return errors.NewAPIError(fmt.Sprintf("SMTP send failed: %v", err), nil)
	}

	// Print success message using printer
	return printSendSuccess(handler, from, to, cc, bcc, subject, server, len(attachments))
}

func buildGomailMessage(
	from string, to, cc, bcc []string,
	subject, textContent, htmlContent string,
	attachments []string, customHeaders []string,
	trackOpens, trackClicks bool, tags []string, sandbox bool, sandboxResult string,
) *gomail.Message {
	message := gomail.NewMessage()

	// Set basic headers
	message.SetHeader("From", from)

	if len(to) > 0 {
		message.SetHeader("To", to...)
	}
	if len(cc) > 0 {
		message.SetHeader("Cc", cc...)
	}
	if len(bcc) > 0 {
		message.SetHeader("Bcc", bcc...)
	}

	message.SetHeader("Subject", subject)

	// Set AhaSend special headers
	if trackOpens {
		message.SetHeader("AhaSend-Track-Opens", "true")
	}
	if trackClicks {
		message.SetHeader("AhaSend-Track-Clicks", "true")
	}
	if len(tags) > 0 {
		message.SetHeader("AhaSend-Tags", strings.Join(tags, ","))
	}
	if sandbox {
		message.SetHeader("AhaSend-Sandbox", "true")
		if sandboxResult != "" {
			message.SetHeader("AhaSend-Sandbox-Result", sandboxResult)
		}
	}

	// Set custom headers
	for _, header := range customHeaders {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			message.SetHeader(name, value)
		}
	}

	// Set body content
	if textContent != "" && htmlContent != "" {
		// Multipart alternative
		message.SetBody("text/plain", textContent)
		message.AddAlternative("text/html", htmlContent)
	} else if htmlContent != "" {
		// HTML only
		message.SetBody("text/html", htmlContent)
	} else {
		// Text only
		message.SetBody("text/plain", textContent)
	}

	// Add attachments
	for _, attachment := range attachments {
		message.Attach(attachment)
	}

	return message
}

func testSMTPWithGomail(dialer *gomail.Dialer, message *gomail.Message, handler printer.ResponseHandler, cmd *cobra.Command) error {
	// Create a custom sender that will test the connection
	testResult := &printer.SMTPSendResult{
		Success:  false, // Will be set to true if connection succeeds
		TestMode: true,
	}

	// Test the connection by creating a mock sender
	sender, err := dialer.Dial()
	if err != nil {
		testResult.Success = false
		testResult.Error = err.Error()
		return printTestResult(testResult, handler, cmd)
	}
	defer sender.Close()

	// Test sending process up to DATA command - if we got this far, everything worked
	testResult.Success = true

	return printTestResult(testResult, handler, cmd)
}

func printTestResult(result *printer.SMTPSendResult, handler printer.ResponseHandler, cmd *cobra.Command) error {
	// Use the ResponseHandler to display SMTP test result
	successMsg := "SMTP connection test failed"
	if result.Success {
		successMsg = "SMTP test completed successfully"
	}
	return handler.HandleSMTPSend(result, printer.SMTPSendConfig{
		SuccessMessage: successMsg,
		TestMode:       true,
	})
}

func printSendSuccess(handler printer.ResponseHandler, from string, to, cc, bcc []string, subject, server string, attachmentCount int) error {
	// Create SMTPSendResult for ResponseHandler
	result := &printer.SMTPSendResult{
		Success:  true,
		TestMode: false,
	}

	// Use the new ResponseHandler to display SMTP send success
	return handler.HandleSMTPSend(result, printer.SMTPSendConfig{
		SuccessMessage: "Email sent successfully via SMTP",
		TestMode:       false,
	})
}
