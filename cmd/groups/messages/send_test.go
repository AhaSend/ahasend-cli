package messages

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/batch"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/testenv"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{
			name:    "valid email",
			email:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			email:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "valid email with plus",
			email:   "user+tag@example.com",
			wantErr: false,
		},
		{
			name:    "empty email",
			email:   "",
			wantErr: true,
		},
		{
			name:    "invalid format - no @",
			email:   "userexample.com",
			wantErr: true,
		},
		{
			name:    "invalid format - no domain",
			email:   "user@",
			wantErr: true,
		},
		{
			name:    "valid format - no TLD, which the API judges",
			email:   "user@example",
			wantErr: false,
		},
		{
			name:    "invalid format - multiple @",
			email:   "user@@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateEmail("--to", tt.email)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Template mode -----------------------------------------------------------------

const sendTemplateID = "77777777-7777-4777-8777-777777777777"

// templateSendCall is one SendTemplateMessageWithIdempotencyKey call.
type templateSendCall struct {
	request requests.CreateTemplateMessageRequest
	key     string
}

// expectTemplateSends records every template send the mock receives and
// answers each with a queued message.
func expectTemplateSends(mockClient *mocks.MockClient) func() []templateSendCall {
	var mu sync.Mutex
	var calls []templateSendCall
	mockClient.On("SendTemplateMessageWithIdempotencyKey",
		mock.AnythingOfType("requests.CreateTemplateMessageRequest"), mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, templateSendCall{
				request: args.Get(0).(requests.CreateTemplateMessageRequest),
				key:     args.String(1),
			})
		}).
		Return(mocks.NewMessageResponse("msg-1"), nil)
	return func() []templateSendCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]templateSendCall(nil), calls...)
	}
}

// withEmptyStdin points os.Stdin at an empty pipe, so any prompt fails.
func withEmptyStdin(t *testing.T) {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	original := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = original
		_ = reader.Close()
	})
}

func TestTemplateSendRejectedBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--template-id", "not-a-uuid"}, `invalid template ID value "not-a-uuid": not a UUID`},
		{[]string{"--template-id", ""}, "template ID cannot be empty"},
	}
	for _, flag := range []string{"text", "html", "amp", "text-file", "html-file", "amp-file"} {
		tests = append(tests, struct {
			args    []string
			message string
		}{
			[]string{"--template-id", sendTemplateID, "--" + flag, "x"},
			fmt.Sprintf("--template-id and --%s cannot be used together", flag),
		})
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunCommandCombined(t, NewSendCommand(), "plain", append(test.args, "--to", "a@example.com")...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

func TestTemplateSendPromptsForNothingAndLeavesSenderAndSubjectToTemplate(t *testing.T) {
	withEmptyStdin(t)
	mockClient := &mocks.MockClient{}
	calls := expectTemplateSends(mockClient)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
		"--template-id", sendTemplateID, "--to", "a@example.com")
	require.NoError(t, err)
	require.Len(t, calls(), 1)
	request := calls()[0].request
	assert.Equal(t, uuid.MustParse(sendTemplateID), request.TemplateID)
	assert.Nil(t, request.From)
	assert.Equal(t, "", request.Subject)
	assert.Equal(t, []common.Recipient{{Email: "a@example.com"}}, request.Recipients)
}

func TestTemplateSendRejectsInvalidFromAsInlineSend(t *testing.T) {
	expected := validation.ValidateEmail("--from", "not-an-email").Error()

	mockClient := &mocks.MockClient{}
	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
		"--template-id", sendTemplateID, "--from", "not-an-email", "--to", "a@example.com")
	require.Error(t, err)
	assert.Equal(t, expected, err.Error())

	_, err = testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
		"--from", "not-an-email", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi")
	require.Error(t, err)
	assert.Equal(t, expected, err.Error())
	mockClient.AssertNotCalled(t, "SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything)
	mockClient.AssertNotCalled(t, "SendMessageWithIdempotencyKey", mock.Anything, mock.Anything)
}

func TestTemplateSendMergesGlobalSubstitutionsWithRecipientPrecedence(t *testing.T) {
	recipientsPath := testutil.WriteTempFile(t, "recipients.json", `[
		{"email": "a@example.com", "substitution_data": {"first_name": "Pat", "plan": "pro"}},
		{"email": "b@example.com"}
	]`)
	globalsPath := testutil.WriteTempFile(t, "globals.json", `{"plan": "free", "company": "Acme"}`)
	mockClient := &mocks.MockClient{}
	calls := expectTemplateSends(mockClient)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", "--template-id", sendTemplateID,
		"--recipients", recipientsPath, "--global-substitutions", globalsPath)
	require.NoError(t, err)
	require.Len(t, calls(), 1)
	assert.Equal(t, []common.Recipient{
		{Email: "a@example.com", Substitutions: map[string]interface{}{"first_name": "Pat", "plan": "pro", "company": "Acme"}},
		{Email: "b@example.com", Substitutions: map[string]interface{}{"plan": "free", "company": "Acme"}},
	}, calls()[0].request.Recipients)
}

