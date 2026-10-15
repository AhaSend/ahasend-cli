package messages

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// rejectedEmail is the recipient the API does not accept in these tests.
const rejectedEmail = "blocked@example.com"

// rejectingResponse is the API's answer to a send to recipients: one item
// per recipient, in the order of the request, echoing the recipient as sent,
// "queued", except rejectedEmail, which gets status "error".
func rejectingResponse(recipients []common.Recipient) *responses.CreateMessageResponse {
	response := &responses.CreateMessageResponse{Object: "list"}
	for _, recipient := range recipients {
		item := responses.CreateSingleMessageResponse{Object: "message", Recipient: recipient, Status: "queued"}
		if recipient.Email == rejectedEmail {
			reason := "Recipient is suppressed"
			item.Status = "error"
			item.Error = &reason
		} else {
			id := "<id-" + recipient.Email + ">"
			item.ID = &id
		}
		response.Data = append(response.Data, item)
	}
	return response
}

// answerSend makes mockClient answer the inline send to exactly emails with
// rejectingResponse of the request's recipients.
func answerSend(mockClient *mocks.MockClient, emails []string) {
	sentTo := mock.MatchedBy(func(request requests.CreateMessageRequest) bool {
		if len(request.Recipients) != len(emails) {
			return false
		}
		for i, recipient := range request.Recipients {
			if recipient.Email != emails[i] {
				return false
			}
		}
		return true
	})
	response := &responses.CreateMessageResponse{}
	mockClient.On("SendMessageWithIdempotencyKey", sentTo, mock.Anything).
		Run(func(args mock.Arguments) {
			*response = *rejectingResponse(args.Get(0).(requests.CreateMessageRequest).Recipients)
		}).
		Return(response, nil)
}

// readFailedRecipientsFile returns the one failed recipients file under home.
func readFailedRecipientsFile(t *testing.T, home string) []map[string]any {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(home, ".ahasend", "failed-*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var entries []map[string]any
	require.NoError(t, json.Unmarshal(data, &entries))
	return entries
}

func TestSendRecipientTheAPIRejectsFailsTheSend(t *testing.T) {
	home := testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	answerSend(mockClient, []string{"ok@example.com", rejectedEmail})

	output, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
		"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi",
		"--to", "ok@example.com", "--to", rejectedEmail)

	require.Error(t, err)
	assert.Equal(t, "1 of 2 recipients failed", err.Error())
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	// The notice of the saved file comes before the result, so it is told
	// even when stdout cannot be written.
	notice, result, _ := strings.Cut(output, "\n")
	assert.True(t, strings.HasPrefix(notice, "Failed recipients saved to "), output)
	assert.True(t, strings.HasPrefix(result, "Message sent to 1 of 2 recipients\n"), output)
	assert.Contains(t, output, "Recipient: "+rejectedEmail+"\n  Status: error\n  Error: Recipient is suppressed\n")
	assert.Equal(t, []map[string]any{{
		"email":      rejectedEmail,
		"_error":     "Recipient is suppressed",
		"_retryable": false,
	}}, readFailedRecipientsFile(t, home))
}

func TestSendRecipientTheAPIRejectsKeepsJSONOutputTheAPIBody(t *testing.T) {
	testutil.UseNewHome(t)
	reason := "Recipient is suppressed"
	response := &responses.CreateMessageResponse{Object: "list", Data: []responses.CreateSingleMessageResponse{
		{Object: "message", Recipient: common.Recipient{Email: rejectedEmail}, Status: "error", Error: &reason},
	}}
	body := `{"object":"list","data":[{"object":"message","id":null,"recipient":{"email":"blocked@example.com","name":null},"status":"error","error":"Recipient is suppressed"}]}`
	apiresponse.Remember(response, []byte(body))
	mockClient := &mocks.MockClient{}
	mockClient.On("SendMessageWithIdempotencyKey", mock.Anything, mock.Anything).Return(response, nil)

	testutil.InjectClient(t, mockClient)

	output, stderr, err := testutil.RunCommand(t, NewSendCommand(), "json", "",
		"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi", "--to", rejectedEmail)

	require.Error(t, err)
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	assert.JSONEq(t, body, output)
	assert.Contains(t, stderr, "Failed recipients saved to ")
}

func TestSendBatchShowsRecipientsTheAPIRejects(t *testing.T) {
	home := testutil.UseNewHome(t)
	var rows strings.Builder
	rows.WriteString("email,first_name\n")
	emails := make([]string, 150)
	for i := range emails {
		emails[i] = fmt.Sprintf("user%d@example.com", i)
		if i == 120 {
			emails[i] = rejectedEmail
		}
		fmt.Fprintf(&rows, "%s,User%d\n", emails[i], i)
	}
	recipientsPath := testutil.WriteTempFile(t, "recipients.csv", rows.String())
	mockClient := &mocks.MockClient{}
	answerSend(mockClient, emails[:100])
	answerSend(mockClient, emails[100:])

	output, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", inlineSendArgs(recipientsPath)...)

	require.Error(t, err)
	assert.Equal(t, "1 of 150 recipients failed", err.Error())
	notice, result, _ := strings.Cut(output, "\n")
	assert.True(t, strings.HasPrefix(notice, "Failed recipients saved to "), output)
	assert.True(t, strings.HasPrefix(result, "Message sent to 149 of 150 recipients\nSucceeded: 149\nFailed: 1\n"), output)
	assert.Contains(t, output, "Item: recipients 1-100\n  Outcome: success\n")
	assert.Contains(t, output, "Item: recipients 101-150\n  Outcome: partial\n  Error: "+rejectedEmail+": Recipient is suppressed\n")
	entries := readFailedRecipientsFile(t, home)
	require.Len(t, entries, 1)
	assert.Equal(t, map[string]any{"first_name": "User120"}, entries[0]["substitution_data"])
}

