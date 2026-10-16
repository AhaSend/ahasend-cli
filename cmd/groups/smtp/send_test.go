package smtp

import (
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sendArgs are the flags of a complete smtp send to server.
func sendArgs(server *fakeSMTPServer, extra ...string) []string {
	return append([]string{
		"--from", "sender@example.com", "--to", "recipient@example.com",
		"--subject", "Hello", "--text", "Hi there",
		"--username", "smtp-user", "--password", "smtp-pass",
		"--server", server.Addr,
	}, extra...)
}

// TestSendCommand_OutputFormats sends a message to a loopback SMTP server in
// every output format and checks the printed result.
func TestSendCommand_OutputFormats(t *testing.T) {
	tests := []struct {
		format string
		assert func(t *testing.T, stdout string)
	}{
		{"table", func(t *testing.T, stdout string) {
			assert.True(t, strings.HasPrefix(stdout, "Email sent successfully via SMTP\n\n"), stdout)
			assert.Regexp(t, `│ Status +│ Sent +│`, stdout)
			assert.Regexp(t, `│ Message ID +│ `+generatedMessageID+` +│`, stdout)
			assert.Regexp(t, `│ Delivery +│ Message queued for delivery +│`, stdout)
		}},
		{"plain", func(t *testing.T, stdout string) {
			assert.Regexp(t, `^Email sent successfully via SMTP\nMessage ID: `+generatedMessageID+`\n$`, stdout)
		}},
		{"json", func(t *testing.T, stdout string) {
			assert.Regexp(t, `^\{\n  "success": true,\n  "message_id": "`+generatedMessageID+`"\n\}\n$`, stdout)
		}},
		{"csv", func(t *testing.T, stdout string) {
			assert.Regexp(t, `^success,message_id\ntrue,`+generatedMessageID+`\n$`, stdout)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			server := newFakeSMTPServer(t)

			stdout, stderr, err := testutil.RunCommand(t, NewSendCommand(), tt.format, "", sendArgs(server)...)

			require.NoError(t, err)
			assert.Empty(t, stderr)
			tt.assert(t, stdout)
			messages := server.Messages()
			require.Len(t, messages, 1)
			assert.Contains(t, messages[0], "Subject: Hello")
			assert.Contains(t, messages[0], "Hi there")
			// The printed ID is the Message-ID header the message carries
			id := regexp.MustCompile(generatedMessageID).FindString(stdout)
			assert.Contains(t, messages[0], "\nMessage-ID: "+id+"\n")
		})
	}
}

// generatedMessageID matches the Message-ID smtp send gives a message from
// sender@example.com: a UUID at the sender's domain.
const generatedMessageID = `<[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}@example\.com>`

// TestSendCommand_ReportsTheMessageIDOfAHeader checks that a Message-ID given
// with --header, in any case, is the one sent and printed, and that the
// message carries no second one.
func TestSendCommand_ReportsTheMessageIDOfAHeader(t *testing.T) {
	server := newFakeSMTPServer(t)

	stdout, _, err := testutil.RunCommand(t, NewSendCommand(), "csv", "",
		sendArgs(server, "--header", "message-id: <given@example.com>")...)

	require.NoError(t, err)
	assert.Equal(t, "success,message_id\ntrue,<given@example.com>\n", stdout)
	messages := server.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, 1, strings.Count(strings.ToLower(messages[0]), "\nmessage-id: "), messages[0])
	assert.Contains(t, strings.ToLower(messages[0]), "\nmessage-id: <given@example.com>\n")
}

// TestSendCommand_TestModeConnectsWithoutSending checks that --test reports
// a working connection and login, and delivers nothing.
func TestSendCommand_TestModeConnectsWithoutSending(t *testing.T) {
	server := newFakeSMTPServer(t)

	stdout, stderr, err := testutil.RunCommand(t, NewSendCommand(), "json", "", sendArgs(server, "--test")...)

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, `{
  "test_mode": true,
  "success": true,
  "connected": true,
  "authenticated": true,
  "error": ""
}
`, stdout)
	assert.Empty(t, server.Messages())
}