func TestTemplateSendSplitsRecipientsIntoJobsWithOwnKeys(t *testing.T) {
	var rows strings.Builder
	rows.WriteString("email,first_name\n")
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&rows, "user%d@example.com,User%d\n", i, i)
	}
	recipientsPath := testutil.WriteTempFile(t, "recipients.csv", rows.String())
	globalsPath := testutil.WriteTempFile(t, "globals.json", `{"company": "Acme", "first_name": "Friend"}`)
	mockClient := &mocks.MockClient{}
	calls := expectTemplateSends(mockClient)

	output, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", "--template-id", sendTemplateID,
		"--recipients", recipientsPath, "--global-substitutions", globalsPath, "--idempotency-key", "base")
	require.NoError(t, err)
	assert.Contains(t, output, "Message sent successfully\nSucceeded: 250\nFailed: 0\n")
	assert.Contains(t, output, "Item: recipients 201-250\n  Outcome: success\n")

	sent := calls()
	require.Len(t, sent, 3)
	// Batches may be sent concurrently, so order the calls by key.
	sort.Slice(sent, func(a, b int) bool { return sent[a].key < sent[b].key })
	for i, size := range []int{100, 100, 50} {
		assert.Equal(t, fmt.Sprintf("base-%d", i), sent[i].key)
		require.Len(t, sent[i].request.Recipients, size)
		first := sent[i].request.Recipients[0]
		assert.Equal(t, fmt.Sprintf("user%d@example.com", i*100), first.Email)
		assert.Equal(t, map[string]interface{}{"company": "Acme", "first_name": fmt.Sprintf("User%d", i*100)}, first.Substitutions)
	}
}

func TestTemplateSendFailureSummaryCountsRecipients(t *testing.T) {
	var rows strings.Builder
	rows.WriteString("email\n")
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&rows, "user%d@example.com\n", i)
	}
	recipientsPath := testutil.WriteTempFile(t, "recipients.csv", rows.String())
	refused := &api.APIError{StatusCode: 400, Message: "bad request", Raw: []byte(`{"message":"bad request"}`)}

	tests := []struct {
		name    string
		mock    func(*mocks.MockClient)
		message string
	}{
		{
			name: "all failed",
			mock: func(m *mocks.MockClient) {
				m.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything).Return(nil, refused)
			},
			message: "150 of 150 recipients failed",
		},
		{
			name: "partial",
			mock: func(m *mocks.MockClient) {
				m.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything).
					Return(mocks.NewMessageResponse("msg-1"), nil).Once()
				m.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything).Return(nil, refused)
			},
			message: "50 of 150 recipients failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The batch writes the failed recipients file under HOME.
			testenv.SetHome(t, t.TempDir())
			mockClient := &mocks.MockClient{}
			test.mock(mockClient)

			_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", "--template-id", sendTemplateID,
				"--recipients", recipientsPath, "--max-retries", "0")
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
		})
	}
}

func TestTemplateSendCarriesInlineSendOptions(t *testing.T) {
	mockClient := &mocks.MockClient{}
	calls := expectTemplateSends(mockClient)
	firstAttempt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", "--template-id", sendTemplateID,
		"--from", "sender@example.com", "--subject", "Override", "--to", "a@example.com",
		"--sandbox", "--tags", "welcome", "--header", "X-Test: 1", "--schedule", firstAttempt.Format(time.RFC3339),
		"--track-opens=false", "--idempotency-key", "key")
	require.NoError(t, err)
	require.Len(t, calls(), 1)

	trackOpens, sandbox, sandboxResult := false, true, "deliver"
	assert.Equal(t, requests.CreateTemplateMessageRequest{
		TemplateID:    uuid.MustParse(sendTemplateID),
		From:          &common.SenderAddress{Email: "sender@example.com"},
		Recipients:    []common.Recipient{{Email: "a@example.com"}},
		Subject:       "Override",
		Headers:       map[string]string{"X-Test": "1"},
		Tags:          []string{"welcome"},
		Sandbox:       &sandbox,
		SandboxResult: &sandboxResult,
		Tracking:      &common.Tracking{Open: &trackOpens},
		Schedule:      &common.MessageSchedule{FirstAttempt: &firstAttempt},
	}, calls()[0].request)
	assert.Equal(t, "key", calls()[0].key)
}

// TestSendSendsOnlyTheTrackingFlagsGiven checks that each tracking setting is
// sent only when its flag is set, so --track-opens=false turns open tracking
// off and no flag leaves both to the account's settings.
func TestSendSendsOnlyTheTrackingFlagsGiven(t *testing.T) {
	off, on := false, true
	tests := map[string]struct {
		args []string
		want *common.Tracking
	}{
		"no tracking flags":   {nil, nil},
		"--track-opens=false": {[]string{"--track-opens=false"}, &common.Tracking{Open: &off}},
		"--track-clicks":      {[]string{"--track-clicks"}, &common.Tracking{Click: &on}},
		"both off": {[]string{"--track-opens=false", "--track-clicks=false"},
			&common.Tracking{Open: &off, Click: &off}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			var sent requests.CreateMessageRequest
			mockClient.On("SendMessageWithIdempotencyKey",
				mock.AnythingOfType("requests.CreateMessageRequest"), mock.AnythingOfType("string")).
				Run(func(args mock.Arguments) { sent = args.Get(0).(requests.CreateMessageRequest) }).
				Return(mocks.NewMessageResponse("msg-1"), nil)

			args := append([]string{"--from", "sender@example.com", "--to", "a@example.com",
				"--subject", "Hi", "--text", "Hi"}, test.args...)
			_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", args...)
			require.NoError(t, err)
			assert.Equal(t, test.want, sent.Tracking)
		})
	}
}

// Recipients files ----------------------------------------------------------------

// inlineSendCall is one SendMessageWithIdempotencyKey call.
type inlineSendCall struct {
	request requests.CreateMessageRequest
	key     string
}

