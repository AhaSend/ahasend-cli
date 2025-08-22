package messages

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/batch"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/progress"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	ahasend "github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewSendCommand creates the send command
func NewSendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send an email message",
		Long: `Send an email message using the AhaSend API.
You can send plain text, HTML, or AMP emails with templates, per-recipient substitutions,
custom headers, and scheduling options.

The sender email address must be from a verified domain in your AhaSend account.

RECIPIENT OPTIONS:
  --to: Use multiple times for simple recipient list (supports global substitutions only)
  --recipients: Use JSON/CSV file for recipients with per-recipient substitutions
  Note: --to and --recipients are mutually exclusive

RECIPIENTS FILE FORMATS:
  JSON format:
    [
      {
        "email": "user1@example.com",
        "name": "John Doe",
        "substitution_data": {
          "first_name": "John",
          "order_id": "12345"
        }
      }
    ]

  CSV format:
    email,name,first_name,order_id
    user1@example.com,John Doe,John,12345
    user2@example.com,Jane Smith,Jane,12346

CONTENT OPTIONS:
  Direct content: --text, --html, --amp (string values)
  Template files: --text-template, --html-template, --amp-template (file paths)
  Multiple content types can be used together for multipart emails

ATTACHMENT OPTIONS:
  --attach: File paths to attach (can be used multiple times, max 10MB per file)
  Supports all file types with automatic MIME type detection
  Files are automatically Base64 encoded for transmission

IDEMPOTENCY:
  --idempotency-key: Unique key for safe retries (auto-generated if not provided)
  Keys prevent duplicate sends and expire after 24 hours

BATCH OPERATIONS:
  --progress: Show progress bar (TTY only, disabled in debug mode)
  --max-concurrency N: Send up to N messages concurrently (default: 1)
  --max-retries N: Retry failed sends up to N times (default: 3)
  --show-metrics: Display performance statistics after completion`,
		Example: `  # Send simple text email to single recipient
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Hello" --text "Hello World"

  # Send multipart email with both HTML and text
  ahasend messages send --from sender@mydomain.com --to user@example.com --subject "Newsletter" --html "<h1>Welcome</h1>" --text "Welcome"

  # Send to multiple recipients (global substitutions only)
  ahasend messages send --from sender@mydomain.com --to user1@example.com --to user2@example.com --subject "Hi {{name}}" --text-template message.txt --global-substitutions data.json

  # Send with recipients file (supports per-recipient substitutions)
  ahasend messages send --from sender@mydomain.com --recipients recipients.json --subject "Order {{order_id}} Confirmation" --html-template order.html

  # Send with global and per-recipient substitutions (recipients override global)
  ahasend messages send --from sender@mydomain.com --recipients recipients.csv --html-template email.html --global-substitutions defaults.json --subject "{{subject_line}}"

  # Send multipart template email (HTML + text + AMP)
  ahasend messages send --from sender@mydomain.com --to user@example.com --subject "Multi-format" --html-template email.html --text-template email.txt --amp-template email.amp

  # Send in sandbox mode for testing
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Test" --text "Test message" --sandbox

  # Send in sandbox mode simulating a bounce
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Test" --text "Test message" --sandbox --sandbox-result bounce

  # Schedule templated email
  ahasend messages send --from sender@mydomain.com --recipients users.json --html-template welcome.html --schedule "2024-12-01T10:00:00Z"

  # Send with attachments
  ahasend messages send --from sender@mydomain.com --to user@example.com --subject "Invoice" --text "See attached invoice" --attach invoice.pdf --attach logo.png

  # Send with custom idempotency key for safe retries
  ahasend messages send --from sender@mydomain.com --to user@example.com --subject "Important" --text "Message" --idempotency-key "my-unique-key-123"

  # Batch send with progress bar and metrics
  ahasend messages send --from sender@mydomain.com --recipients large-list.csv --subject "Newsletter" --html-template newsletter.html --progress --show-metrics

  # High-performance batch send with concurrency
  ahasend messages send --from sender@mydomain.com --recipients 10000-users.json --subject "Announcement" --text-template message.txt --max-concurrency 5 --progress`,
		RunE:         runMessagesSend,
		SilenceUsage: true,
	}

	// Required email parameters
	cmd.Flags().String("from", "", "Sender email address (required)")
	cmd.Flags().StringSlice("to", []string{}, "Recipient email addresses (can be used multiple times)")
	cmd.Flags().String("subject", "", "Email subject")

	// Content options
	cmd.Flags().String("text", "", "Plain text content")
	cmd.Flags().String("html", "", "HTML content")
	cmd.Flags().String("amp", "", "AMP HTML content")

	// Template options
	cmd.Flags().String("text-template", "", "Plain text template file path")
	cmd.Flags().String("html-template", "", "HTML template file path")
	cmd.Flags().String("amp-template", "", "AMP HTML template file path")

	// Recipient and substitution options
	cmd.Flags().String("recipients", "", "Recipients file (JSON or CSV format) with per-recipient substitutions")
	cmd.Flags().String("global-substitutions", "", "JSON file with global template variables")

	// Advanced options
	cmd.Flags().StringSlice("header", []string{}, "Custom headers in format 'Header-Name: value' (can be used multiple times)")
	cmd.Flags().String("schedule", "", "Schedule delivery time in RFC3339 format (e.g., '2024-12-01T10:00:00Z')")
	cmd.Flags().Bool("sandbox", false, "Send in sandbox mode (for testing)")
	cmd.Flags().String("sandbox-result", "deliver", "Sandbox result simulation: deliver, bounce, defer, fail, or suppress (only used with --sandbox)")
	cmd.Flags().StringSlice("tags", []string{}, "Tags for categorization (can be used multiple times)")
	cmd.Flags().String("idempotency-key", "", "Idempotency key for duplicate prevention")

	// Tracking options
	cmd.Flags().Bool("track-opens", true, "Enable open tracking")
	cmd.Flags().Bool("track-clicks", true, "Enable click tracking")

	// Attachments
	cmd.Flags().StringSlice("attach", []string{}, "Attachment file paths (can be used multiple times, max 10MB per file)")

	// Batch operation enhancements
	cmd.Flags().Bool("progress", false, "Show progress bar for batch operations (disabled in debug mode)")
	cmd.Flags().Int("max-concurrency", 1, "Maximum concurrent sends for batch operations")
	cmd.Flags().Int("max-retries", 3, "Maximum retry attempts for failed sends")
	cmd.Flags().Bool("show-metrics", false, "Show performance metrics after batch operations")

	// Mark mutually exclusive flags
	cmd.MarkFlagsMutuallyExclusive("to", "recipients")

	return cmd
}

