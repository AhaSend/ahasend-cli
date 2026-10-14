package messages

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/batch"
	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/progress"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/timeparse"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	ahasend "github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewSendCommand creates the send command
func NewSendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send an email message",
		Long: `Send an email message using the AhaSend API. You can send plain text, HTML,
or AMP emails with templates, per-recipient substitutions, custom headers,
and scheduling options.

The sender email address must be from a verified domain in your AhaSend
account. --from, --subject, a recipient and content are required; when one
is missing and stdin is a terminal, the command asks for it, and
--non-interactive never asks.

SENDER OPTIONS:
  --from-name: The sender's display name (needs --from)
  --reply-to: The address replies go to; --header cannot also set Reply-To

RECIPIENT OPTIONS:
  --to: Use multiple times for a simple recipient list (supports global
    substitutions only)
  --recipients: Use a JSON/CSV file for recipients with per-recipient
    substitutions
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

  The email and name headers may be in any case, and every value is trimmed.
  Each other column is a substitution; an empty cell sets none, so the
  global substitution of the same name, if any, applies to that recipient.

CONTENT OPTIONS:
  Direct content: --text, --html, --amp (string values)
  Content files: --text-file, --html-file, --amp-file (file paths)
  Text or HTML content is required; AMP content is an extra part that goes
  with one of them. Several content types together make a multipart email.

SCHEDULING:
  --schedule: An RFC3339 time or a relative one such as "2h" from now. The
    API takes a future time within 7 days; a time in the past or later than
    that fails before anything is sent.

TEMPLATE MODE:
  --template-id: Send a stored transactional template (see "ahasend
    templates"). The template supplies the content, so the content options
    above cannot be used. --from and --subject are optional and default to
    the template's sender and subject.
  Global substitutions are copied into every recipient's substitutions; a
  recipient's own value for the same key wins.

ATTACHMENT OPTIONS:
  --attach: Regular file to attach (can be used multiple times, at most
    10,485,760 bytes (10 MiB) per file)
  Supports all file types with automatic MIME type detection
  Files are automatically Base64 encoded for transmission

IDEMPOTENCY:
  --idempotency-key: Idempotency key for safe retries (generated when omitted)
  Keys prevent duplicate sends and expire after 24 hours

BATCH OPERATIONS:
  --progress: Show progress bar (TTY only, disabled in debug mode)
  --max-concurrency N: Send up to N messages concurrently, at most 10
    (default: 1)
  --max-retries N: Send a failed request up to N more times (default: 3).
    Only 429, 5xx, network failures and "request in progress" answers are
    retried, after the Retry-After the API gives or one second more per
    attempt.
  --show-metrics: Display performance statistics on stderr after completion

FAILED RECIPIENTS:
  Recipients that were not sent are saved to ~/.ahasend/failed-*.json, in
  the format --recipients reads, to send them again. A request that may have
  been sent when the retries ran out is of unknown outcome: the API was
  still processing it, a gateway answered 502 or 504, no answer came after
  it was written, or the API accepted it with an answer the CLI cannot
  read. Its recipients are saved apart, to
  ~/.ahasend/unknown-*.json, and stderr names the idempotency key. Check
  them with "ahasend messages list", or run the same command again with the
  --idempotency-key stderr names within 24 hours: the API answers a request
  it already took with its stored result instead of sending it again. A
  relative --schedule stands for a time from now, so stderr also gives the
  time it stood for, to pass as --schedule in its place.

INTERRUPTION:
  Ctrl+C (or SIGTERM) stops the send: no new request or retry starts, the
  requests in flight finish, the results are printed, the recipients not
  sent are saved as above, and the command exits with 130. A signal once
  every request has started, when none of them is retried, changes nothing.
  Press Ctrl+C again to stop at once.`,
		Example: `  # Send simple text email to single recipient
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Hello" --text "Hello World"

  # Send multipart email with both HTML and text
  ahasend messages send --from sender@mydomain.com --to user@example.com --subject "Welcome to AhaSend" --html "<h1>Welcome</h1>" --text "Welcome"

  # Send to multiple recipients (global substitutions only)
  ahasend messages send --from sender@mydomain.com --to user1@example.com --to user2@example.com --subject "Hi {{name}}" --text-file message.txt --global-substitutions data.json

  # Send with recipients file (supports per-recipient substitutions)
  ahasend messages send --from sender@mydomain.com --recipients recipients.json --subject "Order {{order_id}} Confirmation" --html-file order.html

  # Send with global and per-recipient substitutions (recipients override global)
  ahasend messages send --from sender@mydomain.com --recipients recipients.csv --html-file email.html --global-substitutions defaults.json --subject "{{subject_line}}"

  # Send multipart email from files (HTML + text + AMP)
  ahasend messages send --from sender@mydomain.com --to user@example.com --subject "Multi-format" --html-file email.html --text-file email.txt --amp-file email.amp

  # Send in sandbox mode for testing
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Test" --text "Test message" --sandbox

  # Send in sandbox mode simulating a bounce
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Test" --text "Test message" --sandbox --sandbox-result bounce

  # Schedule an email from a file for two hours from now
  ahasend messages send --from sender@mydomain.com --recipients users.json --subject "Welcome" --html-file welcome.html --schedule 2h

  # Send a stored template to a recipients file
  ahasend messages send --template-id 123e4567-e89b-12d3-a456-426614174000 --recipients users.csv --global-substitutions defaults.json

  # Send with attachments
  ahasend messages send --from sender@mydomain.com --to user@example.com --subject "Invoice" --text "See attached invoice" --attach invoice.pdf --attach logo.png

  # Send with custom idempotency key for safe retries
  ahasend messages send --from sender@mydomain.com --to user@example.com --subject "Important" --text "Message" --idempotency-key "my-unique-key-123"

  # Batch send with progress bar and metrics
  ahasend messages send --from sender@mydomain.com --recipients large-list.csv --subject "Welcome to AhaSend" --html-file welcome.html --progress --show-metrics

  # High-performance batch send with concurrency
  ahasend messages send --from sender@mydomain.com --recipients 10000-users.json --subject "Announcement" --text-file message.txt --max-concurrency 5 --progress`,
		Args:         cmdutil.NoArgs,
		RunE:         runMessagesSend,
		SilenceUsage: true,
	}

	// Required email parameters
	cmd.Flags().String("from", "", cmdutil.FromUsage+" (required unless --template-id is set)")
	cmdutil.AddSenderNameFlag(cmd)
	cmd.Flags().String("reply-to", "", "Reply-to email address")
	cmdutil.AddRepeatableFlag(cmd, "to", cmdutil.ToUsage)
	cmd.Flags().String("subject", "", cmdutil.SubjectUsage+" (required unless --template-id is set)")

	// Content options
	cmd.Flags().String("text", "", cmdutil.TextUsage)
	cmd.Flags().String("html", "", cmdutil.HTMLUsage)
	cmd.Flags().String("amp", "", "AMP HTML content")

	// Content files
	cmd.Flags().String("text-file", "", cmdutil.TextFileUsage)
	cmd.Flags().String("html-file", "", cmdutil.HTMLFileUsage)
	cmd.Flags().String("amp-file", "", "File with the AMP HTML content")
	for _, name := range []string{"text-file", "html-file", "amp-file"} {
		cmdutil.MarkFileFlag(cmd, name)
	}
	cmdutil.AddFlagAlias(cmd, "text-template", "text-file")
	cmdutil.AddFlagAlias(cmd, "html-template", "html-file")
	cmdutil.AddFlagAlias(cmd, "amp-template", "amp-file")

	// Stored template
	cmd.Flags().String("template-id", "", "Send this transactional template; it supplies the content")

	// Recipient and substitution options
	cmd.Flags().String("recipients", "", "Recipients file (JSON or CSV format) with per-recipient substitutions")
	cmd.Flags().String("global-substitutions", "", "JSON file with global template variables")
	cmdutil.MarkFileFlag(cmd, "recipients")
	cmdutil.MarkFileFlag(cmd, "global-substitutions")

	// Advanced options
	cmdutil.AddRepeatableFlag(cmd, "header", cmdutil.HeaderUsage+
		"; not From, Sender, To, Cc, Bcc, Subject, Message-ID or a header AhaSend sets, such as AhaSend-Sandbox (can be used multiple times)")
	cmd.Flags().String("schedule", "", "Schedule delivery for a time in RFC3339 format or relative like '30m', '2h' (the API takes a future time within 7 days)")
	cmd.Flags().Bool("sandbox", false, cmdutil.SandboxUsage)
	cmdutil.AddOneOfFlag(cmd, "sandbox-result", "deliver", cmdutil.SandboxResultUsage, cmdutil.SandboxResults)
	cmdutil.AddRepeatableFlag(cmd, "tag", cmdutil.TagUsage)
	cmdutil.AddFlagAlias(cmd, "tags", "tag")
	cmdutil.AddIdempotencyKeyFlag(cmd)

	// Tracking options
	cmd.Flags().Bool("track-opens", false, cmdutil.TrackOpensUsage)
	cmd.Flags().Bool("track-clicks", false, cmdutil.TrackClicksUsage)

	// Attachments
	cmdutil.AddRepeatableFlag(cmd, "attach", cmdutil.AttachUsage+", at most "+cmdutil.AttachSizeLimit+" (can be used multiple times)")
	cmdutil.MarkFileFlag(cmd, "attach")

	// Batch operation enhancements
	cmd.Flags().Bool("progress", false, "Show progress bar for batch operations (disabled in debug mode)")
	cmd.Flags().Int("max-concurrency", 1, fmt.Sprintf("Maximum concurrent sends for batch operations (1-%d)", maxSendConcurrency))
	cmd.Flags().Int("max-retries", 3, "Times to send a failed request again (0 or more)")
	cmd.Flags().Bool("show-metrics", false, "Show performance metrics on stderr after the send")
	prompt.AddNonInteractiveFlag(cmd)

	return cmd
}