// expectInlineSends records every inline send the mock receives and answers
// each with err, or with a queued message when err is nil.
func expectInlineSends(mockClient *mocks.MockClient, err error) func() []inlineSendCall {
	var mu sync.Mutex
	var calls []inlineSendCall
	call := mockClient.On("SendMessageWithIdempotencyKey",
		mock.AnythingOfType("requests.CreateMessageRequest"), mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, inlineSendCall{
				request: args.Get(0).(requests.CreateMessageRequest),
				key:     args.String(1),
			})
		})
	if err != nil {
		call.Return(nil, err)
	} else {
		call.Return(mocks.NewMessageResponse("msg-1"), nil)
	}
	return func() []inlineSendCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]inlineSendCall(nil), calls...)
	}
}

// inlineSendArgs are the flags of an inline send to the recipients in path.
func inlineSendArgs(path string) []string {
	return []string{"--from", "sender@example.com", "--subject", "Hi {{first_name}}", "--text", "Hi",
		"--recipients", path, "--max-retries", "0"}
}

func TestSendSplitsRecipientsIntoJobsWithOwnKeys(t *testing.T) {
	var rows strings.Builder
	rows.WriteString("email\n")
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&rows, "user%d@example.com\n", i)
	}
	recipientsPath := testutil.WriteTempFile(t, "recipients.csv", rows.String())
	mockClient := &mocks.MockClient{}
	sends := expectInlineSends(mockClient, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
		append(inlineSendArgs(recipientsPath), "--idempotency-key", "base")...)
	require.NoError(t, err)

	sent := sends()
	require.Len(t, sent, 2)
	// Batches may be sent concurrently, so order the calls by key.
	sort.Slice(sent, func(a, b int) bool { return sent[a].key < sent[b].key })
	for i, size := range []int{100, 50} {
		assert.Equal(t, fmt.Sprintf("base-%d", i), sent[i].key)
		require.Len(t, sent[i].request.Recipients, size)
		assert.Equal(t, fmt.Sprintf("user%d@example.com", i*100), sent[i].request.Recipients[0].Email)
		assert.Equal(t, "Hi {{first_name}}", sent[i].request.Subject)
	}
}

func TestSendCarriesInlineSendOptions(t *testing.T) {
	htmlPath := testutil.WriteTempFile(t, "body.html", "<p>Hi</p>")
	globalsPath := testutil.WriteTempFile(t, "globals.json", `{"company": "Acme"}`)
	mockClient := &mocks.MockClient{}
	sends := expectInlineSends(mockClient, nil)
	firstAttempt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
		"--from", "sender@example.com", "--subject", "Hello", "--to", "a@example.com",
		"--text", "Hi", "--html-file", htmlPath, "--global-substitutions", globalsPath,
		"--sandbox", "--sandbox-result", "bounce", "--tags", "welcome", "--header", "X-Test: 1",
		"--schedule", firstAttempt.Format(time.RFC3339), "--track-clicks", "--idempotency-key", "key")
	require.NoError(t, err)
	require.Len(t, sends(), 1)

	text, html, trackClicks, sandbox, sandboxResult := "Hi", "<p>Hi</p>", true, true, "bounce"
	assert.Equal(t, requests.CreateMessageRequest{
		From:          common.SenderAddress{Email: "sender@example.com"},
		Recipients:    []common.Recipient{{Email: "a@example.com"}},
		Subject:       "Hello",
		TextContent:   &text,
		HtmlContent:   &html,
		Substitutions: map[string]interface{}{"company": "Acme"},
		Headers:       map[string]string{"X-Test": "1"},
		Tags:          []string{"welcome"},
		Sandbox:       &sandbox,
		SandboxResult: &sandboxResult,
		Tracking:      &common.Tracking{Click: &trackClicks},
		Schedule:      &common.MessageSchedule{FirstAttempt: &firstAttempt},
	}, sends()[0].request)
	assert.Equal(t, "key", sends()[0].key)
}