// SendFlags holds all the parsed flags for the send command
type SendFlags struct {
	// Basic email parameters
	FromEmail      string
	ToEmails       []string
	RecipientsFile string
	Subject        string

	// Content options
	TextContent string
	HtmlContent string
	AmpContent  string

	// Template files
	TextTemplate string
	HtmlTemplate string
	AmpTemplate  string

	// Substitutions
	GlobalSubstitutionsFile string

	// Advanced options
	CustomHeaders  []string
	ScheduleTime   string
	Sandbox        bool
	SandboxResult  string
	Tags           []string
	IdempotencyKey string
	TrackOpens     bool
	TrackClicks    bool
	Attachments    []string

	// Batch operation options
	ShowProgress   bool
	MaxConcurrency int
	MaxRetries     int
	ShowMetrics    bool
	DebugMode      bool
}

// parseSendFlags extracts all command flags into a structured object
func parseSendFlags(cmd *cobra.Command) *SendFlags {
	return &SendFlags{
		// Basic email parameters
		FromEmail:      getStringFlag(cmd, "from"),
		ToEmails:       getStringSliceFlag(cmd, "to"),
		RecipientsFile: getStringFlag(cmd, "recipients"),
		Subject:        getStringFlag(cmd, "subject"),

		// Content options
		TextContent: getStringFlag(cmd, "text"),
		HtmlContent: getStringFlag(cmd, "html"),
		AmpContent:  getStringFlag(cmd, "amp"),

		// Template files
		TextTemplate: getStringFlag(cmd, "text-template"),
		HtmlTemplate: getStringFlag(cmd, "html-template"),
		AmpTemplate:  getStringFlag(cmd, "amp-template"),

		// Substitutions
		GlobalSubstitutionsFile: getStringFlag(cmd, "global-substitutions"),

		// Advanced options
		CustomHeaders:  getStringSliceFlag(cmd, "header"),
		ScheduleTime:   getStringFlag(cmd, "schedule"),
		Sandbox:        getBoolFlag(cmd, "sandbox"),
		SandboxResult:  getStringFlag(cmd, "sandbox-result"),
		Tags:           getStringSliceFlag(cmd, "tags"),
		IdempotencyKey: getStringFlag(cmd, "idempotency-key"),
		TrackOpens:     getBoolFlag(cmd, "track-opens"),
		TrackClicks:    getBoolFlag(cmd, "track-clicks"),
		Attachments:    getStringSliceFlag(cmd, "attach"),

		// Batch operation options
		ShowProgress:   getBoolFlag(cmd, "progress"),
		MaxConcurrency: getIntFlag(cmd, "max-concurrency"),
		MaxRetries:     getIntFlag(cmd, "max-retries"),
		ShowMetrics:    getBoolFlag(cmd, "show-metrics"),
		DebugMode:      getBoolFlag(cmd, "debug"),
	}
}