// TestSendCommand_TestModeNeedsOnlyTheServer checks that --test asks for no
// message flags: it connects with --server alone.
func TestSendCommand_TestModeNeedsOnlyTheServer(t *testing.T) {
	server := newFakeSMTPServer(t)

	stdout, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", "--test", "--server", server.Addr)

	require.NoError(t, err)
	assert.Contains(t, stdout, "  Connection: Successful\n")
}

// TestSendCommand_TestModeNeedsBothCredentials checks that --test with one of
// --username and --password asks for the other.
func TestSendCommand_TestModeNeedsBothCredentials(t *testing.T) {
	server := newFakeSMTPServer(t)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", "--test", "--server", server.Addr, "--username", "smtp-user")

	require.Error(t, err)
	assert.Equal(t, "--password is required", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
}

// TestSendCommand_TestModeReportsARefusedLogin checks that --test tells a
// refused login from a failed connection, with the server's reply.
func TestSendCommand_TestModeReportsARefusedLogin(t *testing.T) {
	server := newFakeSMTPServerRefusingLogins(t)

	stdout, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "",
		"--test", "--server", server.Addr, "--username", "smtp-user", "--password", "wrong")

	require.Error(t, err)
	assert.Equal(t, "SMTP login failed: "+fakeSMTPLoginRefusal, err.Error())
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	assert.Equal(t, "SMTP Test Result:\n  Connection: Successful\n  Authentication: Failed\n"+
		"  Error: "+fakeSMTPLoginRefusal+"\n  Status: Please check your SMTP username and password\n", stdout)
}

// TestSendCommand_TestModeSaysWhetherItLoggedIn checks that --test reports a
// login only when credentials are given.
func TestSendCommand_TestModeSaysWhetherItLoggedIn(t *testing.T) {
	tests := map[string]struct {
		credentials bool
		want        string
	}{
		"credentials":    {true, "  Authentication: Passed\n"},
		"no credentials": {false, "  Authentication: Not tested (no --username and --password)\n"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := newFakeSMTPServer(t)
			args := []string{"--test", "--server", server.Addr}
			if test.credentials {
				args = append(args, "--username", "smtp-user", "--password", "smtp-pass")
			}

			stdout, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", args...)

			require.NoError(t, err)
			assert.Equal(t, "SMTP Test Result:\n  Connection: Successful\n"+test.want+
				"  Status: No message was sent\n", stdout)
			assert.Empty(t, server.Messages())
		})
	}
}

// TestMayLogInSendsCredentialsOnlyOverTLSOrToThisMachine checks the rule that
// keeps credentials off an unencrypted connection to another machine: the
// rule of net/smtp's PLAIN login, exactly localhost, 127.0.0.1 and ::1.
func TestMayLogInSendsCredentialsOnlyOverTLSOrToThisMachine(t *testing.T) {
	tests := []struct {
		host      string
		encrypted bool
		want      bool
	}{
		{"send.ahasend.com", false, false},
		{"203.0.113.10", false, false},
		{"send.ahasend.com", true, true},
		{"127.0.0.1", false, true},
		{"::1", false, true},
		{"localhost", false, true},
		{"127.0.0.2", false, false},
		{"LOCALHOST", false, false},
		{"0.0.0.0", false, false},
	}
	for _, test := range tests {
		assert.Equal(t, test.want, mayLogIn(test.host, test.encrypted), "%s encrypted=%v", test.host, test.encrypted)
	}
}

// TestSendCommand_LogsInWithoutTLSToALoopbackServer checks that a loopback
// server that offers no STARTTLS still gets the login and the message.
func TestSendCommand_LogsInWithoutTLSToALoopbackServer(t *testing.T) {
	server := newFakeSMTPServer(t)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server)...)

	require.NoError(t, err)
	assert.Len(t, server.Messages(), 1)
}

// TestSendCommand_LogsInWithAuthLogin checks the LOGIN mechanism, used when
// the server offers it without PLAIN.
func TestSendCommand_LogsInWithAuthLogin(t *testing.T) {
	server := newFakeSMTPServerWithLoginAuth(t)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server)...)

	require.NoError(t, err)
	assert.Equal(t, []string{"smtp-user:smtp-pass"}, server.Logins())
	assert.Len(t, server.Messages(), 1)
}

