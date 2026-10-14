package cmdutil

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parsed returns a command whose flags were added by addFlags and parsed from
// args.
func parsed(t *testing.T, addFlags func(*cobra.Command), args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	addFlags(cmd)
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

// ioCommand returns a command that reads stdin and writes to the returned
// stdout and stderr.
func ioCommand(t *testing.T, stdin string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	var stdout, stderr bytes.Buffer
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr
}

func assertValidationError(t *testing.T, err error, message string) {
	t.Helper()
	require.Error(t, err)
	var cliErr *errors.CLIError
	require.True(t, stderrors.As(err, &cliErr), "want a CLIError, got %T", err)
	assert.Equal(t, errors.ErrCodeValidation, cliErr.Code)
	assert.Contains(t, err.Error(), message)
}

// Changed readers -------------------------------------------------------------

func addPartialFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "")
	cmd.Flags().Bool("enabled", false, "")
	AddRepeatableFlag(cmd, "tag", "")
}

func TestChangedString_UnsetIsNil(t *testing.T) {
	assert.Nil(t, ChangedString(parsed(t, addPartialFlags), "name"))
}

func TestChangedString_EmptyValueIsSent(t *testing.T) {
	got := ChangedString(parsed(t, addPartialFlags, "--name", ""), "name")
	require.NotNil(t, got)
	assert.Equal(t, "", *got)
}

func TestNonEmptyString_UnsetIsNil(t *testing.T) {
	assert.Nil(t, NonEmptyString(parsed(t, addPartialFlags), "name"))
}

func TestNonEmptyString_EmptyValueIsNil(t *testing.T) {
	assert.Nil(t, NonEmptyString(parsed(t, addPartialFlags, "--name", ""), "name"))
}

func TestNonEmptyString_ReturnsValue(t *testing.T) {
	got := NonEmptyString(parsed(t, addPartialFlags, "--name", "news"), "name")
	require.NotNil(t, got)
	assert.Equal(t, "news", *got)
}

func TestChangedBool_UnsetIsNil(t *testing.T) {
	assert.Nil(t, ChangedBool(parsed(t, addPartialFlags), "enabled"))
}

func TestChangedBool_FalseIsSent(t *testing.T) {
	got := ChangedBool(parsed(t, addPartialFlags, "--enabled=false"), "enabled")
	require.NotNil(t, got)
	assert.False(t, *got)
}

func TestReadNonEmpty_UnsetIsNil(t *testing.T) {
	got, err := ReadNonEmpty(parsed(t, addPartialFlags), "name", "")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestReadNonEmpty_ReturnsValue(t *testing.T) {
	got, err := ReadNonEmpty(parsed(t, addPartialFlags, "--name", "a"), "name", "")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "a", *got)
}

func TestReadNonEmpty_EmptyValueRejected(t *testing.T) {
	_, err := ReadNonEmpty(parsed(t, addPartialFlags, "--name", ""), "name", "")
	assertValidationError(t, err, "--name cannot be empty")
	assert.NotContains(t, err.Error(), "use --")
}

func TestReadNonEmpty_EmptyValuePointsToClearFlag(t *testing.T) {
	_, err := ReadNonEmpty(parsed(t, addPartialFlags, "--name", ""), "name", "clear-name")
	assertValidationError(t, err, "--name cannot be empty; use --clear-name to clear it")
}

// Repeatable flags ------------------------------------------------------------

