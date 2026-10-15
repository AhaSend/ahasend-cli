package printer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
)

// APIResult is the outcome of one API call in a command that makes the same
// call for several items.
type APIResult struct {
	Item     string   // what the call was for, such as a message ID or a batch label
	Response any      // decoded result on success
	Err      error    // failure
	Unknown  bool     // the call's outcome is unknown: Err says why the API had not decided it
	Failures []string // parts of a successful call that failed, such as recipients the API did not accept
	// Size is how many items the call is for, such as the recipients of a
	// send request, each a part that Failures can name; 0 counts the call
	// as one item.
	Size int
}

// HandleAPIResults prints one array element per call, in order: the API's
// body for a success, or null when the API sent no body, and for a failure
// the API's error body, or the error object HandleError prints when there is
// no JSON body.
func (h *jsonHandler) HandleAPIResults(results []APIResult, config ResultConfig) error {
	var buffer bytes.Buffer
	buffer.WriteByte('[')
	for i, result := range results {
		if i > 0 {
			buffer.WriteByte(',')
		}
		element, err := apiResultJSON(result)
		if err != nil {
			return err
		}
		buffer.Write(element)
	}
	buffer.WriteByte(']')
	return h.printRaw(buffer.Bytes())
}

// apiResultJSON returns the JSON element HandleAPIResults prints for result.
func apiResultJSON(result APIResult) ([]byte, error) {
	if result.Err != nil {
		if body, ok := apiErrorBody(result.Err); ok {
			return body, nil
		}
		return encodeJSON(errorObject(result.Err), "")
	}
	if isNilPointer(result.Response) {
		return []byte("null"), nil
	}
	if body, ok := apiresponse.Body(result.Response); ok {
		if len(bytes.TrimSpace(body)) == 0 {
			return []byte("null"), nil
		}
		return body, nil
	}
	return encodeJSON(result.Response, "")
}

// encodeJSON encodes value without HTML escaping and without a trailing
// newline, as the JSON handler prints, indenting each level by indent; an
// empty indent encodes compactly.
func encodeJSON(value any, indent string) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}

// HandleAPIResults writes config.SuccessMessage and how many items succeeded
// and failed, and how many are of unknown outcome when any is, then a row per
// call with its outcome and error.
func (h *tableHandler) HandleAPIResults(results []APIResult, config ResultConfig) error {
	succeeded, failed, unknown := countAPIResults(results)
	counts := fmt.Sprintf("Succeeded: %d  Failed: %d", succeeded, failed)
	if unknown > 0 {
		counts += fmt.Sprintf("  Unknown: %d", unknown)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n%s\n\n", config.SuccessMessage, counts); err != nil {
		return err
	}
	rows := make([][]string, len(results))
	for i, result := range results {
		rows[i] = apiResultRow(result)
	}
	h.renderTable([]any{"Item", "Outcome", "Error"}, rows...)
	return nil
}

// HandleAPIResults writes config.SuccessMessage and how many items succeeded
// and failed, and how many are of unknown outcome when any is, then each
// call's item, outcome and error.
func (h *plainHandler) HandleAPIResults(results []APIResult, config ResultConfig) error {
	succeeded, failed, unknown := countAPIResults(results)
	if _, err := fmt.Fprintf(h.writer, "%s\nSucceeded: %d\nFailed: %d\n", config.SuccessMessage, succeeded, failed); err != nil {
		return err
	}
	if unknown > 0 {
		if _, err := fmt.Fprintf(h.writer, "Unknown: %d\n", unknown); err != nil {
			return err
		}
	}
	for i, result := range results {
		row := apiResultRow(result)
		if _, err := fmt.Fprintf(h.writer, "\nResult %d:\n", i+1); err != nil {
			return err
		}
		if err := writeLabelLines(h.writer, "  ", [][]string{
			{"Item", row[0]},
			{"Outcome", row[1]},
			{"Error", row[2]},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (h *csvHandler) HandleAPIResults(results []APIResult, config ResultConfig) error {
	rows := make([][]string, len(results))
	for i, result := range results {
		rows[i] = apiResultRow(result)
	}
	return h.writeRows([]string{"item", "outcome", "error"}, rows...)
}

// apiResultRow returns a call's item, its outcome, and its errors: "unknown"
// or "failed" with the error of a call whose outcome is unknown or that
// failed, "partial" with the failures of a successful call that reports
// some, or "success".
func apiResultRow(result APIResult) []string {
	if result.Unknown {
		return []string{result.Item, "unknown", result.Err.Error()}
	}
	if result.Err != nil {
		return []string{result.Item, "failed", result.Err.Error()}
	}
	if len(result.Failures) > 0 {
		return []string{result.Item, "partial", strings.Join(result.Failures, "; ")}
	}
	return []string{result.Item, "success", ""}
}

// countAPIResults returns how many items succeeded, how many failed, and how
// many are of unknown outcome: every item of a call that failed or whose
// outcome is unknown, and each part of a successful call that Failures names.
func countAPIResults(results []APIResult) (succeeded, failed, unknown int) {
	for _, result := range results {
		size := max(result.Size, 1)
		switch {
		case result.Unknown:
			unknown += size
		case result.Err != nil:
			failed += size
		default:
			partsFailed := min(len(result.Failures), size)
			failed += partsFailed
			succeeded += size - partsFailed
		}
	}
	return succeeded, failed, unknown
}