// TestSendCommand_HidesThePasswordOnATerminal checks that the password prompt
// hides the password as it is typed.
func TestSendCommand_HidesThePasswordOnATerminal(t *testing.T) {
	server := newFakeSMTPServerWithLoginAuth(t)
	terminal := testutil.NewTerminal(t)
	args := []string{
		"--from", "sender@example.com", "--to", "recipient@example.com",
		"--subject", "Hello", "--text", "Hi there", "--username", "smtp-user",
		"--server", server.Addr,
	}

	wait := terminal.Start(t, NewSendCommand(), "plain", args...)
	terminal.TypeHidden(t, "smtp-pass")
	_, stderr, err := wait()

	require.NoError(t, err)
	assert.Equal(t, "SMTP password: \n", stderr)
	assert.Equal(t, []string{"smtp-user:smtp-pass"}, server.Logins())
}

// otherHostAddr is the address of server dialed as 0.0.0.0: it reaches the
// loopback server, but is not a host that may get credentials without TLS.
func otherHostAddr(t *testing.T, server *fakeSMTPServer) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("dialing 0.0.0.0 reaches a loopback listener on Linux only")
	}
	_, port, err := net.SplitHostPort(server.Addr)
	require.NoError(t, err)
	return net.JoinHostPort("0.0.0.0", port)
}

// TestSendCommand_RefusesToLogInWithoutTLSToAnotherHost checks that a server
// that is not localhost, 127.0.0.1 or ::1 and offers AUTH without STARTTLS
// gets neither the login nor the message.
func TestSendCommand_RefusesToLogInWithoutTLSToAnotherHost(t *testing.T) {
	server := newFakeSMTPServer(t)
	args := sendArgs(server, "--server", otherHostAddr(t, server))

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", args...)

	require.Error(t, err)
	assert.Equal(t, "SMTP send failed: "+errUnencryptedLogin.Error(), err.Error())
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	assert.Equal(t, []string{"EHLO"}, server.Verbs())
	assert.Empty(t, server.Messages())
}

// TestSendCommand_TestModeRefusesToLogInWithoutTLSToAnotherHost checks that
// --test fails the same way, and reports that it connected.
func TestSendCommand_TestModeRefusesToLogInWithoutTLSToAnotherHost(t *testing.T) {
	server := newFakeSMTPServer(t)

	stdout, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "",
		"--test", "--server", otherHostAddr(t, server), "--username", "smtp-user", "--password", "smtp-pass")

	require.Error(t, err)
	assert.Equal(t, "SMTP login failed: "+errUnencryptedLogin.Error(), err.Error())
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	assert.Equal(t, "SMTP Test Result:\n  Connection: Successful\n  Authentication: Failed\n"+
		"  Error: "+errUnencryptedLogin.Error()+"\n  Status: Please check your SMTP username and password\n", stdout)
	assert.NotContains(t, server.Verbs(), "AUTH")
}

// TestSendCommand_FailsWhenTheServerOffersNoLogin checks that credentials
// and a server that offers neither STARTTLS nor AUTH fail, with and without
// --test, and send nothing.
func TestSendCommand_FailsWhenTheServerOffersNoLogin(t *testing.T) {
	tests := map[string]struct {
		test bool
		want string
	}{
		"send": {false, "SMTP send failed: the SMTP server does not accept a login"},
		"test": {true, "SMTP login failed: the SMTP server does not accept a login"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := newFakeSMTPServerWithoutAuth(t)
			args := sendArgs(server)
			if test.test {
				args = append(args, "--test")
			}

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", args...)

			require.Error(t, err)
			assert.Equal(t, test.want, err.Error())
			assert.Equal(t, 1, clierrors.GetExitCode(err))
			assert.NotContains(t, server.Verbs(), "MAIL")
			assert.Empty(t, server.Messages())
		})
	}
}