func TestSendJSONRecipientsCarrySubstitutionData(t *testing.T) {
	recipientsPath := testutil.WriteTempFile(t, "recipients.json", `[
		{"email": "a@example.com", "name": "Pat", "substitution_data": {"first_name": "Pat", "order": 12345678901234567890}}
	]`)
	mockClient := &mocks.MockClient{}
	sends := expectInlineSends(mockClient, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", inlineSendArgs(recipientsPath)...)
	require.NoError(t, err)
	require.Len(t, sends(), 1)
	name := "Pat"
	assert.Equal(t, []common.Recipient{{
		Email:         "a@example.com",
		Name:          &name,
		Substitutions: map[string]interface{}{"first_name": "Pat", "order": json.Number("12345678901234567890")},
	}}, sends()[0].request.Recipients)
}

func TestSendGlobalSubstitutionsKeepNumbersAsWritten(t *testing.T) {
	globalsPath := testutil.WriteTempFile(t, "globals.json", `{"account": 12345678901234567890, "rate": 1.50}`)
	mockClient := &mocks.MockClient{}
	sends := expectInlineSends(mockClient, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
		"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
		"--global-substitutions", globalsPath)
	require.NoError(t, err)
	require.Len(t, sends(), 1)
	assert.Equal(t, map[string]interface{}{"account": json.Number("12345678901234567890"), "rate": json.Number("1.50")},
		sends()[0].request.Substitutions)
}

func TestSendJSONRecipientsStillReadSubstitutionsKey(t *testing.T) {
	recipientsPath := testutil.WriteTempFile(t, "recipients.json", `[
		{"email": "a@example.com", "substitutions": {"first_name": "Pat"}}
	]`)
	mockClient := &mocks.MockClient{}
	sends := expectInlineSends(mockClient, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", inlineSendArgs(recipientsPath)...)
	require.NoError(t, err)
	require.Len(t, sends(), 1)
	assert.Equal(t, []common.Recipient{{
		Email:         "a@example.com",
		Substitutions: map[string]interface{}{"first_name": "Pat"},
	}}, sends()[0].request.Recipients)
}

func TestSendJSONRecipientsRejectBothSubstitutionKeys(t *testing.T) {
	recipientsPath := testutil.WriteTempFile(t, "recipients.json", `[
		{"email": "a@example.com", "substitution_data": {"a": 1}, "substitutions": {"b": 2}}
	]`)
	testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewSendCommand(), inlineSendArgs(recipientsPath)...)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sets both substitution_data and substitutions")
}

func TestSendFailedRecipientsFileCanBeSentAgain(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	recipientsPath := testutil.WriteTempFile(t, "recipients.json", `[
		{"email": "a@example.com", "name": "Pat", "substitution_data": {"first_name": "Pat"}},
		{"email": "b@example.com", "substitution_data": {"first_name": "Sam"}}
	]`)
	refused := &api.APIError{StatusCode: 400, Message: "bad request", Raw: []byte(`{"message":"bad request"}`)}
	failingClient := &mocks.MockClient{}
	firstSends := expectInlineSends(failingClient, refused)

	_, err := testutil.RunWithClient(t, failingClient, NewSendCommand(), "plain", inlineSendArgs(recipientsPath)...)
	require.Error(t, err)
	require.Len(t, firstSends(), 1)
	failedFiles, err := filepath.Glob(filepath.Join(home, ".ahasend", "failed-*.json"))
	require.NoError(t, err)
	require.Len(t, failedFiles, 1)

	retryClient := &mocks.MockClient{}
	retrySends := expectInlineSends(retryClient, nil)
	_, err = testutil.RunWithClient(t, retryClient, NewSendCommand(), "plain", inlineSendArgs(failedFiles[0])...)
	require.NoError(t, err)
	require.Len(t, retrySends(), 1)
	assert.Equal(t, firstSends()[0].request.Recipients, retrySends()[0].request.Recipients)
	assert.Equal(t, map[string]interface{}{"first_name": "Sam"}, retrySends()[0].request.Recipients[1].Substitutions)
}

// TestTemplateSendFailedRecipientsFileTakesNewGlobalSubstitutions checks
// that a template send saves the recipients as read, without the global
// substitutions it merged into them, so a corrected --global-substitutions
// applies when the file is sent again.
func TestTemplateSendFailedRecipientsFileTakesNewGlobalSubstitutions(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	recipientsPath := testutil.WriteTempFile(t, "recipients.json",
		`[{"email": "a@example.com", "substitution_data": {"first_name": "Pat"}}]`)
	refused := &api.APIError{StatusCode: 400, Message: "bad request", Raw: []byte(`{"message":"bad request"}`)}
	failingClient := &mocks.MockClient{}
	failingClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything).Return(nil, refused)

	_, err := testutil.RunWithClient(t, failingClient, NewSendCommand(), "plain",
		"--template-id", sendTemplateID, "--recipients", recipientsPath, "--max-retries", "0",
		"--global-substitutions", testutil.WriteTempFile(t, "old.json", `{"offer": "old"}`))
	require.Error(t, err)
	failedFiles, err := filepath.Glob(filepath.Join(home, ".ahasend", "failed-*.json"))
	require.NoError(t, err)
	require.Len(t, failedFiles, 1)

	retryClient := &mocks.MockClient{}
	retrySends := expectTemplateSends(retryClient)
	_, err = testutil.RunWithClient(t, retryClient, NewSendCommand(), "plain",
		"--template-id", sendTemplateID, "--recipients", failedFiles[0],
		"--global-substitutions", testutil.WriteTempFile(t, "new.json", `{"offer": "new"}`))
	require.NoError(t, err)
	require.Len(t, retrySends(), 1)
	assert.Equal(t, map[string]interface{}{"first_name": "Pat", "offer": "new"},
		retrySends()[0].request.Recipients[0].Substitutions)
}

// Batch options -------------------------------------------------------------------

func TestSendRejectsBadOptionsBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--max-concurrency", "0"}, "--max-concurrency must be between 1 and 10"},
		{[]string{"--max-concurrency", "-1"}, "--max-concurrency must be between 1 and 10"},
		{[]string{"--max-concurrency", "11"}, "--max-concurrency must be between 1 and 10"},
		{[]string{"--max-retries", "-1"}, "--max-retries must be 0 or more"},
		{[]string{"--header", "X-Test"}, "--header must be Name: value"},
		{[]string{"--header", ": value"}, "--header must be Name: value"},
		// The API refuses a Reply-To header with a reply-to
		{[]string{"--reply-to", "r@example.com", "--header", "reply-to: x@example.com"},
			"--header reply-to and --reply-to cannot be used together"},
		{[]string{"--header", "X-Test: 1", "--header", "x-test: 2"}, "--header gives the header x-test twice; header names ignore case"},
		{[]string{"--attach", "inv,2026.txt"}, `invalid --attach value "inv": no such file; a file name with a comma must be in double quotes: --attach '"a,b.pdf"'`},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			_, err := testutil.RunCommandCombined(t, NewSendCommand(), "plain", append([]string{
				"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
			}, test.args...)...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

// TestSendRefusesHeadersTheAPIOverridesOrDrops checks that --header refuses,
// in any case and before authenticating, each header the API writes over its
// own or drops, naming the flag that sets it, on inline, template and
// recipients-file sends.
func TestSendRefusesHeadersTheAPIOverridesOrDrops(t *testing.T) {
	recipients := testutil.WriteTempFile(t, "recipients.json", `[{"email": "a@example.com"}]`)
	sends := map[string][]string{
		"inline":     {"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi"},
		"template":   {"--template-id", sendTemplateID, "--to", "a@example.com"},
		"recipients": {"--from", "sender@example.com", "--recipients", recipients, "--subject", "Hi", "--text", "Hi"},
	}
	tests := []struct {
		header  string
		message string
	}{
		// Headers the API writes over the ones it builds
		{"From: x@example.com", "--header From cannot be used; set it with --from"},
		{"sender: x@example.com", "--header sender cannot be used; the sender is --from"},
		{"TO: x@example.com", "--header TO cannot be used; set it with --to or --recipients"},
		{"Cc: x@example.com", "--header Cc cannot be used; the API sends only to --to and --recipients; give the address there"},
		{"bcc: x@example.com", "--header bcc cannot be used; the API sends only to --to and --recipients; give the address there"},
		{"Subject: other", "--header Subject cannot be used; set it with --subject"},
		// AhaSend headers the API drops and sets from the flags
		{"ahasend-sandbox: true", "--header ahasend-sandbox cannot be used; the API drops it; set it with --sandbox"},
		{"AhaSend-Sandbox-Result: bounce", "--header AhaSend-Sandbox-Result cannot be used; the API drops it; set it with --sandbox-result"},
		{"AhaSend-Tags: a", "--header AhaSend-Tags cannot be used; the API drops it; set it with --tag"},
		{"AhaSend-Track-Opens: true", "--header AhaSend-Track-Opens cannot be used; the API drops it; set it with --track-opens"},
		{"AhaSend-Track-Clicks: true", "--header AhaSend-Track-Clicks cannot be used; the API drops it; set it with --track-clicks"},
		{"AhaSend-Schedule: 2030-01-01T00:00:00Z", "--header AhaSend-Schedule cannot be used; the API drops it; set it with --schedule"},
		{"X-Schedule: 2030-01-01T00:00:00Z", "--header X-Schedule cannot be used; the API drops it; set it with --schedule"},
		{"AhaSend-Data-Retention: 1", "--header AhaSend-Data-Retention cannot be used; the API drops it"},
		{"AhaSend-Metadata-Retention: 1", "--header AhaSend-Metadata-Retention cannot be used; the API drops it"},
		{"Message-ID: <a@example.com>", "--header Message-ID cannot be used; the API drops it and sets its own"},
		// Headers AhaSend routes mail by
		{"X-Campaign: 1", "--header X-Campaign cannot be used; the API drops it; AhaSend routes mail by it"},
		{"X-CUSTOMER-ID: 1", "--header X-CUSTOMER-ID cannot be used; the API drops it; AhaSend routes mail by it"},
		{"X-Schedule-ID: 1", "--header X-Schedule-ID cannot be used; the API drops it; AhaSend routes mail by it"},
		{"X-Subscriber-ID: 1", "--header X-Subscriber-ID cannot be used; the API drops it; AhaSend routes mail by it"},
	}
	for name, args := range sends {
		for _, test := range tests {
			t.Run(name+"/"+test.header, func(t *testing.T) {
				authCalled := testutil.RefuseAuth(t)

				_, err := testutil.RunCommandCombined(t, NewSendCommand(), "plain", append(args, "--header", test.header)...)
				require.Error(t, err)
				assert.Equal(t, test.message, err.Error())
				assert.Equal(t, 4, clierrors.GetExitCode(err))
				assert.False(t, *authCalled)
			})
		}
	}
}

// TestSendTakesAReplyToHeaderWithoutReplyTo checks that --header may set
// Reply-To when --reply-to is not given.
func TestSendTakesAReplyToHeaderWithoutReplyTo(t *testing.T) {
	mockClient := &mocks.MockClient{}
	sends := expectInlineSends(mockClient, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
		"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
		"--header", "Reply-To: r@example.com")
	require.NoError(t, err)
	require.Len(t, sends(), 1)
	assert.Equal(t, map[string]string{"Reply-To": "r@example.com"}, sends()[0].request.Headers)
}

func TestSendAcceptsBatchOptionsAtTheirLimits(t *testing.T) {
	for _, args := range [][]string{
		{"--max-concurrency", "1", "--max-retries", "0"},
		{"--max-concurrency", "10"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			sends := expectInlineSends(mockClient, nil)

			_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", append([]string{
				"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
			}, args...)...)
			require.NoError(t, err)
			assert.Len(t, sends(), 1)
		})
	}
}

// idempotencyKeyCharactersError is the error of an idempotency key with a
// character that is not printable ASCII.
const idempotencyKeyCharactersError = "--idempotency-key must be printable ASCII: letters, digits, punctuation and spaces, " +
	"without control or non-ASCII characters"

