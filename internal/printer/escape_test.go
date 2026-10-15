package printer

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hostileSubject holds ESC, BEL and the C1 character CSI, which a terminal
// would act on.
const hostileSubject = "Hi\x1b[2J\x07\u009b31m"

// escapedSubject is hostileSubject as table and plain output show it.
const escapedSubject = `Hi\u001b[2J\u0007\u009b31m`

func hostileMessageList() *responses.PaginatedMessagesResponse {
	message := styleMessage()
	message.Subject = hostileSubject
	return &responses.PaginatedMessagesResponse{Data: []responses.Message{message}, Pagination: common.PaginationInfo{}}
}

// printMessageList prints list in format and returns the output.
func printMessageList(t *testing.T, format string, list *responses.PaginatedMessagesResponse) string {
	t.Helper()
	var out bytes.Buffer
	handler, err := NewResponseHandler(format, &out, &out)
	require.NoError(t, err)
	require.NoError(t, handler.HandleMessageList(list, ListConfig{SuccessMessage: "Messages"}))
	return out.String()
}

func TestTableAndPlainEscapeControlCharacters(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			out := printMessageList(t, format, hostileMessageList())

			assert.Contains(t, out, escapedSubject)
			assert.NotContains(t, out, "\x1b")
			assert.NotContains(t, out, "\x07")
			assert.NotContains(t, out, "\u009b")
		})
	}
}

// TestTableColumnsLineUpWithEscapedValues checks that a table measures a
// value as it shows it, escaped, so every line of the table has one width.
func TestTableColumnsLineUpWithEscapedValues(t *testing.T) {
	out := printMessageList(t, "table", hostileMessageList())

	var widths []int
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.ContainsAny(line, "│┌└├") {
			widths = append(widths, len([]rune(line)))
		}
	}
	require.NotEmpty(t, widths)
	for _, width := range widths {
		assert.Equal(t, widths[0], width)
	}
}

// TestTableLinesUpCRLFAndTabs checks that a table cell with a CRLF line
// break and a tab shows the lines and four spaces for the tab, with every
// line of the table of one width.
func TestTableLinesUpCRLFAndTabs(t *testing.T) {
	message := styleMessage()
	message.Subject = "first\r\nsecond\tpart"
	out := printMessageList(t, "table", &responses.PaginatedMessagesResponse{Data: []responses.Message{message}})

	assert.NotContains(t, out, `\u000d`)
	assert.NotContains(t, out, "\t")
	assert.Regexp(t, `│ first +│`, out)
	assert.Regexp(t, `│ second    part +│`, out)
	var widths []int
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.ContainsAny(line, "│┌└├") {
			widths = append(widths, len([]rune(line)))
		}
	}
	require.NotEmpty(t, widths)
	for _, width := range widths {
		assert.Equal(t, widths[0], width)
	}
}

func TestCSVAndJSONKeepControlCharacters(t *testing.T) {
	assert.Contains(t, printMessageList(t, "csv", hostileMessageList()), hostileSubject)
	// JSON prints the API's bytes, which escape C0 characters and keep C1
	assert.Contains(t, printMessageList(t, "json", hostileMessageList()), "Hi\\u001b[2J\\u0007\u009b31m")
}

func TestEscapeDisplayTextKeepsTabsAndLineEnds(t *testing.T) {
	assert.Equal(t, "a\tb\nc\r\nd\\u000de", EscapeDisplayText("a\tb\nc\r\nd\re"))
	assert.Equal(t, `a\u0009b\u000ac\u007f`, EscapeControl("a\tb\nc\x7f"))
}

func TestPlainIndentsTheFurtherLinesOfAValue(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, writeLabelLines(&out, "  ", [][]string{{"Reason", "first\nsecond"}, {"Code", "550"}}))

	assert.Equal(t, "  Reason: first\n          second\n  Code: 550\n", out.String())
}

func TestTableWrapsAValueOfSeveralLinesInItsCell(t *testing.T) {
	message := styleMessage()
	message.Subject = "first\nsecond"
	out := printMessageList(t, "table", &responses.PaginatedMessagesResponse{Data: []responses.Message{message}})

	assert.Regexp(t, `│ first +│`, out)
	assert.Regexp(t, `│ second +│`, out)
}

func TestTagsWithACommaOrAQuoteAreQuoted(t *testing.T) {
	assert.Equal(t, `news, "a, b", "say ""hi"""`, formatTags([]string{"news", "a, b", `say "hi"`}))
	assert.Equal(t, "", formatTags(nil))

	message := styleMessage()
	message.Tags = []string{"news", "weekly, digest"}
	list := &responses.PaginatedMessagesResponse{Data: []responses.Message{message}}
	var out bytes.Buffer
	handler, err := NewResponseHandler("plain", &out, &out)
	require.NoError(t, err)
	require.NoError(t, handler.HandleMessageList(list, ListConfig{SuccessMessage: "Messages", Detailed: true}))
	assert.Contains(t, out.String(), `Tags: news, "weekly, digest"`+"\n")
}

// TestHiddenAndReorderingCharactersAreEscaped checks that the zero-width and
// bidirectional formatting characters are escaped in table and plain output,
// and left as they are in JSON.
func TestHiddenAndReorderingCharactersAreEscaped(t *testing.T) {
	hidden := []rune{'\u200b', '\u200d', '\u200f', '\u202a', '\u202e', '\u2066', '\u2069', '\ufeff'}
	for _, r := range hidden {
		assert.Equal(t, fmt.Sprintf(`a\u%04xb`, r), EscapeControl("a"+string(r)+"b"))
		assert.Equal(t, fmt.Sprintf(`a\u%04xb`, r), EscapeDisplayText("a"+string(r)+"b"))
	}
	// The neighbours of each range are shown as they are
	for _, r := range []rune{'\u200a', '\u2010', '\u2029', '\u202f', '\u2065', '\u206a', '\ufefe'} {
		assert.Equal(t, "a"+string(r)+"b", EscapeControl("a"+string(r)+"b"))
	}

	message := styleMessage()
	message.Subject = "invoice\u202egnp.exe"
	list := &responses.PaginatedMessagesResponse{Data: []responses.Message{message}}
	for _, format := range []string{"table", "plain"} {
		out := printMessageList(t, format, list)
		assert.Contains(t, out, `invoice\u202egnp.exe`, format)
		assert.NotContains(t, out, "\u202e", format)
	}
	assert.Contains(t, printMessageList(t, "json", list), "invoice\u202egnp.exe")
}