func TestReadRepeatable_UnsetIsNil(t *testing.T) {
	got, err := ReadRepeatable(parsed(t, addPartialFlags), "tag", "")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestReadRepeatable_ReturnsValues(t *testing.T) {
	got, err := ReadRepeatable(parsed(t, addPartialFlags, "--tag", "a", "--tag", "b"), "tag", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestReadRepeatable_SplitsOnCommas(t *testing.T) {
	got, err := ReadRepeatable(parsed(t, addPartialFlags, "--tag", "a,b", "--tag", "c"), "tag", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestReadRepeatable_QuotedValueKeepsItsComma(t *testing.T) {
	got, err := ReadRepeatable(parsed(t, addPartialFlags, "--tag", `"Doe, Jane <j@example.com>"`), "tag", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"Doe, Jane <j@example.com>"}, got)
}

func TestReadRepeatable_EmptyValueRejected(t *testing.T) {
	for _, value := range []string{"", "a,,b", "a,"} {
		t.Run(value, func(t *testing.T) {
			_, err := ReadRepeatable(parsed(t, addPartialFlags, "--tag", value), "tag", "")
			assertValidationError(t, err, "--tag cannot be empty")
			assert.NotContains(t, err.Error(), "use --")
		})
	}
}

func TestReadRepeatable_TrimsEachValue(t *testing.T) {
	got, err := ReadRepeatable(parsed(t, addPartialFlags, "--tag", "news, weekly", "--tag", " digest "), "tag", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"news", "weekly", "digest"}, got)
}

func TestReadRepeatable_ValueOfSpacesRejected(t *testing.T) {
	_, err := ReadRepeatable(parsed(t, addPartialFlags, "--tag", "a, "), "tag", "")
	assertValidationError(t, err, "--tag cannot be empty")
}

// TestRepeatableFlag_UnsplittableValueIsAValidationError checks that a value
// the flag cannot split, such as one with an unclosed double quote, fails as
// a validation error that names the flag and how to quote.
func TestRepeatableFlag_UnsplittableValueIsAValidationError(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addPartialFlags(cmd)

	err := cmd.ParseFlags([]string{"--tag", `"news, weekly`})

	assertValidationError(t, err, `invalid --tag value "\"news, weekly": a value with a comma or a double quote must be in double quotes, `+
		`with each double quote in it doubled, such as '"a, ""b"""'`)
}

func TestReadRepeatable_EmptyValuePointsToClearFlag(t *testing.T) {
	_, err := ReadRepeatable(parsed(t, addPartialFlags, "--tag", ""), "tag", "clear-tags")
	assertValidationError(t, err, "--tag cannot be empty; use --clear-tags to clear it")
}

// IP allow list ---------------------------------------------------------------

func addHeaderFlag(cmd *cobra.Command) { AddRepeatableFlag(cmd, "header", "") }

func TestReadHeaders_UnsetIsNil(t *testing.T) {
	headers, err := ReadHeaders(parsed(t, addHeaderFlag), "header")
	require.NoError(t, err)
	assert.Nil(t, headers)
}

func TestReadHeaders_ParsesNameAndValue(t *testing.T) {
	headers, err := ReadHeaders(parsed(t, addHeaderFlag, "--header", "X-Test: a:b", "--header", "X-Empty:"), "header")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"X-Test": "a:b", "X-Empty": ""}, headers)
}

// TestReadHeaders_RejectsAValueSplitAtAComma checks that the part of a
// header value after an unquoted ", ", which the flag splits off, fails
// instead of being sent as a header of its own.
func TestReadHeaders_RejectsAValueSplitAtAComma(t *testing.T) {
	_, err := ReadHeaders(parsed(t, addHeaderFlag, "--header", "List-Unsubscribe: <mailto:u@example.com>, <https://example.com/u>"), "header")

	assertValidationError(t, err, `invalid --header value " <https://example.com/u>": " <https" is not a header name, `+
		`which is visible ASCII without spaces; a header value with a comma must be in double quotes: --header '"Name: a, b"'`)
}

// TestReadHeaders_RejectsAValueSplitAtACommaWithoutASpace checks that the
// part of a header value after an unquoted ",", which has no colon, fails
// with the quoting hint, while a value of its own without a colon does not
// get it.
func TestReadHeaders_RejectsAValueSplitAtACommaWithoutASpace(t *testing.T) {
	_, err := ReadHeaders(parsed(t, addHeaderFlag, "--header", "X-Note: first,second"), "header")
	assertValidationError(t, err, `invalid --header value "second": not Name: value; `+
		`a header value with a comma must be in double quotes: --header '"Name: a, b"'`)

	_, err = ReadHeaders(parsed(t, addHeaderFlag, "--header", "X-Note: first", "--header", "second"), "header")
	assertValidationError(t, err, "--header must be Name: value")
}

func TestReadHeaders_RejectsANameTheAPIRefuses(t *testing.T) {
	for _, value := range []string{"X Test: a", "X-Tést: a", " X-Test: a"} {
		t.Run(value, func(t *testing.T) {
			_, err := ReadHeaders(parsed(t, addHeaderFlag, "--header", value), "header")
			assertValidationError(t, err, "is not a header name")
		})
	}
}

func TestReadHeaders_QuotedValueKeepsItsComma(t *testing.T) {
	headers, err := ReadHeaders(parsed(t, addHeaderFlag, "--header",
		`"List-Unsubscribe: <mailto:u@example.com>, <https://example.com/u>"`, "--header", "X-Test : a"), "header")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"List-Unsubscribe": "<mailto:u@example.com>, <https://example.com/u>",
		"X-Test":           "a",
	}, headers)
}

func TestReadHeaders_RejectsMissingColonOrName(t *testing.T) {
	for _, value := range []string{"X-Test", ": value", " :value"} {
		t.Run(value, func(t *testing.T) {
			_, err := ReadHeaders(parsed(t, addHeaderFlag, "--header", value), "header")
			assertValidationError(t, err, "--header must be Name: value")
		})
	}
}

func addAttachFlag(cmd *cobra.Command) { AddRepeatableFlag(cmd, "attach", "") }

// TestReadAttachPaths_HintsAtQuotingAFileNameWithAComma checks that a path
// that does not exist fails with the quoting hint when it came from a value
// the flag split at a comma, and without it otherwise.
func TestReadAttachPaths_HintsAtQuotingAFileNameWithAComma(t *testing.T) {
	_, err := ReadAttachPaths(parsed(t, addAttachFlag, "--attach", "inv,2026.txt"))
	assertValidationError(t, err, `invalid --attach value "inv": no such file; `+
		`a file name with a comma must be in double quotes: --attach '"a,b.pdf"'`)

	paths, err := ReadAttachPaths(parsed(t, addAttachFlag, "--attach", "inv.txt"))
	require.NoError(t, err)
	assert.Equal(t, []string{"inv.txt"}, paths)
}

// TestReadAttachPaths_TakesAQuotedFileNameWithAComma checks that a quoted
// file name keeps its comma.
func TestReadAttachPaths_TakesAQuotedFileNameWithAComma(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inv,2026.txt")
	require.NoError(t, os.WriteFile(path, []byte("invoice"), 0o600))

	paths, err := ReadAttachPaths(parsed(t, addAttachFlag, "--attach", `"`+path+`"`))
	require.NoError(t, err)
	assert.Equal(t, []string{path}, paths)
}

// TestReadHeaders_RejectsANameGivenTwice checks that a header name given
// twice, in the same case or another, fails rather than keeping one value or
// sending both.
func TestReadHeaders_RejectsANameGivenTwice(t *testing.T) {
	for _, second := range []string{"X-Test: b", "x-test: b"} {
		t.Run(second, func(t *testing.T) {
			_, err := ReadHeaders(parsed(t, addHeaderFlag, "--header", "X-Test: a", "--header", second), "header")
			name, _, _ := strings.Cut(second, ":")
			assertValidationError(t, err, "--header gives the header "+name+" twice; header names ignore case")
		})
	}
}