// TestSendCommand_PrintsTheServersReply checks that a refused login and a
// refused recipient fail with the server's reply as it sent it.
func TestSendCommand_PrintsTheServersReply(t *testing.T) {
	tests := map[string]struct {
		server func(*testing.T) *fakeSMTPServer
		reply  string
	}{
		"login":     {newFakeSMTPServerRefusingLogins, fakeSMTPLoginRefusal},
		"recipient": {newFakeSMTPServerRefusingRecipients, "the SMTP server refused recipient recipient@example.com: " + fakeSMTPRcptRefusal},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := test.server(t)

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server)...)

			require.Error(t, err)
			assert.Equal(t, "SMTP send failed: "+test.reply, err.Error())
			assert.Equal(t, 1, clierrors.GetExitCode(err))
		})
	}
}

// TestSendCommand_ARefusedRecipientSendsNothingAndQuits checks that a
// recipient the server refuses, among others it accepts, is named in the
// error, and that the command sends no data, resets the transaction and
// quits.
func TestSendCommand_ARefusedRecipientSendsNothingAndQuits(t *testing.T) {
	server := startFakeSMTPServer(t, &fakeSMTPServer{offerAuth: true, refuseRcptTo: "bad@example.com"})

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "",
		sendArgs(server, "--to", "bad@example.com", "--cc", "copy@example.com")...)

	require.Error(t, err)
	assert.Equal(t, "SMTP send failed: the SMTP server refused recipient bad@example.com: "+fakeSMTPRcptRefusal, err.Error())
	assert.Equal(t, 1, clierrors.GetExitCode(err))
	assert.Empty(t, server.Messages())
	require.Eventually(t, func() bool {
		verbs := server.Verbs()
		return len(verbs) >= 2 && verbs[len(verbs)-1] == "QUIT"
	}, time.Second, 10*time.Millisecond, "verbs: %v", server.Verbs())
	verbs := server.Verbs()
	assert.Equal(t, []string{"RCPT", "RSET", "QUIT"}, verbs[len(verbs)-3:])
	assert.NotContains(t, verbs, "DATA")
}

// TestSendCommand_TimesOutOnASilentServer checks that a server that accepts
// the connection and never greets fails the send and --test once the session
// times out, as a network failure.
func TestSendCommand_TimesOutOnASilentServer(t *testing.T) {
	previous := sessionTimeout
	sessionTimeout = 200 * time.Millisecond
	t.Cleanup(func() { sessionTimeout = previous })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()
	server := &fakeSMTPServer{Addr: listener.Addr().String()}

	for name, extra := range map[string][]string{"send": nil, "test": {"--test"}} {
		t.Run(name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server, extra...)...)
				done <- err
			}()
			var err error
			select {
			case err = <-done:
			case <-time.After(5 * time.Second):
				require.FailNow(t, "the command still waits for the greeting")
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), "i/o timeout")
			assert.Equal(t, 7, clierrors.GetExitCode(err))
		})
	}
}

// TestSendCommand_ReadsEveryAttachmentBeforeConnecting checks that a file
// --attach cannot attach fails before the command connects.
func TestSendCommand_ReadsEveryAttachmentBeforeConnecting(t *testing.T) {
	dir := t.TempDir()
	large := filepath.Join(dir, "large.bin")
	require.NoError(t, os.WriteFile(large, nil, 0o600))
	require.NoError(t, os.Truncate(large, cmdutil.MaxAttachmentSize+1))
	tests := map[string]struct {
		path string
		want string
	}{
		"directory": {dir, "--attach " + dir + " is a directory"},
		"too large": {large, "--attach file " + large + " is larger than 10,485,760 bytes (10 MiB)"},
		"missing":   {filepath.Join(dir, "missing.pdf"), "failed to read --attach: " + testutil.StatError(t, filepath.Join(dir, "missing.pdf"))},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := newFakeSMTPServer(t)

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server, "--attach", test.path)...)

			require.Error(t, err)
			assert.Equal(t, test.want, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.Empty(t, server.Verbs())
		})
	}
}

// TestSendCommand_AttachesTheFileRead checks that the message carries the
// attachment as read before connecting.
func TestSendCommand_AttachesTheFileRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("attached notes"), 0o600))
	server := newFakeSMTPServer(t)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server, "--attach", path)...)

	require.NoError(t, err)
	messages := server.Messages()
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], `filename="notes.txt"`)
	assert.Contains(t, messages[0], base64.StdEncoding.EncodeToString([]byte("attached notes")))
}