func TestSendRejectsAnIdempotencyKeyLongerThanTheAPIAllows(t *testing.T) {
	var rows strings.Builder
	rows.WriteString("email\n")
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&rows, "user%d@example.com\n", i)
	}
	recipientsPath := testutil.WriteTempFile(t, "recipients.csv", rows.String())
	tests := map[string]struct {
		args []string
		want string
	}{
		"one request": {
			[]string{"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
				"--idempotency-key", strings.Repeat("k", 256)},
			"--idempotency-key must be at most 255 characters",
		},
		"two requests": {
			append(inlineSendArgs(recipientsPath), "--idempotency-key", strings.Repeat("k", 254)),
			"--idempotency-key must be at most 253 characters for a send of 2 requests, since each request adds -N to it",
		},
		// An HTTP header cannot carry them as they are
		"a control character": {
			[]string{"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
				"--idempotency-key", "order\n7"},
			idempotencyKeyCharactersError,
		},
		"a non-ASCII character": {
			[]string{"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
				"--idempotency-key", "commande-é"},
			idempotencyKeyCharactersError,
		},
		// The HTTP client drops the spaces around a header value, so the API
		// would get no key, or another key than the notices name
		"a blank key": {
			[]string{"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
				"--idempotency-key", "   "},
			"--idempotency-key cannot be empty",
		},
		"an empty key": {
			[]string{"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
				"--idempotency-key", ""},
			"--idempotency-key cannot be empty",
		},
		"spaces around the key": {
			[]string{"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
				"--idempotency-key", " k1 "},
			"--idempotency-key cannot start or end with a space, which the HTTP header would drop",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}

			_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", test.args...)

			require.Error(t, err)
			assert.Equal(t, test.want, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			mockClient.AssertNotCalled(t, "SendMessageWithIdempotencyKey", mock.Anything, mock.Anything)
		})
	}
}

// TestSendReadsFilesSavedWithAByteOrderMark checks the recipients and global
// substitutions files that programs such as Excel save with a UTF-8 byte
// order mark.
func TestSendReadsFilesSavedWithAByteOrderMark(t *testing.T) {
	const bom = "\uFEFF"
	globalsPath := testutil.WriteTempFile(t, "globals.json", bom+`{"company": "Acme"}`)
	for name, recipients := range map[string]string{
		"recipients.csv":  bom + "email,first_name\na@example.com,Pat\n",
		"recipients.json": bom + `[{"email": "a@example.com", "substitution_data": {"first_name": "Pat"}}]`,
	} {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			sends := expectInlineSends(mockClient, nil)

			_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
				append(inlineSendArgs(testutil.WriteTempFile(t, name, recipients)), "--global-substitutions", globalsPath)...)

			require.NoError(t, err)
			require.Len(t, sends(), 1)
			assert.Equal(t, []common.Recipient{{Email: "a@example.com", Substitutions: map[string]interface{}{"first_name": "Pat"}}},
				sends()[0].request.Recipients)
			assert.Equal(t, map[string]interface{}{"company": "Acme"}, sends()[0].request.Substitutions)
		})
	}
}

// TestSendReadsCSVRowsWithoutTheirEmptyLastColumns checks that a CSV row may
// leave out empty optional columns at its end.
func TestSendReadsCSVRowsWithoutTheirEmptyLastColumns(t *testing.T) {
	mockClient := &mocks.MockClient{}
	sends := expectInlineSends(mockClient, nil)
	recipients := testutil.WriteTempFile(t, "recipients.csv", "email,name,first_name\na@example.com,Pat,Pat\nb@example.com\n")

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", inlineSendArgs(recipients)...)

	require.NoError(t, err)
	require.Len(t, sends(), 1)
	assert.Equal(t, []common.Recipient{
		{Email: "a@example.com", Name: ahasend.String("Pat"), Substitutions: map[string]interface{}{"first_name": "Pat"}},
		{Email: "b@example.com"},
	}, sends()[0].request.Recipients)
}

// TestSendRejectsCSVRowsThatDoNotFitTheHeader checks a CSV row without the
// email column and one with more columns than the header.
func TestSendRejectsCSVRowsThatDoNotFitTheHeader(t *testing.T) {
	tests := map[string]struct {
		csv, want string
	}{
		"no email column":  {"name,email\nPat,a@example.com\nSam\n", "--recipients row 3 has no email column"},
		"too many columns": {"email,name\na@example.com,Pat,extra\n", "--recipients row 2 has 3 columns, more than the 2 of the header"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}

			_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain",
				inlineSendArgs(testutil.WriteTempFile(t, "recipients.csv", test.csv))...)

			require.Error(t, err)
			assert.Equal(t, test.want, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			mockClient.AssertNotCalled(t, "SendMessageWithIdempotencyKey", mock.Anything, mock.Anything)
		})
	}
}

// TestRecipientFilesNoticeReportsEachSaveError checks that a recipients file
// that could not be saved is reported with its own message, and that the
// idempotency keys of the requests of unknown outcome are named even when
// their recipients could not be saved.
func TestRecipientFilesNoticeReportsEachSaveError(t *testing.T) {
	unknown := []batch.FailedRecipient{{Email: "a@example.com"}}
	results := []*batch.SendResult{{Job: &batch.SendJob{IdempotencyKey: "key-1"}, Outcome: batch.OutcomeUnknown}}
	tests := map[string]struct {
		result *batch.BatchResult
		want   []string
	}{
		"failed file not saved": {
			result: &batch.BatchResult{FailedSaveError: errors.New("disk full"), UnknownRecipients: unknown,
				UnknownRecipientsFile: "/home/u/.ahasend/unknown-1.json", Results: results},
			want: []string{
				"Could not save the failed recipients: disk full\n",
				"(idempotency key key-1) is unknown: they may have been sent, and are saved to /home/u/.ahasend/unknown-1.json. Check them",
			},
		},
		"unknown file not saved": {
			result: &batch.BatchResult{FailedRecipientsFile: "/home/u/.ahasend/failed-1.json",
				UnknownSaveError: errors.New("disk full"), UnknownRecipients: unknown, Results: results},
			want: []string{
				"Failed recipients saved to /home/u/.ahasend/failed-1.json; send them again with --recipients /home/u/.ahasend/failed-1.json\n",
				"Could not save the recipients whose outcome is unknown: disk full\n",
				"The outcome of the request of 1 recipients (idempotency key key-1) is unknown: they may have been sent. Check them",
				"--idempotency-key send-key within 24 hours",
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var out strings.Builder

			writeRecipientFilesNotice(&out, test.result, "send-key", "")

			for _, want := range test.want {
				assert.Contains(t, out.String(), want)
			}
		})
	}
}