func addTimeRangeFlags(cmd *cobra.Command) {
	cmd.Flags().String("from-time", "", "")
	cmd.Flags().String("to-time", "", "")
}

func TestReadPastTimeRange_UnsetIsNil(t *testing.T) {
	from, to, err := ReadPastTimeRange(parsed(t, addTimeRangeFlags))
	require.NoError(t, err)
	assert.Nil(t, from)
	assert.Nil(t, to)
}

func TestReadPastTimeRange_ReturnsBothTimes(t *testing.T) {
	from, to, err := ReadPastTimeRange(parsed(t, addTimeRangeFlags, "--from-time", "2026-01-01T00:00:00Z", "--to-time", "1h"))
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), from.UTC())
	assert.NotNil(t, to)
}

func TestReadPastTimeRange_FromNotBeforeToRejected(t *testing.T) {
	for _, args := range [][]string{
		{"--from-time", "1d", "--to-time", "7d"},
		{"--from-time", "2026-01-01T00:00:00Z", "--to-time", "2026-01-01T00:00:00Z"},
	} {
		_, _, err := ReadPastTimeRange(parsed(t, addTimeRangeFlags, args...))
		assertValidationError(t, err, "--from-time must be before --to-time")
	}
}

func TestIPAllowListFlag_SplitsOnCommas(t *testing.T) {
	got, err := ReadRepeatable(parsed(t, addAccountKeyCreateFlags, "--ip-allow-list", "203.0.113.0/24,2001:db8::1"), "ip-allow-list", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"203.0.113.0/24", "2001:db8::1"}, got)
}

func readIPAllowListUpdate(t *testing.T, args ...string) (*[]string, error) {
	t.Helper()
	return ReadListUpdate(parsed(t, addAccountKeyUpdateFlags, args...), "ip-allow-list", "clear-ip-allow-list")
}

func TestReadListUpdate_UnsetIsNil(t *testing.T) {
	got, err := readIPAllowListUpdate(t)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestReadListUpdate_ReturnsValues(t *testing.T) {
	got, err := readIPAllowListUpdate(t, "--ip-allow-list", "203.0.113.0/24")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, []string{"203.0.113.0/24"}, *got)
}

func TestReadListUpdate_ClearSendsEmptyList(t *testing.T) {
	got, err := readIPAllowListUpdate(t, "--clear-ip-allow-list")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, []string{}, *got)
}

func TestReadListUpdate_ClearFalseIsNil(t *testing.T) {
	got, err := readIPAllowListUpdate(t, "--clear-ip-allow-list=false")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestReadListUpdate_ValuesAndClearRejected(t *testing.T) {
	_, err := readIPAllowListUpdate(t, "--ip-allow-list", "203.0.113.0/24", "--clear-ip-allow-list")
	assertValidationError(t, err, "--ip-allow-list and --clear-ip-allow-list cannot be used together")
}

func TestReadListUpdate_EmptyValuePointsToClearFlag(t *testing.T) {
	_, err := readIPAllowListUpdate(t, "--ip-allow-list", "")
	assertValidationError(t, err, "--ip-allow-list cannot be empty; use --clear-ip-allow-list to clear it")
}

// API key readers -------------------------------------------------------------

func TestReadAPIKeyCreate_BuildsTheRequest(t *testing.T) {
	cmd := parsed(t, addAccountKeyCreateFlags, "--label", "CI", "--scope", "messages:send:all,domains:read",
		"--ip-allow-list", "203.0.113.0/24")

	req, err := ReadAPIKeyCreate(cmd, AccountKey)

	require.NoError(t, err)
	assert.Equal(t, requests.CreateAPIKeyRequest{
		Label:       "CI",
		Scopes:      []string{"messages:send:all", "domains:read"},
		IPAllowList: []string{"203.0.113.0/24"},
	}, req)
}

func TestReadAPIKeyCreate_Rejects(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{"missing label", []string{"--scope", "messages:send:all"}, "--label is required"},
		{"missing scope", []string{"--label", "CI"}, "--scope is required"},
		{"invalid scope", []string{"--label", "CI", "--scope", "bogus"}, "invalid --scope value \"bogus\": not an API key scope; `test --help` lists the scopes"},
		{"empty IP allow list entry", []string{"--label", "CI", "--scope", "messages:send:all", "--ip-allow-list", ""},
			"--ip-allow-list cannot be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadAPIKeyCreate(parsed(t, addAccountKeyCreateFlags, tt.args...), AccountKey)
			assertValidationError(t, err, tt.message)
		})
	}
}

func TestReadAPIKeyUpdate_HoldsOnlyTheFieldsGiven(t *testing.T) {
	req, err := ReadAPIKeyUpdate(parsed(t, addAccountKeyUpdateFlags, "--scope", "domains:read", "--clear-ip-allow-list"), AccountKey)

	require.NoError(t, err)
	assert.Nil(t, req.Label)
	require.NotNil(t, req.Scopes)
	assert.Equal(t, []string{"domains:read"}, *req.Scopes)
	require.NotNil(t, req.IPAllowList)
	assert.Empty(t, *req.IPAllowList)
}

func TestReadAPIKeyUpdate_Rejects(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{"no field", nil, "at least one of --label, --scope, --ip-allow-list or --clear-ip-allow-list must be provided"},
		{"empty label", []string{"--label", ""}, "--label cannot be empty"},
		{"invalid scope", []string{"--scope", "bogus"}, "invalid --scope value \"bogus\": not an API key scope; `test --help` lists the scopes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadAPIKeyUpdate(parsed(t, addAccountKeyUpdateFlags, tt.args...), AccountKey)
			assertValidationError(t, err, tt.message)
		})
	}
}