// Helper functions for flag extraction
func getStringFlag(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}

func getStringSliceFlag(cmd *cobra.Command, name string) []string {
	value, _ := cmd.Flags().GetStringSlice(name)
	return value
}

func getBoolFlag(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}

func getIntFlag(cmd *cobra.Command, name string) int {
	value, _ := cmd.Flags().GetInt(name)
	return value
}

func runMessagesSend(cmd *cobra.Command, args []string) error {
	// Get response handler instance and authenticated client
	handler := printer.GetResponseHandlerFromCommand(cmd)
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Parse all flags into structured object
	flags := parseSendFlags(cmd)

	// Process the batch send operation
	return processBatchSend(handler, client, flags)
}

// processBatchSend handles the main batch sending workflow
func processBatchSend(handler printer.ResponseHandler, cl client.AhaSendClient, flags *SendFlags) error {
	// Process and create send jobs
	sendJobs, err := createSendJobsFromFlags(flags)
	if err != nil {
		return handler.HandleError(err)
	}

	// Set up progress reporting
	progressReporter := setupProgressReporting(sendJobs, flags)

	// Process batch
	batchResult, err := executeBatchSend(cl, sendJobs, flags, progressReporter)
	if err != nil {
		return handler.HandleError(err)
	}

	// Format and return response using the new handler
	return formatBatchResponse(handler, batchResult, flags)
}

// createSendJobsFromFlags creates send jobs using the parsed flags
func createSendJobsFromFlags(flags *SendFlags) ([]*batch.SendJob, error) {
	return createSendJobs(
		flags.FromEmail, flags.ToEmails, flags.RecipientsFile, flags.Subject,
		flags.TextContent, flags.HtmlContent, flags.AmpContent,
		flags.TextTemplate, flags.HtmlTemplate, flags.AmpTemplate,
		flags.GlobalSubstitutionsFile,
		flags.CustomHeaders, flags.ScheduleTime, flags.Sandbox, flags.SandboxResult, flags.Tags,
		flags.TrackOpens, flags.TrackClicks, flags.Attachments, flags.IdempotencyKey,
	)
}

// setupProgressReporting configures progress reporting based on job requirements
func setupProgressReporting(sendJobs []*batch.SendJob, flags *SendFlags) *progress.Reporter {
	// Calculate total recipients (not jobs)
	totalRecipients := 0
	for _, job := range sendJobs {
		totalRecipients += job.RecipientCount
	}

	// Set up progress reporting if needed
	if totalRecipients > 1 || flags.ShowProgress {
		return progress.NewReporter(totalRecipients, flags.ShowProgress, flags.DebugMode)
	}
	return nil
}

// executeBatchSend performs the actual batch send operation
func executeBatchSend(cl client.AhaSendClient, sendJobs []*batch.SendJob, flags *SendFlags, progressReporter *progress.Reporter) (*batch.BatchResult, error) {
	batchProcessor := batch.NewBatchProcessor(cl, flags.MaxConcurrency, flags.MaxRetries, progressReporter)
	return batchProcessor.ProcessJobs(context.Background(), sendJobs)
}