// sendFlags holds all the parsed flags for the send command
type sendFlags struct {
	// TemplateID switches the send to a stored template when set
	TemplateID *uuid.UUID

	// Basic email parameters
	FromEmail      string
	FromName       *string
	ReplyTo        *string
	ToEmails       []string
	RecipientsFile string
	Subject        string

	// Content options
	TextContent string
	HtmlContent string
	AmpContent  string

	// Content files
	TextFile string
	HtmlFile string
	AmpFile  string

	// Substitutions
	GlobalSubstitutionsFile string

	// Advanced options
	CustomHeaders  map[string]string
	ScheduleTime   string
	Sandbox        bool
	SandboxResult  string
	Tags           []string
	IdempotencyKey string
	Tracking       *common.Tracking
	Attachments    []string

	// Batch operation options
	ShowProgress   bool
	MaxConcurrency int
	MaxRetries     int
	ShowMetrics    bool
	DebugMode      bool
}

// apiReservedHeaders are the headers --header refuses. The API writes a
// custom header over the From, To and Subject it builds from the request,
// and writes a Bcc into the message every recipient gets; it drops the
// others, the AhaSend headers it sets from the request's options and the
// ones AhaSend routes mail by, so a send with one would go out without what
// it asks for, such as a sandbox send that delivers.
var apiReservedHeaders = []cmdutil.ReservedHeader{
	{Name: "From", Hint: "set it with --from"},
	{Name: "Sender", Hint: "the sender is --from"},
	{Name: "To", Hint: "set it with --to or --recipients"},
	{Name: "Cc", Hint: "the API sends only to --to and --recipients; give the address there"},
	{Name: "Bcc", Hint: "the API sends only to --to and --recipients; give the address there"},
	{Name: "Subject", Hint: "set it with --subject"},
	{Name: "AhaSend-Sandbox", Hint: "the API drops it; set it with --sandbox"},
	{Name: "AhaSend-Sandbox-Result", Hint: "the API drops it; set it with --sandbox-result"},
	{Name: "AhaSend-Tags", Hint: "the API drops it; set it with --tag"},
	{Name: "AhaSend-Track-Opens", Hint: "the API drops it; set it with --track-opens"},
	{Name: "AhaSend-Track-Clicks", Hint: "the API drops it; set it with --track-clicks"},
	{Name: "AhaSend-Schedule", Hint: "the API drops it; set it with --schedule"},
	{Name: "X-Schedule", Hint: "the API drops it; set it with --schedule"},
	{Name: "AhaSend-Data-Retention", Hint: "the API drops it"},
	{Name: "AhaSend-Metadata-Retention", Hint: "the API drops it"},
	{Name: "Message-ID", Hint: "the API drops it and sets its own"},
	{Name: "X-Campaign", Hint: "the API drops it; AhaSend routes mail by it"},
	{Name: "X-Customer-ID", Hint: "the API drops it; AhaSend routes mail by it"},
	{Name: "X-Schedule-ID", Hint: "the API drops it; AhaSend routes mail by it"},
	{Name: "X-Subscriber-ID", Hint: "the API drops it; AhaSend routes mail by it"},
}