// Pagination ------------------------------------------------------------------

func TestReadPagination_DefaultsAreNil(t *testing.T) {
	limit, after, before, err := ReadPagination(parsed(t, AddPaginationFlags))
	require.NoError(t, err)
	assert.Nil(t, limit)
	assert.Nil(t, after)
	assert.Nil(t, before)
}

func TestReadPagination_ReturnsValues(t *testing.T) {
	limit, after, before, err := ReadPagination(parsed(t, AddPaginationFlags, "--limit", "25", "--after", "cur"))
	require.NoError(t, err)
	require.NotNil(t, limit)
	assert.Equal(t, int32(25), *limit)
	require.NotNil(t, after)
	assert.Equal(t, "cur", *after)
	assert.Nil(t, before)
}

func TestReadPagination_ReturnsBefore(t *testing.T) {
	_, _, before, err := ReadPagination(parsed(t, AddPaginationFlags, "--before", "prev"))
	require.NoError(t, err)
	require.NotNil(t, before)
	assert.Equal(t, "prev", *before)
}

func TestReadPagination_LimitOutOfRange(t *testing.T) {
	for _, limit := range []string{"--limit=-1", "--limit=101"} {
		t.Run(limit, func(t *testing.T) {
			_, _, _, err := ReadPagination(parsed(t, AddPaginationFlags, limit))
			assertValidationError(t, err, "--limit must be between 0 and 100")
		})
	}
}

func TestReadPagination_AfterAndBeforeRejected(t *testing.T) {
	_, _, _, err := ReadPagination(parsed(t, AddPaginationFlags, "--after", "a", "--before", "b"))
	assertValidationError(t, err, "--after and --before cannot be used together")
}

func TestReadPaginationParams_ReturnsTheSDKParams(t *testing.T) {
	params, err := ReadPaginationParams(parsed(t, AddPaginationFlags, "--limit", "25", "--after", "cur"))
	require.NoError(t, err)
	require.NotNil(t, params.Limit)
	assert.Equal(t, int32(25), *params.Limit)
	require.NotNil(t, params.After)
	assert.Equal(t, "cur", *params.After)
	assert.Nil(t, params.Before)
	assert.Nil(t, params.Cursor, "the API's v2 contract has no cursor parameter")
}

func TestLegacyCursorAliasSetsAfter(t *testing.T) {
	params, err := ReadPaginationParams(parsed(t, func(cmd *cobra.Command) {
		AddPaginationFlags(cmd)
		AddLegacyCursorAlias(cmd)
	}, "--cursor", "cur"))
	require.NoError(t, err)
	require.NotNil(t, params.After)
	assert.Equal(t, "cur", *params.After)
}

// Prompts ---------------------------------------------------------------------

func TestCheckCredentialFlags_RefusesAKeyThatIsNotPrintableASCII(t *testing.T) {
	addCredentialFlags := func(cmd *cobra.Command) {
		cmd.Flags().String("api-key", "", "")
		cmd.Flags().String("account-id", "", "")
	}
	for _, key := range []string{"aha-sk-\x1b[A", "aha-sk-\x00", "aha-sk-k\u00e9y", "aha-sk-\x7f"} {
		err := CheckCredentialFlags(parsed(t, addCredentialFlags, "--api-key", key, "--account-id", "11111111-1111-4111-8111-111111111111"))

		assertValidationError(t, err, "--api-key must be printable ASCII, without control or non-ASCII characters")
	}
	assert.NoError(t, CheckCredentialFlags(parsed(t, addCredentialFlags, "--api-key", "aha-sk-~ key!",
		"--account-id", "11111111-1111-4111-8111-111111111111")))
}

func TestAskIfEmpty_EndOfInputCancels(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	cmd, _, _ := ioCommand(t, "")

	_, err := AskIfEmpty(cmd, "", "Route name", "--name is required")

	assertValidationError(t, err, "operation cancelled")
	assert.Equal(t, "operation cancelled", err.Error())
}

// Idempotency -----------------------------------------------------------------

func TestIdempotencyOptions_UnsetPassesNoOption(t *testing.T) {
	opts, err := IdempotencyOptions(parsed(t, AddIdempotencyKeyFlag))
	require.NoError(t, err)
	assert.Empty(t, opts)
}

func TestIdempotencyOptions_SetsKey(t *testing.T) {
	opts, err := IdempotencyOptions(parsed(t, AddIdempotencyKeyFlag, "--idempotency-key", "my-key"))
	require.NoError(t, err)
	require.Len(t, opts, 1)
	config := api.RequestConfig{}
	opts[0](&config)
	assert.Equal(t, "my-key", config.Headers["Idempotency-Key"])
}

func TestIdempotencyOptions_AcceptsTheAPIsLongestKey(t *testing.T) {
	opts, err := IdempotencyOptions(parsed(t, AddIdempotencyKeyFlag, "--idempotency-key", strings.Repeat("k", 255)))
	require.NoError(t, err)
	assert.Len(t, opts, 1)
}

func TestIdempotencyOptions_RejectsALongerKey(t *testing.T) {
	_, err := IdempotencyOptions(parsed(t, AddIdempotencyKeyFlag, "--idempotency-key", strings.Repeat("k", 256)))
	assertValidationError(t, err, "--idempotency-key must be at most 255 characters")
}