// TestRecipientFilesNoticeNamesTheFirstKeysWhenTheFileListsThemAll checks
// that the notice of many requests of unknown outcome names the first few
// idempotency keys and the file that lists the rest, and names every key
// when that file could not be saved.
func TestRecipientFilesNoticeNamesTheFirstKeysWhenTheFileListsThemAll(t *testing.T) {
	var results []*batch.SendResult
	var unknown []batch.FailedRecipient
	for i := range 5 {
		results = append(results, &batch.SendResult{Job: &batch.SendJob{IdempotencyKey: fmt.Sprintf("key-%d", i)}, Outcome: batch.OutcomeUnknown})
		unknown = append(unknown, batch.FailedRecipient{Email: fmt.Sprintf("u%d@example.com", i)})
	}

	var saved strings.Builder
	writeRecipientFilesNotice(&saved, &batch.BatchResult{UnknownRecipients: unknown, Results: results,
		UnknownRecipientsFile: "/home/u/.ahasend/unknown-1.json"}, "key", "")
	var notSaved strings.Builder
	writeRecipientFilesNotice(&notSaved, &batch.BatchResult{UnknownRecipients: unknown, Results: results,
		UnknownSaveError: errors.New("disk full")}, "key", "")

	assert.Contains(t, saved.String(), "The outcome of the requests of 5 recipients (idempotency keys key-0, key-1, key-2 "+
		"and 2 more, listed in /home/u/.ahasend/unknown-1.json) is unknown: they may have been sent, and are saved there. Check them")
	assert.Contains(t, notSaved.String(), "(idempotency keys key-0, key-1, key-2, key-3, key-4) is unknown: they may have been sent. Check them")
}

// TestSendUnknownRecipientsFileNamesTheKeysAndCanBeSentAgain checks that
// the file of recipients whose outcome is unknown names the idempotency key
// of each one's request, and that --recipients reads it.
func TestSendUnknownRecipientsFileNamesTheKeysAndCanBeSentAgain(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	recipientsPath := testutil.WriteTempFile(t, "recipients.json", `[{"email": "a@example.com"}]`)
	gateway := &api.APIError{StatusCode: 504, Message: "Gateway Timeout", Raw: []byte(`{"message":"Gateway Timeout"}`)}
	failingClient := &mocks.MockClient{}
	expectInlineSends(failingClient, gateway)

	_, err := testutil.RunWithClient(t, failingClient, NewSendCommand(), "plain",
		append(inlineSendArgs(recipientsPath), "--idempotency-key", "order-7")...)
	require.Error(t, err)
	unknownFiles, err := filepath.Glob(filepath.Join(home, ".ahasend", "unknown-*.json"))
	require.NoError(t, err)
	require.Len(t, unknownFiles, 1)
	data, err := os.ReadFile(unknownFiles[0])
	require.NoError(t, err)
	assert.Contains(t, string(data), `"_idempotency_key": "order-7"`)

	retryClient := &mocks.MockClient{}
	retrySends := expectInlineSends(retryClient, nil)
	_, err = testutil.RunWithClient(t, retryClient, NewSendCommand(), "plain",
		append(inlineSendArgs(unknownFiles[0]), "--idempotency-key", "order-7")...)
	require.NoError(t, err)
	require.Len(t, retrySends(), 1)
	assert.Equal(t, "a@example.com", retrySends()[0].request.Recipients[0].Email)
}

// TestRecipientsCSVReadsEachRow checks how a recipients CSV file becomes
// recipients: the email and name headers in any case, every value trimmed,
// an empty name and empty cells left out, and the file's extension in any
// case.
func TestRecipientsCSVReadsEachRow(t *testing.T) {
	path := testutil.WriteTempFile(t, "recipients.CSV",
		" EMAIL ,Name, first_name ,plan\n"+
			" a@example.com ,  Ann  , Ann ,pro\n"+
			"b@example.com,,,\n")

	recipients, err := loadRecipientsFromFile(path)

	require.NoError(t, err)
	assert.Equal(t, []common.Recipient{
		{Email: "a@example.com", Name: ahasend.String("Ann"),
			Substitutions: map[string]interface{}{"first_name": "Ann", "plan": "pro"}},
		{Email: "b@example.com"},
	}, recipients)
}

// TestAnEmptyCSVCellKeepsTheGlobalSubstitution checks that a recipient whose
// cell is empty gets the --global-substitutions value, and one whose cell is
// set keeps its own.
func TestAnEmptyCSVCellKeepsTheGlobalSubstitution(t *testing.T) {
	recipients, err := loadRecipientsFromCSV(strings.NewReader("email,plan\na@example.com,pro\nb@example.com,\n"))
	require.NoError(t, err)

	merged := mergeGlobalSubstitutions(recipients, map[string]interface{}{"plan": "free"})

	assert.Equal(t, "pro", merged[0].Substitutions["plan"])
	assert.Equal(t, "free", merged[1].Substitutions["plan"])
}