// formatBatchResponse formats the batch result using the ResponseHandler
func formatBatchResponse(handler printer.ResponseHandler, batchResult *batch.BatchResult, flags *SendFlags) error {
	// Single message success - use HandleCreateMessage
	if len(batchResult.SuccessfulResponses) == 1 && batchResult.FailedJobs == 0 {
		response := batchResult.SuccessfulResponses[0]
		return handler.HandleCreateMessage(response, printer.CreateConfig{
			SuccessMessage: "Message sent successfully",
			ItemName:       "message",
		})
	}

	// For batch operations or mixed results, create a summary success message
	successCount := len(batchResult.SuccessfulResponses)
	failedCount := batchResult.FailedJobs
	totalCount := successCount + failedCount

	if failedCount == 0 {
		// All successful
		return handler.HandleSimpleSuccess(fmt.Sprintf("✅ Successfully sent all %d messages", successCount))
	} else if successCount == 0 {
		// All failed
		return handler.HandleError(fmt.Errorf("failed to send all %d messages", failedCount))
	} else {
		// Partial success - still return as error with details
		return handler.HandleError(fmt.Errorf("partial success: %d succeeded, %d failed out of %d total messages", successCount, failedCount, totalCount))
	}
}

// createSendJobs converts the send request into batch jobs
func createSendJobs(
	fromEmail string, toEmails []string, recipientsFile, subject string,
	textContent, htmlContent, ampContent string,
	textTemplate, htmlTemplate, ampTemplate string,
	globalSubstitutionsFile string,
	customHeaders []string, scheduleTime string, sandbox bool, sandboxResult string, tags []string,
	trackOpens, trackClicks bool, attachmentPaths []string, idempotencyKey string,
) ([]*batch.SendJob, error) {
	// Process the send request to get the base request
	request, finalIdempotencyKey, err := processSendRequest(
		fromEmail, toEmails, recipientsFile, subject,
		textContent, htmlContent, ampContent,
		textTemplate, htmlTemplate, ampTemplate,
		globalSubstitutionsFile,
		customHeaders, scheduleTime, sandbox, sandboxResult, tags,
		trackOpens, trackClicks, attachmentPaths, idempotencyKey,
	)
	if err != nil {
		return nil, err
	}

	// Convert to batch jobs - group recipients into batches of up to 100
	const MAX_BATCH_SIZE = 100
	var jobs []*batch.SendJob

	for i := 0; i < len(request.Recipients); i += MAX_BATCH_SIZE {
		end := i + MAX_BATCH_SIZE
		if end > len(request.Recipients) {
			end = len(request.Recipients)
		}

		// Create batch request with up to 100 recipients
		batchRequest := *request
		batchRequest.Recipients = request.Recipients[i:end]

		// Generate unique idempotency key for this batch
		batchIdempotencyKey := fmt.Sprintf("%s-batch-%d", finalIdempotencyKey, i/MAX_BATCH_SIZE)

		job := &batch.SendJob{
			Request:        &batchRequest,
			IdempotencyKey: batchIdempotencyKey,
			BatchIndex:     i / MAX_BATCH_SIZE,
			Recipients:     batchRequest.Recipients,
			RecipientCount: len(batchRequest.Recipients),
		}
		jobs = append(jobs, job)
	}

	logger.Get().WithField("total_jobs", len(jobs)).Debug("Created batch jobs")
	return jobs, nil
}

func promptFromEmail() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter sender email address: ")

	email, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(email), nil
}

func promptToEmail() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter recipient email address: ")

	email, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(email), nil
}

func promptSubject() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter email subject: ")

	subject, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(subject), nil
}

func promptForContent() (content, contentType string, err error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter email content (plain text): ")

	text, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}

	return strings.TrimSpace(text), "text", nil
}

// RecipientData represents a single recipient with their email, name and substitution data
type RecipientData struct {
	Email         string                 `json:"email"`
	Name          string                 `json:"name,omitempty"`
	Substitutions map[string]interface{} `json:"substitutions,omitempty"`
}