// parseSendFlags reads the command's flags. An empty value of a repeatable
// flag, a malformed header and an unknown sandbox result are validation
// errors.
func parseSendFlags(cmd *cobra.Command) (*sendFlags, error) {
	toEmails, err := cmdutil.ReadRepeatable(cmd, "to", "")
	if err != nil {
		return nil, err
	}
	tags, err := cmdutil.ReadRepeatable(cmd, "tag", "")
	if err != nil {
		return nil, err
	}
	attachments, err := cmdutil.ReadAttachPaths(cmd)
	if err != nil {
		return nil, err
	}
	headers, err := cmdutil.ReadHeaders(cmd, "header")
	if err != nil {
		return nil, err
	}
	sandboxResult, err := cmdutil.ReadSandboxResult(cmd)
	if err != nil {
		return nil, err
	}
	replyTo, err := cmdutil.ReadNonEmpty(cmd, "reply-to", "")
	if err != nil {
		return nil, err
	}
	if err := cmdutil.CheckReservedHeaders(headers, apiReservedHeaders); err != nil {
		return nil, err
	}
	// The API refuses a Reply-To header, in any case, with a reply-to
	if name, ok := cmdutil.FindHeader(headers, "Reply-To"); ok && replyTo != nil {
		return nil, errors.NewValidationError(fmt.Sprintf("--header %s and --reply-to cannot be used together", name), nil)
	}
	fromName, err := cmdutil.ReadSenderName(cmd)
	if err != nil {
		return nil, err
	}
	idempotencyKey, err := cmdutil.ReadIdempotencyKey(cmd)
	if err != nil {
		return nil, err
	}

	flags := cmd.Flags()
	from, _ := flags.GetString("from")
	recipientsFile, _ := flags.GetString("recipients")
	subject, _ := flags.GetString("subject")
	text, _ := flags.GetString("text")
	html, _ := flags.GetString("html")
	amp, _ := flags.GetString("amp")
	textFile, _ := flags.GetString("text-file")
	htmlFile, _ := flags.GetString("html-file")
	ampFile, _ := flags.GetString("amp-file")
	globalSubstitutionsFile, _ := flags.GetString("global-substitutions")
	schedule, _ := flags.GetString("schedule")
	sandbox, _ := flags.GetBool("sandbox")
	showProgress, _ := flags.GetBool("progress")
	maxConcurrency, _ := flags.GetInt("max-concurrency")
	maxRetries, _ := flags.GetInt("max-retries")
	showMetrics, _ := flags.GetBool("show-metrics")
	debugMode, _ := flags.GetBool("debug")

	return &sendFlags{
		// Basic email parameters
		FromEmail:      from,
		FromName:       fromName,
		ReplyTo:        replyTo,
		ToEmails:       toEmails,
		RecipientsFile: recipientsFile,
		Subject:        subject,

		// Content options
		TextContent: text,
		HtmlContent: html,
		AmpContent:  amp,

		// Content files
		TextFile: textFile,
		HtmlFile: htmlFile,
		AmpFile:  ampFile,

		// Substitutions
		GlobalSubstitutionsFile: globalSubstitutionsFile,

		// Advanced options
		ScheduleTime:   schedule,
		Sandbox:        sandbox,
		SandboxResult:  sandboxResult,
		Tags:           tags,
		IdempotencyKey: idempotencyKey,
		Tracking:       readTracking(cmd),
		Attachments:    attachments,
		CustomHeaders:  headers,

		// Batch operation options
		ShowProgress:   showProgress,
		MaxConcurrency: maxConcurrency,
		MaxRetries:     maxRetries,
		ShowMetrics:    showMetrics,
		DebugMode:      debugMode,
	}, nil
}

