package messages

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/batch"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/common"
)

// formatBatchResponse prints the API response of each send request; the
// jobs an interruption kept from being sent made no request and print
// nothing, so a send interrupted before its first request prints nothing.
func formatBatchResponse(handler printer.ResponseHandler, batchResult *batch.BatchResult) error {
	var sent []*batch.SendResult
	for _, result := range batchResult.Results {
		if result.Outcome != batch.OutcomeNotSent {
			sent = append(sent, result)
		}
	}
	// An interruption before the first request leaves nothing to print
	if len(sent) == 0 {
		return nil
	}

	// A single send request prints its API response, or returns its error
	// so the API's error response is printed.
	if len(batchResult.Results) == 1 && len(sent) == 1 {
		result := sent[0]
		if result.Error != nil {
			return result.Error
		}
		if err := handler.HandleCreateMessage(result.Response, printer.ResultConfig{
			SuccessMessage: sendSummary(batchResult),
		}); err != nil {
			return err
		}
		return recipientsFailedError(batchResult)
	}

	// Several send requests, each holding up to maxBatchSize recipients,
	// print one API response per request, in order, and count recipients.
	results := make([]printer.APIResult, 0, len(sent))
	for _, result := range sent {
		first := result.Job.BatchIndex*maxBatchSize + 1
		results = append(results, printer.APIResult{
			Item:     fmt.Sprintf("recipients %d-%d", first, first+len(result.Job.Recipients)-1),
			Response: result.Response,
			Err:      result.Error,
			Unknown:  result.Outcome == batch.OutcomeUnknown,
			Failures: rejectedRecipients(result),
			Size:     len(result.Job.Recipients),
		})
	}

	if err := handler.HandleAPIResults(results, printer.ResultConfig{
		SuccessMessage: sendSummary(batchResult),
	}); err != nil {
		return err
	}
	return recipientsFailedError(batchResult)
}

// sendSummary returns the heading of a send's result: whether every
// recipient was sent, some were, or none was.
func sendSummary(batchResult *batch.BatchResult) string {
	total := batchResult.TotalRecipients
	notSent := len(batchResult.FailedRecipients) + len(batchResult.NotSentRecipients) + len(batchResult.UnknownRecipients)
	switch {
	case notSent == 0:
		return "Message sent successfully"
	case notSent >= total:
		return "Message not sent to any recipient"
	default:
		return fmt.Sprintf("Message sent to %d of %d recipients", total-notSent, total)
	}
}

// recipientsFailedError returns the error for a send in which some
// recipients failed, whether their request failed or the API did not accept
// them, were not sent after a request too large for the API, or the outcome
// of their request is unknown, or nil when every recipient was sent.
func recipientsFailedError(batchResult *batch.BatchResult) error {
	failed, unknown := len(batchResult.FailedRecipients), len(batchResult.UnknownRecipients)
	notSent := len(batchResult.NotSentRecipients)
	if unknown == 0 && notSent == 0 {
		if failed > 0 {
			return errors.NewItemsFailedError(failed, batchResult.TotalRecipients, "recipients")
		}
		return nil
	}
	var parts []string
	if notSent > 0 {
		parts = append(parts, fmt.Sprintf("%d were not sent", notSent))
	}
	if unknown > 0 {
		parts = append(parts, fmt.Sprintf("the outcome of %d is unknown", unknown))
	}
	message := fmt.Sprintf("%d of %d recipients failed", failed, batchResult.TotalRecipients)
	if len(parts) == 2 {
		message += ", " + parts[0] + " and " + parts[1]
	} else {
		message += " and " + parts[0]
	}
	if batchResult.TooLarge {
		message += ": a request was too large for the API (HTTP 413)"
	}
	return errors.NewResultPrintedError(message, 1)
}

// rejectedRecipients lists the recipients of a sent request that the API
// did not accept, each with the API's error.
func rejectedRecipients(result *batch.SendResult) []string {
	var rejected []string
	for _, recipient := range batch.RejectedRecipients(result) {
		rejected = append(rejected, fmt.Sprintf("%s: %s", recipient.Email, recipient.Error))
	}
	return rejected
}

// interruptedError is the error of an interrupted send: how many recipients
// it did not send, and how many failed or are of unknown outcome.
func interruptedError(batchResult *batch.BatchResult) error {
	message := fmt.Sprintf("interrupted: %d of %d recipients were not sent",
		len(batchResult.NotSentRecipients), batchResult.TotalRecipients)
	if failed := len(batchResult.FailedRecipients); failed > 0 {
		message += fmt.Sprintf(", %d failed", failed)
	}
	if unknown := len(batchResult.UnknownRecipients); unknown > 0 {
		message += fmt.Sprintf(", the outcome of %d is unknown", unknown)
	}
	return errors.NewResultPrintedError(message, interruptedExitCode)
}