// processSendRequest handles all the validation and processing logic for the send request
func processSendRequest(
	fromEmail string, toEmails []string, recipientsFile, subject string,
	textContent, htmlContent, ampContent string,
	textTemplate, htmlTemplate, ampTemplate string,
	globalSubstitutionsFile string,
	customHeaders []string, scheduleTime string, sandbox bool, sandboxResult string, tags []string,
	trackOpens, trackClicks bool, attachmentPaths []string, idempotencyKey string,
) (*requests.CreateMessageRequest, string, error) {

	// Generate or validate idempotency key
	finalIdempotencyKey := idempotencyKey
	var err error
	if finalIdempotencyKey == "" {
		finalIdempotencyKey, err = generateIdempotencyKey()
		if err != nil {
			return nil, "", errors.NewAPIError("failed to generate idempotency key", err)
		}
	}

	// Interactive prompts for missing required fields
	if fromEmail == "" {
		fromEmail, err = promptFromEmail()
		if err != nil {
			return nil, "", errors.NewValidationError("failed to get sender email", err)
		}
	}

	// Validate sender email
	if err := validation.ValidateEmail(fromEmail); err != nil {
		return nil, "", err
	}

	// Validate subject
	if subject == "" {
		subject, err = promptSubject()
		if err != nil {
			return nil, "", errors.NewValidationError("failed to get email subject", err)
		}
	}

	// Process recipients (either --to or --recipients)
	var recipients []common.Recipient
	if recipientsFile != "" {
		recipients, err = loadRecipientsFromFile(recipientsFile)
		if err != nil {
			return nil, "", err
		}
	} else {
		// Use --to flags
		if len(toEmails) == 0 {
			toEmail, err := promptToEmail()
			if err != nil {
				return nil, "", errors.NewValidationError("failed to get recipient email", err)
			}
			toEmails = []string{toEmail}
		}
		recipients, err = createRecipientsFromEmails(toEmails)
		if err != nil {
			return nil, "", err
		}
	}

	// Process content (direct strings or templates)
	contentData, err := processEmailContent(
		textContent, htmlContent, ampContent,
		textTemplate, htmlTemplate, ampTemplate,
	)
	if err != nil {
		return nil, "", err
	}

	// Ensure at least one content type is provided
	if contentData.TextContent == "" && contentData.HtmlContent == "" && contentData.AmpContent == "" {
		// Prompt for content
		content, _, err := promptForContent()
		if err != nil {
			return nil, "", errors.NewValidationError("failed to get email content", err)
		}
		contentData.TextContent = content
	}

	// Process global substitutions
	var globalSubstitutions map[string]interface{}
	if globalSubstitutionsFile != "" {
		globalSubstitutions, err = loadGlobalSubstitutions(globalSubstitutionsFile)
		if err != nil {
			return nil, "", err
		}
	}

	// Process attachments
	var attachments []common.Attachment
	if len(attachmentPaths) > 0 {
		attachments, err = processAttachments(attachmentPaths)
		if err != nil {
			return nil, "", err
		}
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
			return nil, "", errors.NewValidationError(
				fmt.Sprintf("invalid sandbox-result '%s', must be one of: deliver, bounce, defer, fail, suppress", sandboxResult), nil)
		}
	}

	// Build the SDK request
	request, err := buildAdvancedMessageRequest(
		fromEmail, recipients, subject, contentData, globalSubstitutions,
		customHeaders, scheduleTime, sandbox, sandboxResult, tags,
		trackOpens, trackClicks, attachments,
	)
	if err != nil {
		return nil, "", err
	}

	return request, finalIdempotencyKey, nil
}

// ContentData holds the processed email content
type ContentData struct {
	TextContent string
	HtmlContent string
	AmpContent  string
}

// processEmailContent handles both direct content and template files
func processEmailContent(
	textContent, htmlContent, ampContent string,
	textTemplate, htmlTemplate, ampTemplate string,
) (*ContentData, error) {
	content := &ContentData{
		TextContent: textContent,
		HtmlContent: htmlContent,
		AmpContent:  ampContent,
	}

	// Load template files if provided
	if textTemplate != "" {
		textFromFile, err := loadTemplateFile(textTemplate)
		if err != nil {
			return nil, errors.NewFileError("failed to load text template", err)
		}
		content.TextContent = textFromFile
	}

	if htmlTemplate != "" {
		htmlFromFile, err := loadTemplateFile(htmlTemplate)
		if err != nil {
			return nil, errors.NewFileError("failed to load HTML template", err)
		}
		content.HtmlContent = htmlFromFile
	}

	if ampTemplate != "" {
		ampFromFile, err := loadTemplateFile(ampTemplate)
		if err != nil {
			return nil, errors.NewFileError("failed to load AMP template", err)
		}
		content.AmpContent = ampFromFile
	}

	return content, nil
}

// loadTemplateFile loads content from a template file
func loadTemplateFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", errors.NewFileError(fmt.Sprintf("cannot open template file %s", filePath), err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", errors.NewFileError(fmt.Sprintf("cannot read template file %s", filePath), err)
	}

	return string(content), nil
}