func TestCheckIdempotencyKey_LeavesRoomForTheSuffixOfEveryRequest(t *testing.T) {
	key := strings.Repeat("k", 253)
	require.NoError(t, CheckIdempotencyKey(key, 10))
	assert.Equal(t, key+"-9", IdempotencyKeyFor(key, 9, 10))

	assertValidationError(t, CheckIdempotencyKey(key, 11),
		"--idempotency-key must be at most 252 characters for a send of 11 requests, since each request adds -N to it")
}

func TestIdempotencyKeyFor_SendsASingleRequestsKeyAsGiven(t *testing.T) {
	assert.Equal(t, "key", IdempotencyKeyFor("key", 0, 1))
	assert.Equal(t, "key-0", IdempotencyKeyFor("key", 0, 2))
}

// Batch files -----------------------------------------------------------------

type testDataItem struct {
	Email string `json:"email"`
	Score any    `json:"score,omitempty"`
}

func TestReadDataFile_ReturnsItems(t *testing.T) {
	items, err := readDataFile(testutil.WriteTempFile(t, "content", `{"data":[{"email":"a@example.com"},{"email":"b@example.com"}]}`), "items")
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestReadDataFile_RejectsBadShapes(t *testing.T) {
	tests := map[string]struct {
		content string
		message string
	}{
		"array envelope":     {`[]`, `batch file must be a JSON object, {"data": [...]}; CSV is not read`},
		"null envelope":      {`null`, `batch file must be a JSON object, {"data": [...]}; CSV is not read`},
		"CSV":                {"email,first_name\nperson@example.com,Ada\n", `batch file must be a JSON object, {"data": [...]}; CSV is not read`},
		"cut-off object":     {`{"data": [`, "batch file must be a JSON object"},
		"empty file":         {` `, "batch file is empty"},
		"missing data":       {`{"items":[]}`, "batch file data must be a JSON array"},
		"null data":          {`{"data":null}`, "batch file data must be a JSON array"},
		"object data":        {`{"data":{}}`, "batch file data must be a JSON array"},
		"extra envelope key": {`{"data":[{}],"extra":true}`, "batch file must contain only a data field"},
		"two objects":        {`{"data":[{}]} {"data":[{}]}`, "batch file must contain exactly one JSON object"},
		"no items":           {`{"data":[]}`, "batch file data cannot be empty"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := readDataFile(testutil.WriteTempFile(t, "content", tt.content), "items")
			assertValidationError(t, err, tt.message)
			assert.Equal(t, tt.message, err.Error(), "the message names no Go type")
		})
	}
}

// TestReadInputFile_DropsAByteOrderMark checks that a file saved with a UTF-8
// byte order mark, as Excel saves "CSV UTF-8", reads as its text alone.
func TestReadInputFile_DropsAByteOrderMark(t *testing.T) {
	data, err := ReadInputFile("file", testutil.WriteTempFile(t, "content", "\uFEFFemail\n"))
	require.NoError(t, err)
	assert.Equal(t, "email\n", string(data))

	items, err := readDataFile(testutil.WriteTempFile(t, "content", "\uFEFF{\"data\":[{}]}"), "items")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

// endlessReader is a file that grows without end while it is read, and
// counts the bytes read from it.
type endlessReader struct {
	read int
}

func (r *endlessReader) Read(p []byte) (int, error) {
	r.read += len(p)
	return len(p), nil
}

// TestReadAttachmentData_ReadsAtMostOneByteOverTheLimit checks that a file
// that grew after its size was checked is read only up to the limit and
// refused.
func TestReadAttachmentData_ReadsAtMostOneByteOverTheLimit(t *testing.T) {
	file := &endlessReader{}

	_, err := readAttachmentData("growing.log", file)

	assertValidationError(t, err, "--attach file growing.log is larger than 10,485,760 bytes (10 MiB)")
	assert.Equal(t, MaxAttachmentSize+1, file.read)
}

// TestReadAttachments_LimitsEachFileTo10MiB checks that a file of exactly
// 10,485,760 bytes is attached and one byte more is refused, with an error
// that gives the limit in bytes.
func TestReadAttachments_LimitsEachFileTo10MiB(t *testing.T) {
	dir := t.TempDir()
	atLimit := filepath.Join(dir, "at-limit.bin")
	overLimit := filepath.Join(dir, "over-limit.bin")
	for path, size := range map[string]int64{atLimit: 10_485_760, overLimit: 10_485_761} {
		require.NoError(t, os.WriteFile(path, nil, 0o600))
		require.NoError(t, os.Truncate(path, size))
	}

	attachments, err := ReadAttachments([]string{atLimit})
	require.NoError(t, err)
	assert.Len(t, attachments[0].Data, 10_485_760)

	_, err = ReadAttachments([]string{overLimit})
	assertValidationError(t, err, "--attach file "+overLimit+" is larger than 10,485,760 bytes (10 MiB)")
}

func TestReadAttachmentData_ReadsAFileAtTheLimit(t *testing.T) {
	data, err := readAttachmentData("full.bin", bytes.NewReader(make([]byte, MaxAttachmentSize)))

	require.NoError(t, err)
	assert.Len(t, data, MaxAttachmentSize)
}

// TestReadInputFile_RejectsTextThatIsNotUTF8 checks that a file in another
// encoding fails instead of being sent with its characters replaced.
func TestReadInputFile_RejectsTextThatIsNotUTF8(t *testing.T) {
	tests := map[string]struct {
		content string
		message string
	}{
		"Latin-1":            {"caf\xe9\n", "is not UTF-8 text; save it as UTF-8"},
		"UTF-8 BOM, Latin-1": {"\uFEFFcaf\xe9\n", "is not UTF-8 text; save it as UTF-8"},
		"UTF-16 LE":          {"\xff\xfee\x00m\x00", "is UTF-16 text, not UTF-8; save it as UTF-8"},
		"UTF-16 BE":          {"\xfe\xff\x00e\x00m", "is UTF-16 text, not UTF-8; save it as UTF-8"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			path := testutil.WriteTempFile(t, "content", tt.content)

			_, err := ReadInputFile("file", path)
			require.Error(t, err)
			assert.Equal(t, path+" "+tt.message, err.Error())
			assert.Equal(t, 4, errors.GetExitCode(err))

			_, err = readDataFile(path, "items")
			require.Error(t, err)
			assert.Equal(t, path+" "+tt.message, err.Error())
		})
	}
}