// readTracking returns the tracking settings of the --track-opens and
// --track-clicks flags given on the command line, or nil when neither is set,
// so the API applies its own settings.
func readTracking(cmd *cobra.Command) *common.Tracking {
	open := cmdutil.ChangedBool(cmd, "track-opens")
	click := cmdutil.ChangedBool(cmd, "track-clicks")
	if open == nil && click == nil {
		return nil
	}
	return &common.Tracking{Open: open, Click: click}
}

func runMessagesSend(cmd *cobra.Command, args []string) error {
	templateID, err := readTemplateID(cmd)
	if err != nil {
		return err
	}
	for _, pair := range conflictingSendFlags {
		if err := cmdutil.RejectTogether(cmd, pair[0], pair[1]); err != nil {
			return err
		}
	}
	if err := validateBatchOptions(cmd); err != nil {
		return err
	}

	flags, err := parseSendFlags(cmd)
	if err != nil {
		return err
	}
	flags.TemplateID = templateID
	if err := promptForMissingFields(cmd, flags); err != nil {
		return err
	}

	// Build the send requests, which checks every input, before
	// authenticating
	sendJobs, err := createSendJobsFromFlags(cmd, flags)
	if err != nil {
		return err
	}

	// Get response handler instance and authenticated client
	handler := printer.GetResponseHandlerFromCommand(cmd)
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Process the batch send operation
	return processBatchSend(cmd, handler, client, flags, sendJobs)
}