// TestSendCommand_AClosedConnectionIsANetworkFailure checks that a server
// that closes the connection fails the send and --test as a network failure.
func TestSendCommand_AClosedConnectionIsANetworkFailure(t *testing.T) {
	tests := map[string]struct {
		closeOn string
		extra   []string
	}{
		"send closed at MAIL": {"MAIL", nil},
		"send closed at AUTH": {"AUTH", nil},
		"test closed at AUTH": {"AUTH", []string{"--test"}},
		"test closed at EHLO": {"EHLO", []string{"--test"}},
		"send closed at RCPT": {"RCPT", nil},
		"send closed at DATA": {"DATA", nil},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := startFakeSMTPServer(t, &fakeSMTPServer{offerAuth: true, closeOn: test.closeOn})

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server, test.extra...)...)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "the SMTP server closed the connection")
			assert.NotContains(t, err.Error(), "accepted")
			assert.Equal(t, 7, clierrors.GetExitCode(err))
		})
	}
}

// TestSendCommand_AResetConnectionIsAClosedConnection checks that a server
// that resets the connection fails the send and --test as one that closed it.
func TestSendCommand_AResetConnectionIsAClosedConnection(t *testing.T) {
	tests := map[string]struct {
		resetOn string
		extra   []string
	}{
		"send reset at MAIL": {"MAIL", nil},
		"test reset at EHLO": {"EHLO", []string{"--test"}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := startFakeSMTPServer(t, &fakeSMTPServer{offerAuth: true, resetOn: test.resetOn})

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server, test.extra...)...)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "the SMTP server closed the connection")
			assert.Equal(t, 7, clierrors.GetExitCode(err))
		})
	}
}

// TestSendCommand_AConnectionLostAfterTheMessageMayHaveSentIt checks that a
// connection that closes or times out after the whole message was sent,
// before the server's reply, says the message may have been accepted, as a
// network failure.
func TestSendCommand_AConnectionLostAfterTheMessageMayHaveSentIt(t *testing.T) {
	previous := sendTimeout
	sendTimeout = 200 * time.Millisecond
	t.Cleanup(func() { sendTimeout = previous })
	tests := map[string]struct {
		server *fakeSMTPServer
		want   string
	}{
		"closed": {&fakeSMTPServer{offerAuth: true, closeOn: "."},
			"the SMTP server closed the connection after the message was sent; it may have been accepted"},
		"silent": {&fakeSMTPServer{offerAuth: true, silentOn: "."},
			"the SMTP server did not answer after the message was sent; it may have been accepted"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := startFakeSMTPServer(t, test.server)

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server)...)

			require.EqualError(t, err, test.want)
			assert.Equal(t, 7, clierrors.GetExitCode(err))
			assert.Len(t, server.Messages(), 1)
		})
	}
}

// TestSendCommand_AFailedQuitAfterTheMessageWasAcceptedSucceeds checks that
// a send succeeds once the server accepted the message, even when the
// connection closes at QUIT.
func TestSendCommand_AFailedQuitAfterTheMessageWasAcceptedSucceeds(t *testing.T) {
	server := startFakeSMTPServer(t, &fakeSMTPServer{offerAuth: true, closeOn: "QUIT"})

	stdout, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "", sendArgs(server)...)

	require.NoError(t, err)
	assert.Contains(t, stdout, "Email sent successfully via SMTP")
	assert.Len(t, server.Messages(), 1)
}

// TestSendCommand_AnEnvelopeHeaderSendsNothing checks that a --header naming
// a recipient header, which gomail would build the envelope from, fails
// before connecting, so no address but the flags' gets the message.
func TestSendCommand_AnEnvelopeHeaderSendsNothing(t *testing.T) {
	server := newFakeSMTPServer(t)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "",
		sendArgs(server, "--header", "to: other@example.com")...)

	require.Error(t, err)
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.Empty(t, server.Rcpts())
	assert.Empty(t, server.Messages())
}