func TestReadInputFile_KeepsUTF8Text(t *testing.T) {
	data, err := ReadInputFile("file", testutil.WriteTempFile(t, "content", "café ✓\n"))
	require.NoError(t, err)
	assert.Equal(t, "café ✓\n", string(data))
}

func TestReadDataFile_RejectsMoreThanMaxItems(t *testing.T) {
	content := `{"data":[` + strings.Repeat(`{},`, MaxDataFileItems) + `{}]}`
	_, err := readDataFile(testutil.WriteTempFile(t, "content", content), "items")
	assertValidationError(t, err, "batch file data takes at most 1000 items, got 1001")
}

func TestReadDataFile_AcceptsMaxItems(t *testing.T) {
	content := `{"data":[` + strings.Repeat(`{},`, MaxDataFileItems-1) + `{}]}`
	items, err := readDataFile(testutil.WriteTempFile(t, "content", content), "items")
	require.NoError(t, err)
	assert.Len(t, items, MaxDataFileItems)
}

func TestReadDataFile_MissingFileRejected(t *testing.T) {
	_, err := readDataFile(filepath.Join(t.TempDir(), "missing.json"), "items")
	assertValidationError(t, err, "failed to read --file")
}

func TestReadDataItems_DecodesEachItem(t *testing.T) {
	items, err := ReadDataItems[testDataItem](testutil.WriteTempFile(t, "content", `{"data":[{"email":"a@example.com","score":7},{"email":"b@example.com"}]}`), "items", "item")
	require.NoError(t, err)
	assert.Equal(t, []testDataItem{{Email: "a@example.com", Score: json.Number("7")}, {Email: "b@example.com"}}, items)
}

func TestReadDataItems_FileErrorNamesItems(t *testing.T) {
	tooMany := `{"data":[` + strings.Repeat(`{},`, 1000) + `{}]}`
	_, err := ReadDataItems[testDataItem](testutil.WriteTempFile(t, "content", tooMany), "contacts", "contact")
	assertValidationError(t, err, "batch file data takes at most 1000 contacts, got 1001")
}

func TestReadDataItems_ItemErrorNamesItemAndPosition(t *testing.T) {
	_, err := ReadDataItems[testDataItem](testutil.WriteTempFile(t, "content", `{"data":[{"email":"a@example.com"},{"list_id":"x"}]}`), "contacts", "contact")
	assertValidationError(t, err, "batch contact at position 1 is invalid")
}

func TestDecodeDataItem_KeepsNumbers(t *testing.T) {
	var item testDataItem
	require.NoError(t, decodeDataItem([]byte(`{"email":"a@example.com","score":7}`), 0, "item", &item))
	assert.Equal(t, "a@example.com", item.Email)
	assert.Equal(t, json.Number("7"), item.Score)
}

func TestDecodeDataItem_RejectsUnknownField(t *testing.T) {
	var item testDataItem
	err := decodeDataItem([]byte(`{"email":"a@example.com","list_id":"x"}`), 3, "item", &item)
	assertValidationError(t, err, "batch item at position 3 is invalid")
}

func TestDecodeDataItem_RejectsNullField(t *testing.T) {
	var item testDataItem
	err := decodeDataItem([]byte(`{"email":"a@example.com","score":null}`), 2, "item", &item)
	assertValidationError(t, err, "batch item at position 2 sets score to null")
}

// TestDecodeJSON_NamesAFieldInAnArrayWithoutItsIndex checks that a field of
// the wrong type in an array is named by its keys alone: Go 1.27's decoder
// puts the array index in the path and Go 1.26's does not.
func TestDecodeJSON_NamesAFieldInAnArrayWithoutItsIndex(t *testing.T) {
	var target struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	err := DecodeJSON(strings.NewReader(`{"items":[{"name":"a"},{"name":5}]}`), &target, "--input", "JSON object")
	assertValidationError(t, err, "--input is invalid: items.name must be a string")
}

func TestDecodeDataItem_KeepsNestedNull(t *testing.T) {
	var item testDataItem
	require.NoError(t, decodeDataItem([]byte(`{"email":"a@example.com","score":{"old":null}}`), 0, "item", &item))
	assert.Equal(t, map[string]any{"old": nil}, item.Score)
}

// URLs ------------------------------------------------------------------------

func TestValidateWebhookURL_AcceptsHTTPAndHTTPS(t *testing.T) {
	for _, rawURL := range []string{"https://example.com/hook", "http://example.com/hook"} {
		t.Run(rawURL, func(t *testing.T) {
			assert.NoError(t, ValidateWebhookURL("url", rawURL))
		})
	}
}

func TestValidateWebhookURL_Rejects(t *testing.T) {
	for _, rawURL := range []string{"ftp://example.com", "example.com/hook", "https://", "://bad", "http://:8080/hook"} {
		t.Run(rawURL, func(t *testing.T) {
			assertValidationError(t, ValidateWebhookURL("url", rawURL), "--url must be an HTTP or HTTPS URL with a host")
		})
	}
}