// maxScheduleAhead is the latest --schedule the API takes, 7 days from now.
const maxScheduleAhead = 7 * 24 * time.Hour

// maxSendConcurrency is the most send requests --max-concurrency allows in
// flight at once.
const maxSendConcurrency = client.MaxSendConcurrency

// validateBatchOptions checks --max-concurrency and --max-retries. It runs
// before authentication.
func validateBatchOptions(cmd *cobra.Command) error {
	concurrency, _ := cmd.Flags().GetInt("max-concurrency")
	if concurrency < 1 || concurrency > maxSendConcurrency {
		return errors.NewValidationError(fmt.Sprintf("--max-concurrency must be between 1 and %d", maxSendConcurrency), nil)
	}
	if retries, _ := cmd.Flags().GetInt("max-retries"); retries < 0 {
		return errors.NewValidationError("--max-retries must be 0 or more", nil)
	}
	return nil
}

// conflictingSendFlags are the pairs of flags that give the same input two
// ways, which a send rejects together.
var conflictingSendFlags = [][2]string{
	{"to", "recipients"},
	{"text", "text-file"},
	{"html", "html-file"},
	{"amp", "amp-file"},
}

// templateContentFlags are the content flags that template mode rejects, since
// the template supplies the content.
var templateContentFlags = []string{"text", "html", "amp", "text-file", "html-file", "amp-file"}

// readTemplateID returns the --template-id of a template send, or nil for an
// inline send. It runs before authentication.
func readTemplateID(cmd *cobra.Command) (*uuid.UUID, error) {
	value := cmdutil.ChangedString(cmd, "template-id")
	if value == nil {
		return nil, nil
	}
	templateID, err := validation.ParseUUIDField("template ID", *value)
	if err != nil {
		return nil, err
	}
	for _, flag := range templateContentFlags {
		if err := cmdutil.RejectTogether(cmd, "template-id", flag); err != nil {
			return nil, err
		}
	}
	return &templateID, nil
}