// TestSendCommand_HeaderMaySetTheDate checks that --header may set Date, in
// any case, and the message then has that date only.
func TestSendCommand_HeaderMaySetTheDate(t *testing.T) {
	server := newFakeSMTPServer(t)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "",
		sendArgs(server, "--header", `"date: Mon, 12 Oct 2026 10:00:00 +0000"`, "--header", "Message-ID: <1@example.com>")...)

	require.NoError(t, err)
	assert.Equal(t, []string{"RCPT TO:<recipient@example.com>"}, server.Rcpts())
	messages := server.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, 1, strings.Count("\n"+strings.ToLower(messages[0]), "\ndate: "), messages[0])
	assert.Contains(t, messages[0], "Date: Mon, 12 Oct 2026 10:00:00 +0000")
	assert.Contains(t, messages[0], "Message-ID: <1@example.com>")
}

// stdinSendArgs are the flags of a complete smtp send to server that reads
// the password from stdin.
func stdinSendArgs(server *fakeSMTPServer, extra ...string) []string {
	return append([]string{
		"--from", "sender@example.com", "--to", "recipient@example.com",
		"--subject", "Hello", "--text", "Hi there",
		"--username", "smtp-user", "--password-stdin",
		"--server", server.Addr,
	}, extra...)
}

func TestSendCommand_ReadsThePasswordFromStdin(t *testing.T) {
	tests := []struct{ name, stdin, password string }{
		{"a line", "smtp-pass\n", "smtp-pass"},
		{"a CRLF line, spaces kept", " smtp pass \r\nnext line\n", " smtp pass "},
		{"no line break", "smtp-pass", "smtp-pass"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newFakeSMTPServerWithLoginAuth(t)

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", test.stdin, stdinSendArgs(server)...)

			require.NoError(t, err)
			assert.Equal(t, []string{"smtp-user:" + test.password}, server.Logins())
			assert.Len(t, server.Messages(), 1)
		})
	}
}

func TestSendCommand_TestModeReadsThePasswordFromStdin(t *testing.T) {
	server := newFakeSMTPServerWithLoginAuth(t)

	_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", "smtp-pass\n",
		"--test", "--server", server.Addr, "--username", "smtp-user", "--password-stdin")

	require.NoError(t, err)
	assert.Equal(t, []string{"smtp-user:smtp-pass"}, server.Logins())
}

func TestSendCommand_PasswordStdinRefusals(t *testing.T) {
	tests := []struct {
		name    string
		stdin   string
		args    []string
		message string
	}{
		{"with --password", "smtp-pass\n", []string{"--password", "other"},
			"--password-stdin and --password cannot be used together"},
		{"empty stdin", "", nil, "--password-stdin read no password from stdin"},
		{"an empty first line", "\nsmtp-pass\n", nil, "--password-stdin read no password from stdin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newFakeSMTPServerWithLoginAuth(t)

			_, _, err := testutil.RunCommand(t, NewSendCommand(), "plain", test.stdin, stdinSendArgs(server, test.args...)...)

			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.Empty(t, server.Verbs())
		})
	}
}

// TestSendCommand_PasswordStdinNeverPrompts checks that a command reading
// the password from stdin asks for no other value, since stdin holds the
// password.
func TestSendCommand_PasswordStdinNeverPrompts(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	server := newFakeSMTPServerWithLoginAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "smtp-pass\n",
		"--to", "recipient@example.com", "--subject", "Hello", "--text", "Hi there",
		"--username", "smtp-user", "--password-stdin", "--server", server.Addr)

	require.Error(t, err)
	assert.Equal(t, "--from is required", err.Error())
	assert.NotContains(t, stderr, "Sender email address:")
	assert.Empty(t, server.Verbs())
}

// TestSendCommand_TestModeWithPasswordStdinNeedsAUsername checks that
// --test --password-stdin, which asks to log in, fails without --username
// instead of only connecting, and never prompts, since stdin holds the
// password.
func TestSendCommand_TestModeWithPasswordStdinNeedsAUsername(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	server := newFakeSMTPServerWithLoginAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewSendCommand(), "plain", "smtp-pass\n",
		"--test", "--server", server.Addr, "--password-stdin")

	require.Error(t, err)
	assert.Equal(t, "--username is required", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.NotContains(t, stderr, "SMTP username:")
	assert.Empty(t, server.Verbs())
}