// TestValidateWebhookURL_RejectsCredentials checks that a URL with a user
// name or password is refused, since commands such as webhooks get would
// show it.
func TestValidateWebhookURL_RejectsCredentials(t *testing.T) {
	for _, rawURL := range []string{"https://user:secret@example.com/hook", "https://user@example.com/hook"} {
		t.Run(rawURL, func(t *testing.T) {
			assertValidationError(t, ValidateWebhookURL("url", rawURL),
				"--url must have no user name or password, such as https://example.com/webhook")
		})
	}
}

func TestWarnIfInsecureURL_WarnsForRemoteHTTP(t *testing.T) {
	cmd, stdout, stderr := ioCommand(t, "")
	WarnIfInsecureURL(cmd, "url", "http://example.com/hook", WebhookSends)
	assert.Equal(t, "Warning: --url http://example.com/hook uses plain HTTP, so each event is sent unencrypted. Use HTTPS.\n", stderr.String())
	assert.Empty(t, stdout.String())
}

func TestWarnIfInsecureURL_SilentForHTTPSAndLocalHosts(t *testing.T) {
	for _, rawURL := range []string{"https://example.com/hook", "http://localhost:8080/hook", "http://127.0.0.1/hook", "http://[::1]:8080/hook", "http://127.0.0.2:3000/hook"} {
		t.Run(rawURL, func(t *testing.T) {
			cmd, _, stderr := ioCommand(t, "")
			WarnIfInsecureURL(cmd, "url", rawURL, WebhookSends)
			assert.Empty(t, stderr.String())
		})
	}
}

// Times -----------------------------------------------------------------------

func addTimeFlag(cmd *cobra.Command) {
	cmd.Flags().String("from-time", "", "")
}

func TestReadPastTime_EmptyIsNil(t *testing.T) {
	value, err := readPastTime(parsed(t, addTimeFlag), "from-time")
	require.NoError(t, err)
	assert.Nil(t, value)
}

func TestReadPastTime_ReturnsTime(t *testing.T) {
	value, err := readPastTime(parsed(t, addTimeFlag, "--from-time", "2024-01-02T03:04:05Z"), "from-time")
	require.NoError(t, err)
	require.NotNil(t, value)
	assert.Equal(t, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), *value)
}

func TestReadPastTime_BadValueRejected(t *testing.T) {
	_, err := readPastTime(parsed(t, addTimeFlag, "--from-time", "yesterday"), "from-time")
	assertValidationError(t, err, `invalid --from-time value "yesterday"`)
}

// Enums -----------------------------------------------------------------------

func addSandboxResultFlag(cmd *cobra.Command) {
	AddOneOfFlag(cmd, "sandbox-result", "deliver", "Result: %s", SandboxResults)
}

func TestMatchOneOf_ReturnsTheListedSpellingForAnyCase(t *testing.T) {
	cmd := parsed(t, addSandboxResultFlag)
	for _, value := range []string{"bounce", "BOUNCE", "Bounce"} {
		match, err := MatchOneOf(cmd, "sandbox-result", value)
		require.NoError(t, err, value)
		assert.Equal(t, "bounce", match, value)
	}
}

func TestMatchOneOf_RejectsOtherValues(t *testing.T) {
	cmd := parsed(t, addSandboxResultFlag)
	for _, value := range []string{"", "explode", "bounced"} {
		_, err := MatchOneOf(cmd, "sandbox-result", value)
		assertValidationError(t, err, "--sandbox-result must be one of: deliver, bounce, defer, fail, suppress")
	}
}

func TestReadOneOf_ReturnsTheListedSpelling(t *testing.T) {
	cmd := parsed(t, func(cmd *cobra.Command) { AddOneOfFlag(cmd, "scope", "", "Scope: %s", Scopes) }, "--scope", "SCOPED")
	value, err := ReadOneOf(cmd, "scope")
	require.NoError(t, err)
	assert.Equal(t, "scoped", *value)
}

func TestReadFilterOneOf_ReturnsTheListedSpelling(t *testing.T) {
	editors := []string{"advanced", "simple", "html"}
	cmd := parsed(t, func(cmd *cobra.Command) { AddOneOfFlag(cmd, "editor", "", "Editor: %s", editors) }, "--editor", "Simple")
	value, err := ReadFilterOneOf(cmd, "editor")
	require.NoError(t, err)
	assert.Equal(t, "simple", *value)
}

// TestOneOfReadersFailOnAFlagNotAddedByAddOneOfFlag checks that a reader
// given a flag the command does not have, or one without values, fails as a
// mistake in the CLI, exit code 1, rather than panicking or refusing every
// value, even when the flag is not set.
func TestOneOfReadersFailOnAFlagNotAddedByAddOneOfFlag(t *testing.T) {
	cmd := parsed(t, func(cmd *cobra.Command) { cmd.Flags().String("plain", "", "") })
	readers := map[string]func(name string) error{
		"MatchOneOf": func(name string) error { _, err := MatchOneOf(cmd, name, "x"); return err },
		"ReadOneOf":  func(name string) error { _, err := ReadOneOf(cmd, name); return err },
		"ReadFilterOneOf": func(name string) error {
			_, err := ReadFilterOneOf(cmd, name)
			return err
		},
	}
	for reader, read := range readers {
		for _, name := range []string{"missing", "plain"} {
			err := read(name)

			require.Error(t, err, "%s --%s", reader, name)
			assert.Equal(t, "internal error: test has no --"+name+" flag added by AddOneOfFlag", err.Error())
			assert.Equal(t, 1, errors.GetExitCode(err))
		}
	}
}