func TestSendHeadingMatchesTheOutcome(t *testing.T) {
	tests := map[string]struct {
		to      []string
		heading string
	}{
		"all sent":  {[]string{"ok@example.com"}, "Message sent successfully"},
		"some sent": {[]string{"ok@example.com", rejectedEmail}, "Message sent to 1 of 2 recipients"},
		"none sent": {[]string{rejectedEmail}, "Message not sent to any recipient"},
	}
	for name, test := range tests {
		for _, format := range []string{"table", "plain"} {
			t.Run(name+"/"+format, func(t *testing.T) {
				testutil.UseNewHome(t)
				mockClient := &mocks.MockClient{}
				answerSend(mockClient, test.to)
				args := []string{"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi"}
				for _, email := range test.to {
					args = append(args, "--to", email)
				}
				testutil.InjectClient(t, mockClient)

				output, _, _ := testutil.RunCommand(t, NewSendCommand(), format, "", args...)

				assert.True(t, strings.HasPrefix(output, test.heading+"\n"), output)
			})
		}
	}
}

func TestSendTellsWhereTheFailedRecipientsAreOnStderr(t *testing.T) {
	home := testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	answerSend(mockClient, []string{rejectedEmail})
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "",
		"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi", "--to", rejectedEmail)

	require.Error(t, err)
	files, globErr := filepath.Glob(filepath.Join(home, ".ahasend", "failed-*.json"))
	require.NoError(t, globErr)
	require.Len(t, files, 1)
	assert.NotContains(t, stdout, files[0])
	assert.Contains(t, stderr, "Failed recipients saved to "+files[0]+"; send them again with --recipients "+cmdutil.QuotePath(files[0])+"\n")
}

func TestSendWithoutFailuresSavesAndTellsNothing(t *testing.T) {
	home := testutil.UseNewHome(t)
	mockClient := &mocks.MockClient{}
	answerSend(mockClient, []string{"ok@example.com"})
	testutil.InjectClient(t, mockClient)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "",
		"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi", "--to", "ok@example.com")

	require.NoError(t, err)
	assert.NotContains(t, stderr, "Failed recipients")
	assert.NoDirExists(t, filepath.Join(home, ".ahasend"))
}

// TestSendRejectsInvalidRecipientAddresses checks the address of each
// recipient, from --to, a CSV recipients file and a JSON one, before the
// command authenticates.
func TestSendRejectsInvalidRecipientAddresses(t *testing.T) {
	tests := map[string]struct {
		recipients []string
		message    string
	}{
		"--to": {[]string{"--to", "not-an-email"},
			`invalid --to value "not-an-email": not an email address`},
		"--to with a display name": {[]string{"--to", "Jane <jane@example.com>"},
			`invalid --to value "Jane <jane@example.com>": not an email address`},
		"CSV": {[]string{"--recipients", testutil.WriteTempFile(t, "recipients.csv", "email,name\nok@example.com,Ok\nnot-an-email,Bad\n")},
			`--recipients row 3: "not-an-email" is not an email address`},
		"JSON": {[]string{"--recipients", testutil.WriteTempFile(t, "recipients.json", `[{"email":"ok@example.com"},{"email":"not-an-email"}]`)},
			`--recipients index 1: "not-an-email" is not an email address`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "",
				append([]string{"--from", "sender@example.com", "--subject", "Hi", "--text", "Hi"}, tt.recipients...)...)

			require.Error(t, err)
			assert.Equal(t, tt.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestSendSavesEachRejectedEntryOfOneAddressWithItsOwnSubstitutions checks
// that two entries of one address the API rejects keep their own
// substitutions in the failed recipients file: the API answers in the order
// of the request.
func TestSendSavesEachRejectedEntryOfOneAddressWithItsOwnSubstitutions(t *testing.T) {
	home := testutil.UseNewHome(t)
	recipientsPath := testutil.WriteTempFile(t, "recipients.csv",
		"email,first_name\n"+rejectedEmail+",Pat\n"+rejectedEmail+",Sam\nok@example.com,Lee\n")
	mockClient := &mocks.MockClient{}
	answerSend(mockClient, []string{rejectedEmail, rejectedEmail, "ok@example.com"})

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", inlineSendArgs(recipientsPath)...)

	require.Error(t, err)
	entries := readFailedRecipientsFile(t, home)
	require.Len(t, entries, 2)
	assert.Equal(t, map[string]any{"first_name": "Pat"}, entries[0]["substitution_data"])
	assert.Equal(t, map[string]any{"first_name": "Sam"}, entries[1]["substitution_data"])
}