// loadRecipientsFromFile loads recipients from JSON or CSV file
func loadRecipientsFromFile(filePath string) ([]common.Recipient, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.NewFileError(fmt.Sprintf("cannot open recipients file %s", filePath), err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		return loadRecipientsFromJSON(file)
	case ".csv":
		return loadRecipientsFromCSV(file)
	default:
		return nil, errors.NewValidationError(fmt.Sprintf("unsupported recipients file format %s (supported: .json, .csv)", ext), nil)
	}
}

// loadRecipientsFromJSON parses recipients from JSON file
func loadRecipientsFromJSON(file *os.File) ([]common.Recipient, error) {
	var recipientData []RecipientData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&recipientData); err != nil {
		return nil, errors.NewFileError("failed to parse JSON recipients file", err)
	}

	var recipients []common.Recipient
	for i, data := range recipientData {
		if err := validation.ValidateEmail(data.Email); err != nil {
			return nil, errors.NewValidationError(fmt.Sprintf("invalid email at index %d: %v", i, err), nil)
		}

		recipient := common.Recipient{
			Email: data.Email,
		}
		if data.Name != "" {
			recipient.Name = ahasend.String(data.Name)
		}
		if len(data.Substitutions) > 0 {
			recipient.Substitutions = data.Substitutions
		}
		recipients = append(recipients, recipient)
	}

	return recipients, nil
}

// loadRecipientsFromCSV parses recipients from CSV file
func loadRecipientsFromCSV(file *os.File) ([]common.Recipient, error) {
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, errors.NewFileError("failed to parse CSV recipients file", err)
	}

	if len(records) < 2 { // Need header + at least one data row
		return nil, errors.NewValidationError("CSV file must have at least a header row and one data row", nil)
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
			return nil, errors.NewValidationError(fmt.Sprintf("row %d has insufficient columns", i+2), nil)
		}

		email := strings.TrimSpace(record[emailIndex])
		if err := validation.ValidateEmail(email); err != nil {
			return nil, errors.NewValidationError(fmt.Sprintf("invalid email at row %d: %v", i+2, err), nil)
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
		if err := validation.ValidateEmail(email); err != nil {
			return nil, errors.NewValidationError(fmt.Sprintf("invalid recipient email %s: %v", email, err), nil)
		}
		recipients = append(recipients, common.Recipient{
			Email: email,
		})
	}
	return recipients, nil
}

// loadGlobalSubstitutions loads global substitution variables from JSON file
func loadGlobalSubstitutions(filePath string) (map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.NewFileError(fmt.Sprintf("cannot open global substitutions file %s", filePath), err)
	}
	defer file.Close()

	var substitutions map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&substitutions); err != nil {
		return nil, errors.NewFileError("failed to parse global substitutions JSON", err)
	}

	return substitutions, nil
}