func TestAddOneOfFlag_ListsTheValuesInTheHelpAndCompletesThem(t *testing.T) {
	tests := map[string]struct {
		values []string
		help   string
	}{
		"one value":    {[]string{"a"}, "Pick: a (default a)"},
		"two values":   {[]string{"a", "b"}, "Pick: a or b (default a)"},
		"three values": {[]string{"a", "b", "c"}, "Pick: a, b or c (default a)"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			AddOneOfFlag(cmd, "pick", "a", "Pick: %s (default a)", test.values)

			assert.Equal(t, test.help, cmd.Flag("pick").Usage)
			complete, ok := cmd.GetFlagCompletionFunc("pick")
			require.True(t, ok)
			values, _ := complete(cmd, nil, "")
			assert.Equal(t, test.values, values)
		})
	}
}

// Flag conflicts --------------------------------------------------------------

func addConflictingFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "")
	cmd.Flags().Bool("clear-name", false, "")
	AddRepeatableFlag(cmd, "tag", "")
}

func TestRejectTogether_OneFlagIsAccepted(t *testing.T) {
	for _, args := range [][]string{{"--name", "a"}, {"--clear-name"}, {}} {
		assert.NoError(t, RejectTogether(parsed(t, addConflictingFlags, args...), "name", "clear-name"), args)
	}
}

// TestRejectTogether_SetFlagsAreRejectedWhateverTheirValue checks that a flag
// set to an empty value or to false still counts as given.
func TestRejectTogether_SetFlagsAreRejectedWhateverTheirValue(t *testing.T) {
	for _, args := range [][]string{
		{"--name", "a", "--clear-name"},
		{"--name=", "--clear-name"},
		{"--name", "a", "--clear-name=false"},
		{"--name=", "--clear-name=false"},
	} {
		assertValidationError(t, RejectTogether(parsed(t, addConflictingFlags, args...), "name", "clear-name"),
			"--name and --clear-name cannot be used together")
	}
	assertValidationError(t, RejectTogether(parsed(t, addConflictingFlags, "--tag=", "--clear-name=false"), "tag", "clear-name"),
		"--tag and --clear-name cannot be used together")
}

func TestUsageArgumentsNamesTheArgumentsButNotFlagValues(t *testing.T) {
	tests := map[string][]string{
		"list":                                   nil,
		"get <sub-account-id> <key-id>":          {"sub-account-id", "key-id"},
		"create [domain]":                        {"domain"},
		"cancel <message-id>...":                 {"message-id"},
		"create --label <label> --scope <scope>": nil,
		"add <list-id> --email <email>":          {"list-id"},
	}
	for use, want := range tests {
		t.Run(use, func(t *testing.T) {
			assert.Equal(t, want, UsageArguments(&cobra.Command{Use: use}))
		})
	}
}

func TestCheckMaxLength_CountsCharacters(t *testing.T) {
	atLimit, overLimit := strings.Repeat("é", 255), strings.Repeat("é", 256)
	assert.NoError(t, CheckMaxLength("name", &atLimit, MaxNameLength))
	assert.NoError(t, CheckMaxLength("name", nil, MaxNameLength))
	assertValidationError(t, CheckMaxLength("name", &overLimit, MaxNameLength), "--name must be at most 255 characters")
}

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"order-7":                     "order-7",
		"/home/pat/.ahasend/f-1.json": "/home/pat/.ahasend/f-1.json",
		"order 7":                     "'order 7'",
		"Bob's":                       `'Bob'\''s'`,
		"$HOME;rm":                    "'$HOME;rm'",
		"":                            "''",
		"=ls":                         "'=ls'",
		"a=b":                         "a=b",
		"~/f":                         "'~/f'",
	}
	for value, want := range tests {
		assert.Equal(t, want, ShellQuote(value), value)
	}
}

// TestQuotePath checks that a path is quoted for the system's shell: on
// Windows, whose cmd.exe keeps single quotes, in double quotes only when it
// holds a space, and elsewhere as ShellQuote quotes it.
func TestQuotePath(t *testing.T) {
	assert.Equal(t, `C:\Users\pat\.ahasend\failed-1.json`, quotePath("windows", `C:\Users\pat\.ahasend\failed-1.json`))
	assert.Equal(t, `C:\Users\Jörg_2\.ahasend\failed-1.json`, quotePath("windows", `C:\Users\Jörg_2\.ahasend\failed-1.json`))
	for _, user := range []string{"Pat Lee", "O'Brien", "R&D", "100%", "$pat", "a^b", "a(b)"} {
		path := `C:\Users\` + user + `\.ahasend\failed-1.json`
		assert.Equal(t, `"`+path+`"`, quotePath("windows", path))
	}
	assert.Equal(t, "'/home/Pat Lee/.ahasend/failed-1.json'", quotePath("linux", "/home/Pat Lee/.ahasend/failed-1.json"))
	assert.Equal(t, "/home/pat/.ahasend/failed-1.json", quotePath("darwin", "/home/pat/.ahasend/failed-1.json"))
}

// addAccountKeyCreateFlags adds the flags of a command that creates a key of
// the account.
func addAccountKeyCreateFlags(cmd *cobra.Command) { AddAPIKeyCreateFlags(cmd, AccountKey) }

// addAccountKeyUpdateFlags adds the flags of a command that updates a key of
// the account.
func addAccountKeyUpdateFlags(cmd *cobra.Command) { AddAPIKeyUpdateFlags(cmd, AccountKey) }