// processBatchSend handles the main batch sending workflow. SIGINT and
// SIGTERM interrupt the send: no new request starts, and the command reports
// what was sent and exits with interruptedExitCode.
func processBatchSend(cmd *cobra.Command, handler printer.ResponseHandler, cl client.AhaSendClient, flags *sendFlags, sendJobs []*batch.SendJob) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, a second one stops the command at once
	go func() {
		<-ctx.Done()
		stop()
	}()

	// Set up progress reporting. While the send runs, the progress bar
	// owns stderr: the retry notices and the logs go through it.
	progressReporter := setupProgressReporting(sendJobs, flags, cmd.ErrOrStderr())
	stderr := cmd.ErrOrStderr()
	if progressReporter != nil {
		stderr = progressReporter.Writer(stderr)
	}
	restoreLogs := logger.Redirect(stderr)

	// Process batch
	batchProcessor := batch.NewBatchProcessor(cl, flags.MaxConcurrency, flags.MaxRetries, progressReporter)
	batchProcessor.Notices = stderr
	batchResult, err := batchProcessor.ProcessJobs(ctx, sendJobs)
	restoreLogs()
	if err != nil {
		return err
	}

	// Metrics go to stderr, so JSON output stays the API responses alone.
	if flags.ShowMetrics {
		progress.ShowMetrics(batchResult.Stats, cmd.ErrOrStderr())
	}

	// Where the recipients went is told on stderr, so JSON output stays the
	// API responses alone, and before the results, so it is told even when
	// stdout cannot be written, such as to a closed pipe.
	writeRecipientFilesNotice(cmd.ErrOrStderr(), batchResult, flags.IdempotencyKey, rerunSchedule(flags, sendJobs))
	err = formatBatchResponse(handler, batchResult)
	if !batchResult.Interrupted {
		return err
	}

	// The interruption sets the exit code, so the error of a single request,
	// which formatBatchResponse returns for the caller to print, is printed
	// here.
	single := batchResult.Results[0]
	if len(batchResult.Results) == 1 && single.Outcome != batch.OutcomeNotSent && single.Error != nil {
		_ = handler.HandleError(single.Error)
	}
	interrupted := interruptedError(batchResult)
	// JSON output prints nothing when no request was made, and the error
	// below is not printed in JSON mode, so stderr says why.
	if len(batchResult.NotSentRecipients) == batchResult.TotalRecipients && handler.GetFormat() == "json" {
		fmt.Fprintln(cmd.ErrOrStderr(), interrupted.Error())
	}
	return interrupted
}

// createSendJobsFromFlags builds the send request of flags, inline or
// template, which checks every input, and splits it into batch jobs of up to
// maxBatchSize recipients. A single job sends --idempotency-key as given;
// several send it with "-N" added, as cmdutil.IdempotencyKeyFor says.
func createSendJobsFromFlags(cmd *cobra.Command, flags *sendFlags) ([]*batch.SendJob, error) {
	var err error
	var request *requests.CreateMessageRequest
	var templateRequest *requests.CreateTemplateMessageRequest
	// recipients are the recipients as read, which the failed recipients
	// file saves; a template request's recipients carry the global
	// substitutions too.
	var recipients []common.Recipient
	if flags.TemplateID != nil {
		templateRequest, recipients, err = buildTemplateMessageRequest(*flags.TemplateID, flags)
		if err != nil {
			return nil, err
		}
	} else {
		request, err = buildMessageRequest(cmd, flags)
		if err != nil {
			return nil, err
		}
		recipients = request.Recipients
	}

	batches := recipientBatches(recipients)
	var templateBatches [][]common.Recipient
	if templateRequest != nil {
		templateBatches = recipientBatches(templateRequest.Recipients)
	}
	idempotencyKey := flags.IdempotencyKey
	if idempotencyKey != "" {
		if err := cmdutil.CheckIdempotencyKey(idempotencyKey, len(batches)); err != nil {
			return nil, err
		}
	} else if idempotencyKey, err = generateIdempotencyKey(); err != nil {
		return nil, err
	}
	// The notice of recipients of unknown outcome names the key
	flags.IdempotencyKey = idempotencyKey

	var jobs []*batch.SendJob
	for index, batchRecipients := range batches {
		job := &batch.SendJob{
			IdempotencyKey: cmdutil.IdempotencyKeyFor(idempotencyKey, index, len(batches)),
			BatchIndex:     index,
			Recipients:     batchRecipients,
		}
		if templateRequest != nil {
			batchRequest := *templateRequest
			batchRequest.Recipients = templateBatches[index]
			job.TemplateRequest = &batchRequest
		} else {
			batchRequest := *request
			batchRequest.Recipients = batchRecipients
			job.Request = &batchRequest
		}
		jobs = append(jobs, job)
	}

	logger.Get().WithField("total_jobs", len(jobs)).Debug("Created batch jobs")
	return jobs, nil
}

// setupProgressReporting configures progress reporting based on job
// requirements, drawing on stderr.
func setupProgressReporting(sendJobs []*batch.SendJob, flags *sendFlags, stderr io.Writer) *progress.Reporter {
	// Calculate total recipients (not jobs)
	totalRecipients := 0
	for _, job := range sendJobs {
		totalRecipients += len(job.Recipients)
	}

	// Set up progress reporting if needed; it also collects the metrics
	if totalRecipients > 1 || flags.ShowProgress || flags.ShowMetrics {
		return progress.NewReporter(totalRecipients, flags.ShowProgress, flags.DebugMode, stderr)
	}
	return nil
}