// buildAdvancedMessageRequest builds the SDK request with all the new features
func buildAdvancedMessageRequest(
	fromEmail string, recipients []common.Recipient, subject string,
	content *ContentData, globalSubstitutions map[string]interface{},
	customHeaders []string, scheduleTime string, sandbox bool, sandboxResult string, tags []string,
	trackOpens, trackClicks bool, attachments []common.Attachment,
) (*requests.CreateMessageRequest, error) {
	// Build sender address
	sender := common.SenderAddress{
		Email: fromEmail,
	}

	// Create the base request with required fields
	request := &requests.CreateMessageRequest{
		From:       sender,
		Recipients: recipients,
		Subject:    subject,
	}

	// Set content types
	if content.TextContent != "" {
		request.TextContent = ahasend.String(content.TextContent)
	}
	if content.HtmlContent != "" {
		request.HtmlContent = ahasend.String(content.HtmlContent)
	}
	if content.AmpContent != "" {
		request.AmpContent = ahasend.String(content.AmpContent)
	}

	// Set global substitutions
	if len(globalSubstitutions) > 0 {
		request.Substitutions = globalSubstitutions
	}

	// Set sandbox mode
	if sandbox {
		request.Sandbox = ahasend.Bool(true)
		// Use the specified sandbox result, defaulting to "deliver" if not specified
		if sandboxResult != "" {
			request.SandboxResult = ahasend.String(sandboxResult)
		} else {
			request.SandboxResult = ahasend.String("deliver")
		}
	}

	// Set tags
	if len(tags) > 0 {
		request.Tags = tags
	}

	// Set tracking settings
	if trackOpens || trackClicks {
		request.Tracking = &common.Tracking{
			Open:  ahasend.Bool(trackOpens),
			Click: ahasend.Bool(trackClicks),
		}
	}

	// Parse and set custom headers
	if len(customHeaders) > 0 {
		headers := make(map[string]string)
		for _, header := range customHeaders {
			parts := strings.SplitN(header, ":", 2)
			if len(parts) == 2 {
				headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		if len(headers) > 0 {
			request.Headers = headers
		}
	}

	// Parse and set schedule time
	if scheduleTime != "" {
		parsedTime, err := time.Parse(time.RFC3339, scheduleTime)
		if err != nil {
			return nil, errors.NewValidationError("invalid schedule time format (use RFC3339)", err)
		}
		request.Schedule = &common.MessageSchedule{
			FirstAttempt: &parsedTime,
		}
	}

	// Set attachments
	if len(attachments) > 0 {
		request.Attachments = attachments
	}

	return request, nil
}

// generateIdempotencyKey generates a unique idempotency key
func generateIdempotencyKey() (string, error) {
	// Generate 16 random bytes and encode as hex
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.NewAPIError("failed to generate random bytes", err)
	}

	// Add timestamp prefix for better debugging
	timestamp := time.Now().Unix()
	return fmt.Sprintf("cli-%d-%s", timestamp, hex.EncodeToString(bytes)), nil
}

// processAttachments processes attachment file paths into SDK Attachment objects
func processAttachments(filePaths []string) ([]common.Attachment, error) {
	const maxFileSize = 10 * 1024 * 1024 // 10MB in bytes

	var attachments []common.Attachment

	for _, filePath := range filePaths {
		// Check if file exists and get info
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return nil, errors.NewFileError(fmt.Sprintf("cannot access file %s", filePath), err)
		}

		// Check file size limit
		if fileInfo.Size() > maxFileSize {
			return nil, errors.NewValidationError(fmt.Sprintf("file %s is too large (%.2f MB > 10 MB)",
				filePath, float64(fileInfo.Size())/(1024*1024)), nil)
		}

		// Read file content
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return nil, errors.NewFileError(fmt.Sprintf("cannot read file %s", filePath), err)
		}

		// Detect MIME type
		contentType := detectMIMEType(filePath, fileContent)

		// Get file name without path
		fileName := filepath.Base(filePath)

		// Create attachment with Base64 encoding
		attachment := common.Attachment{
			FileName:    fileName,
			ContentType: contentType,
			Data:        base64.StdEncoding.EncodeToString(fileContent),
			Base64:      true,
		}

		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

// detectMIMEType detects the MIME type of a file
func detectMIMEType(filePath string, content []byte) string {
	// First try to detect by file extension
	if mimeType := mime.TypeByExtension(filepath.Ext(filePath)); mimeType != "" {
		return mimeType
	}

	// Fall back to content-based detection for common types
	if len(content) == 0 {
		return "application/octet-stream"
	}

	// Check for common file signatures
	switch {
	case len(content) >= 4 && string(content[:4]) == "\x89PNG":
		return "image/png"
	case len(content) >= 3 && string(content[:3]) == "\xFF\xD8\xFF":
		return "image/jpeg"
	case len(content) >= 4 && string(content[:4]) == "GIF8":
		return "image/gif"
	case len(content) >= 4 && string(content[:4]) == "%PDF":
		return "application/pdf"
	case len(content) >= 2 && string(content[:2]) == "PK":
		// Could be ZIP, DOCX, XLSX, etc.
		if strings.HasSuffix(strings.ToLower(filePath), ".docx") {
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}
		if strings.HasSuffix(strings.ToLower(filePath), ".xlsx") {
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
		return "application/zip"
	default:
		// Check if content is text
		if isTextContent(content) {
			return "text/plain"
		}
		return "application/octet-stream"
	}
}

// isTextContent checks if content appears to be text
func isTextContent(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	// Sample first 512 bytes to check for text
	sample := content
	if len(content) > 512 {
		sample = content[:512]
	}

	// Count printable characters
	printable := 0
	for _, b := range sample {
		if (b >= 0x20 && b <= 0x7E) || b == '\t' || b == '\n' || b == '\r' {
			printable++
		}
	}

	// If more than 95% of characters are printable, consider it text
	return float64(printable)/float64(len(sample)) > 0.95
}