// interruptedExitCode is the exit code of a send stopped by SIGINT or
// SIGTERM, 128 plus the number of SIGINT, as shells report it.
const interruptedExitCode = 130

// rerunSchedule returns the --schedule a run of the same command must give
// to send the same requests again: the time a relative --schedule stood for,
// as the requests carry it, since the API takes an idempotency key again
// only with the same request body. It returns "" when --schedule was an RFC
// 3339 time or not given.
func rerunSchedule(flags *sendFlags, sendJobs []*batch.SendJob) string {
	if flags.ScheduleTime == "" || len(sendJobs) == 0 {
		return ""
	}
	if _, err := time.Parse(time.RFC3339, flags.ScheduleTime); err == nil {
		return ""
	}
	var schedule *common.MessageSchedule
	if job := sendJobs[0]; job.Request != nil {
		schedule = job.Request.Schedule
	} else if job.TemplateRequest != nil {
		schedule = job.TemplateRequest.Schedule
	}
	if schedule == nil || schedule.FirstAttempt == nil {
		return ""
	}
	return schedule.FirstAttempt.Format(time.RFC3339Nano)
}

// writeRecipientFilesNotice tells where the recipients to send again and the
// recipients whose outcome is unknown were saved, or why they could not be.
// For the latter it always names the idempotency keys of their requests, and
// tells how to send them again safely with idempotencyKey, the key of the
// send, and schedule, the --schedule to give in place of a relative one, if
// any. A send without either writes nothing.
func writeRecipientFilesNotice(w io.Writer, batchResult *batch.BatchResult, idempotencyKey, schedule string) {
	if batchResult.FailedSaveError != nil {
		fmt.Fprintf(w, "Could not save the failed recipients: %v\n", batchResult.FailedSaveError)
	}
	if batchResult.FailedRecipientsFile != "" {
		fmt.Fprintf(w, "Failed recipients saved to %s; send them again with --recipients %s\n",
			batchResult.FailedRecipientsFile, cmdutil.QuotePath(batchResult.FailedRecipientsFile))
	}
	if len(batchResult.UnknownRecipients) == 0 {
		return
	}
	if batchResult.UnknownSaveError != nil {
		fmt.Fprintf(w, "Could not save the recipients whose outcome is unknown: %v\n", batchResult.UnknownSaveError)
	}
	var keys []string
	for _, result := range batchResult.Results {
		if result.Outcome == batch.OutcomeUnknown {
			keys = append(keys, cmdutil.ShellQuote(result.Job.IdempotencyKey))
		}
	}
	requests, keyNoun := "request", "idempotency key"
	if len(keys) > 1 {
		requests, keyNoun = "requests", "idempotency keys"
	}
	keyList := strings.Join(keys, ", ")
	saved := "."
	if batchResult.UnknownRecipientsFile != "" {
		saved = ", and are saved to " + batchResult.UnknownRecipientsFile + "."
		// The file names the key of each recipient's request
		if len(keys) > maxListedKeys {
			keyList = fmt.Sprintf("%s and %d more, listed in %s", strings.Join(keys[:maxListedKeys], ", "),
				len(keys)-maxListedKeys, batchResult.UnknownRecipientsFile)
			saved = ", and are saved there."
		}
	}
	rerun := "--idempotency-key " + cmdutil.ShellQuote(idempotencyKey)
	if schedule != "" {
		rerun += " and --schedule " + schedule + ", the time the relative --schedule stood for,"
	}
	fmt.Fprintf(w, "The outcome of the %s of %d recipients (%s %s) is unknown: they may have been sent%s "+
		"Check them with 'ahasend messages list', or run the same command again with "+
		"%s within %s: the API answers a request it already took with its stored result "+
		"instead of sending it again\n",
		requests, len(batchResult.UnknownRecipients), keyNoun, keyList, saved,
		rerun, idempotencyReplayWindow)
}

// maxListedKeys is how many idempotency keys of requests of unknown outcome
// the notice names when the file of their recipients names them all.
const maxListedKeys = 3

// idempotencyReplayWindow is how long the API keeps the result of a send
// request for its idempotency key.
const idempotencyReplayWindow = "24 hours"