// promptForMissingFields asks for the fields a send needs that no flag gave,
// in the order smtp send asks for them too: the sender of an inline send, the
// recipient of any send without --to or --recipients, then the subject and
// content of an inline send. It asks only when CanPrompt allows
// it; a field that is still missing is a validation error. It runs before
// authentication.
func promptForMissingFields(cmd *cobra.Command, flags *sendFlags) error {
	var err error
	if flags.TemplateID == nil {
		if flags.FromEmail, err = cmdutil.ReadOrAsk(cmd, "from", "Sender email address"); err != nil {
			return err
		}
	}
	if flags.RecipientsFile == "" && len(flags.ToEmails) == 0 {
		toEmail, err := cmdutil.AskIfEmpty(cmd, "", "Recipient email address", "one of --to or --recipients must be provided")
		if err != nil {
			return err
		}
		flags.ToEmails = []string{toEmail}
	}
	if flags.TemplateID == nil {
		if flags.Subject, err = cmdutil.ReadOrAsk(cmd, "subject", "Email subject"); err != nil {
			return err
		}
	}
	if flags.TemplateID == nil && !flags.hasTextOrHTML() {
		if flags.TextContent, err = cmdutil.AskIfEmpty(cmd, "", "Email content (plain text)",
			contentRequiredMessage); err != nil {
			return err
		}
	}
	return nil
}

// contentRequiredMessage is the error of an inline send without text or HTML
// content. The API refuses AMP content alone: AMP is an extra part.
const contentRequiredMessage = "one of --text, --html, --text-file or --html-file must be provided; --amp and --amp-file only add an AMP part"

// hasTextOrHTML reports whether a text or HTML content or content file flag
// is set.
func (f *sendFlags) hasTextOrHTML() bool {
	return f.TextContent != "" || f.HtmlContent != "" || f.TextFile != "" || f.HtmlFile != ""
}

// buildMessageRequest checks the inputs of an inline send and builds its
// request.
func buildMessageRequest(cmd *cobra.Command, flags *sendFlags) (*requests.CreateMessageRequest, error) {
	if err := validation.ValidateEmail("--from", flags.FromEmail); err != nil {
		return nil, err
	}

	recipients, err := readRecipients(flags.RecipientsFile, flags.ToEmails)
	if err != nil {
		return nil, err
	}

	content, err := readEmailContent(cmd, flags)
	if err != nil {
		return nil, err
	}
	if content.TextContent == "" && content.HtmlContent == "" {
		return nil, errors.NewValidationError(contentRequiredMessage, nil)
	}

	globalSubstitutions, attachments, err := loadSubstitutionsAndAttachments(flags.GlobalSubstitutionsFile, flags.Attachments)
	if err != nil {
		return nil, err
	}

	return buildAdvancedMessageRequest(flags, recipients, content, globalSubstitutions, attachments)
}

// buildTemplateMessageRequest checks the inputs of a template send and builds
// its request, whose recipients carry the global substitutions, and returns
// the recipients as read. The template supplies the content, and the sender
// and subject when they are not given.
func buildTemplateMessageRequest(templateID uuid.UUID, flags *sendFlags) (*requests.CreateTemplateMessageRequest, []common.Recipient, error) {
	if flags.FromEmail != "" {
		if err := validation.ValidateEmail("--from", flags.FromEmail); err != nil {
			return nil, nil, err
		}
	}

	recipients, err := readRecipients(flags.RecipientsFile, flags.ToEmails)
	if err != nil {
		return nil, nil, err
	}

	globalSubstitutions, attachments, err := loadSubstitutionsAndAttachments(flags.GlobalSubstitutionsFile, flags.Attachments)
	if err != nil {
		return nil, nil, err
	}

	// The inline builder parses the options both endpoints share
	base, err := buildAdvancedMessageRequest(flags, mergeGlobalSubstitutions(recipients, globalSubstitutions), &contentData{}, nil, attachments)
	if err != nil {
		return nil, nil, err
	}

	request := &requests.CreateTemplateMessageRequest{
		TemplateID:    templateID,
		Recipients:    base.Recipients,
		Subject:       base.Subject,
		ReplyTo:       base.ReplyTo,
		Attachments:   base.Attachments,
		Headers:       base.Headers,
		Tags:          base.Tags,
		Sandbox:       base.Sandbox,
		SandboxResult: base.SandboxResult,
		Tracking:      base.Tracking,
		Retention:     base.Retention,
		Schedule:      base.Schedule,
	}
	if flags.FromEmail != "" {
		request.From = &base.From
	}

	return request, recipients, nil
}