// TestSendRefusesAttachmentsTooLargeForARequest checks that attachments
// whose encoded size leaves no room for the rest of a send request under
// the API's 30 MB limit are refused before authenticating, with their size.
func TestSendRefusesAttachmentsTooLargeForARequest(t *testing.T) {
	dir := t.TempDir()
	var args []string
	for _, name := range []string{"a.bin", "b.bin", "c.bin"} {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, nil, 0o600))
		require.NoError(t, os.Truncate(path, 8_000_000))
		args = append(args, "--attach", path)
	}
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunCommandCombined(t, NewSendCommand(), "plain", append([]string{
		"--from", "sender@example.com", "--to", "a@example.com", "--subject", "Hi", "--text", "Hi",
	}, args...)...)

	require.Error(t, err)
	assert.Regexp(t, `^the --attach files take 32,\d{3},\d{3} bytes encoded, more than the 29,000,000 bytes a send request has room for: `+
		`the API refuses a request over 30,000,000 bytes$`, err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *authCalled)
}

// TestCheckAttachmentsFitLeavesTheMarginForTheRestOfTheRequest checks the
// boundary: attachments whose JSON takes exactly the room a request has
// fit, and one byte more does not, with an error whose sizes read apart.
func TestCheckAttachmentsFitLeavesTheMarginForTheRestOfTheRequest(t *testing.T) {
	attachment := common.Attachment{FileName: "a.bin", ContentType: "application/octet-stream", Base64: true}
	overhead, err := json.Marshal([]common.Attachment{attachment})
	require.NoError(t, err)
	attachment.Data = strings.Repeat("A", maxRequestBytes-requestMarginBytes-len(overhead))

	assert.NoError(t, checkAttachmentsFit([]common.Attachment{attachment}))
	attachment.Data += "A"
	assert.EqualError(t, checkAttachmentsFit([]common.Attachment{attachment}),
		"the --attach files take 29,000,001 bytes encoded, more than the 29,000,000 bytes a send request has room for: "+
			"the API refuses a request over 30,000,000 bytes")
}

// TestSendStopsAfterARequestTooLargeForTheAPI checks that once the API
// refuses a request with attachments with 413, the requests after it, which
// carry the same attachments, are not sent, and the
// command says how many recipients failed and how many were not sent, and
// saves both to the failed recipients file.
func TestSendStopsAfterARequestTooLargeForTheAPI(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	var rows strings.Builder
	rows.WriteString("email\n")
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&rows, "user%d@example.com\n", i)
	}
	recipientsPath := testutil.WriteTempFile(t, "recipients.csv", rows.String())
	tooLarge := &api.APIError{StatusCode: 413, Message: "Request Entity Too Large", Raw: []byte(`{"message":"Request Entity Too Large"}`)}
	mockClient := &mocks.MockClient{}
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything).Return(nil, tooLarge).Once()

	attachment := testutil.WriteTempFile(t, "report.pdf", "%PDF-1.4")

	_, err := testutil.RunWithClient(t, mockClient, NewSendCommand(), "plain", "--template-id", sendTemplateID,
		"--recipients", recipientsPath, "--attach", attachment)

	require.Error(t, err)
	assert.Equal(t, "100 of 250 recipients failed and 150 were not sent: a request was too large for the API (HTTP 413)", err.Error())
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	mockClient.AssertExpectations(t)
	failedFiles, err := filepath.Glob(filepath.Join(home, ".ahasend", "failed-*.json"))
	require.NoError(t, err)
	require.Len(t, failedFiles, 1)
	data, err := os.ReadFile(failedFiles[0])
	require.NoError(t, err)
	var saved []map[string]any
	require.NoError(t, json.Unmarshal(data, &saved))
	assert.Len(t, saved, 250)
}

// TestSendWritesLogsAboveTheProgressBar checks, on a terminal, that a log
// line written while the progress bar is drawn clears the bar's line first,
// and that the bar is drawn again below it.
func TestSendWritesLogsAboveTheProgressBar(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv("TERM", "xterm")
	terminal := testutil.NewTerminal(t)
	shown := terminal.Shown(t)
	var rows strings.Builder
	rows.WriteString("email\n")
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&rows, "user%d@example.com\n", i)
	}
	recipientsPath := testutil.WriteTempFile(t, "recipients.csv", rows.String())
	mockClient := &mocks.MockClient{}
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything).
		Return(mocks.NewMessageResponse("msg-1"), nil).Once()
	mockClient.On("SendTemplateMessageWithIdempotencyKey", mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			// The log comes once the bar shows the first request
			for deadline := time.Now().Add(5 * time.Second); !strings.Contains(shown(), "(1/150)") && time.Now().Before(deadline); {
				time.Sleep(5 * time.Millisecond)
			}
			logger.Get().Warn("a log line during the send")
		}).
		Return(mocks.NewMessageResponse("msg-2"), nil).Once()
	testutil.InjectClient(t, mockClient)
	cmd := NewSendCommand()
	var stdout bytes.Buffer
	cmd.SetContext(context.WithValue(context.Background(), printer.ResponseHandlerKey,
		testutil.ResponseHandler(t, "plain", &stdout, terminal.Stdin)))
	cmd.SetOut(&stdout)
	cmd.SetErr(terminal.Stdin)
	cmd.SetArgs([]string{"--template-id", sendTemplateID, "--recipients", recipientsPath, "--progress"})

	require.NoError(t, cmd.Execute())

	// Each mock response holds one recipient, so the bar counts one per
	// request
	require.Eventually(t, func() bool { return strings.Contains(shown(), "\r\033[KSent 2/150 messages") }, 5*time.Second,
		10*time.Millisecond, "%q", shown())
	// The ETA depends on how fast the requests run, so any value will do
	assert.Regexp(t, `\(1/150\) ETA: \d+s\r\x1b\[Ktime=`, shown())
	assert.Contains(t, shown(), "msg=\"a log line during the send\"\r\n\r\033[K[")
}