// loadSubstitutionsAndAttachments reads the --global-substitutions file and
// the --attach files of a send. Both are nil when their flag is not set.
func loadSubstitutionsAndAttachments(globalSubstitutionsFile string, attachmentPaths []string) (map[string]interface{}, []common.Attachment, error) {
	var globalSubstitutions map[string]interface{}
	if globalSubstitutionsFile != "" {
		loaded, err := loadGlobalSubstitutions(globalSubstitutionsFile)
		if err != nil {
			return nil, nil, err
		}
		globalSubstitutions = loaded
	}

	attachments, err := processAttachments(attachmentPaths)
	if err != nil {
		return nil, nil, err
	}
	if err := checkAttachmentsFit(attachments); err != nil {
		return nil, nil, err
	}
	return globalSubstitutions, attachments, nil
}

// contentData holds the processed email content
type contentData struct {
	TextContent string
	HtmlContent string
	AmpContent  string
}

// readEmailContent returns the content of an inline send: the --text, --html
// and --amp values, or the content of their files. A read error names the
// file flag as the user gave it, such as --text-template.
func readEmailContent(cmd *cobra.Command, flags *sendFlags) (*contentData, error) {
	content := &contentData{
		TextContent: flags.TextContent,
		HtmlContent: flags.HtmlContent,
		AmpContent:  flags.AmpContent,
	}

	files := []struct {
		flag  string
		path  string
		field *string
	}{
		{"text-file", flags.TextFile, &content.TextContent},
		{"html-file", flags.HtmlFile, &content.HtmlContent},
		{"amp-file", flags.AmpFile, &content.AmpContent},
	}
	for _, file := range files {
		if file.path == "" {
			continue
		}
		fromFile, err := cmdutil.ReadFlagFile(cmdutil.TypedName(cmd, file.flag), file.path)
		if err != nil {
			return nil, err
		}
		*file.field = fromFile
	}

	return content, nil
}

// buildAdvancedMessageRequest builds the SDK request of an inline send from
// flags and the recipients, content, substitutions and attachments read for it
func buildAdvancedMessageRequest(
	flags *sendFlags, recipients []common.Recipient, content *contentData,
	globalSubstitutions map[string]interface{}, attachments []common.Attachment,
) (*requests.CreateMessageRequest, error) {
	// Create the base request with required fields
	request := &requests.CreateMessageRequest{
		From:       common.SenderAddress{Email: flags.FromEmail, Name: flags.FromName},
		Recipients: recipients,
		Subject:    flags.Subject,
	}
	if flags.ReplyTo != nil {
		if err := validation.ValidateEmail("--reply-to", *flags.ReplyTo); err != nil {
			return nil, err
		}
		request.ReplyTo = &common.SenderAddress{Email: *flags.ReplyTo}
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

	// Set sandbox mode; parseSendFlags made sure the result is valid
	if flags.Sandbox {
		request.Sandbox = ahasend.Bool(true)
		request.SandboxResult = ahasend.String(flags.SandboxResult)
	}

	// Set tags
	if len(flags.Tags) > 0 {
		request.Tags = flags.Tags
	}

	request.Tracking = flags.Tracking
	request.Headers = flags.CustomHeaders

	// Parse and set schedule time
	if flags.ScheduleTime != "" {
		parsedTime, err := timeparse.ParseTimeAhead("schedule", flags.ScheduleTime, "30m or 2h", 0, maxScheduleAhead)
		if err != nil {
			return nil, err
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
		return "", fmt.Errorf("failed to generate an idempotency key: %w", err)
	}

	// Add timestamp prefix for better debugging
	timestamp := time.Now().Unix()
	return fmt.Sprintf("cli-%d-%s", timestamp, hex.EncodeToString(bytes)), nil
}
