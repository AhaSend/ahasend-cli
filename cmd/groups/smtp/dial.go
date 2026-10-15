package smtp

import (
	"crypto/tls"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"gopkg.in/gomail.v2"
)

var (
	// errUnencryptedLogin is the failure of a login the server would receive
	// in plain text.
	errUnencryptedLogin = stderrors.New("the SMTP server did not offer STARTTLS; refusing to send credentials unencrypted")
	// errNoLogin is the failure of a login to a server that offers no AUTH.
	errNoLogin = stderrors.New("the SMTP server does not accept a login")
)

// dialTimeout bounds the time to connect to the SMTP server.
const dialTimeout = 10 * time.Second

// sessionTimeout bounds the time from connecting to the end of the login:
// the greeting, TLS and AUTH. sendTimeout bounds the time to send a message,
// from MAIL to QUIT. Tests shorten them.
var (
	sessionTimeout = 30 * time.Second
	sendTimeout    = 5 * time.Minute
)

// smtpSession is a connection to an SMTP server.
type smtpSession struct {
	client *smtp.Client
	conn   net.Conn
	host   string
}

// connectSMTP connects to host:port, reads the greeting, says EHLO and starts
// TLS: at once on port 465, otherwise with STARTTLS when the server offers it.
// The connection, the greeting, TLS and the login that may follow must end
// within sessionTimeout.
func connectSMTP(host string, port int) (*smtpSession, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), dialTimeout)
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(sessionTimeout)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	tlsConfig := &tls.Config{ServerName: host}
	transport := conn
	if port == 465 {
		transport = tls.Client(conn, tlsConfig)
	}
	client, err := smtp.NewClient(transport, host)
	if err != nil {
		_ = transport.Close()
		return nil, err
	}
	session := &smtpSession{client: client, conn: conn, host: host}
	if err := client.Hello("localhost"); err != nil {
		session.close()
		return nil, err
	}
	if ok, _ := client.Extension("STARTTLS"); ok && port != 465 {
		if err := client.StartTLS(tlsConfig); err != nil {
			session.close()
			return nil, err
		}
	}
	return session, nil
}

// logIn logs in with username and password. Credentials never go over a
// connection without TLS, except to a loopback host: that fails with
// errUnencryptedLogin before the login. A server that offers no AUTH fails
// with errNoLogin.
func (s *smtpSession) logIn(username, password string) error {
	if _, encrypted := s.client.TLSConnectionState(); !mayLogIn(s.host, encrypted) {
		return errUnencryptedLogin
	}
	ok, mechanisms := s.client.Extension("AUTH")
	if !ok {
		return errNoLogin
	}
	return s.client.Auth(chooseAuth(mechanisms, s.host, username, password))
}

// mayLogIn reports whether credentials may go to host over a connection that
// is encrypted or not: only an encrypted one, or one to localhost, 127.0.0.1
// or ::1, which never leaves the machine. It is the rule of net/smtp's PLAIN
// login, so every login follows it.
func mayLogIn(host string, encrypted bool) bool {
	return encrypted || host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// chooseAuth returns the login for the server's AUTH mechanisms: CRAM-MD5
// when offered, LOGIN when offered without PLAIN, PLAIN otherwise.
func chooseAuth(mechanisms, host, username, password string) smtp.Auth {
	switch {
	case strings.Contains(mechanisms, "CRAM-MD5"):
		return smtp.CRAMMD5Auth(username, password)
	case strings.Contains(mechanisms, "LOGIN") && !strings.Contains(mechanisms, "PLAIN"):
		return &loginAuth{username: username, password: password}
	default:
		return smtp.PlainAuth("", username, password, host)
	}
}

// loginAuth is the LOGIN mechanism: the server asks for the username and the
// password, and the client answers each.
type loginAuth struct {
	username, password string
}

func (a *loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSuffix(string(fromServer), ":")) {
	case "username":
		return []byte(a.username), nil
	case "password":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected server challenge: %s", fromServer)
	}
}

// send sends message, which must end within sendTimeout, then quits. Once
// the server accepted the message, a failed QUIT only closes the connection.
// When the server refuses a command, such as a recipient, nothing is sent:
// the session resets the transaction with RSET and quits. A connection that
// fails after the whole message was written, before the server's reply,
// fails with an unknownOutcomeError.
func (s *smtpSession) send(message *gomail.Message) error {
	if err := s.conn.SetDeadline(time.Now().Add(sendTimeout)); err != nil {
		s.close()
		return err
	}
	// gomail.Send reports a failure of deliver with its own prefix; keep the
	// failure itself.
	var failure error
	deliver := gomail.SendFunc(func(from string, to []string, msg io.WriterTo) error {
		failure = s.deliver(from, to, msg)
		return failure
	})
	if err := gomail.Send(deliver, message); err != nil {
		var reply *textproto.Error
		if stderrors.As(failure, &reply) {
			_ = s.client.Reset()
			s.quit()
		} else {
			s.close()
		}
		if failure != nil {
			return failure
		}
		return err
	}
	s.quit()
	return nil
}

// deliver sends msg from from to every address of to. A recipient the server
// refuses fails with a refusedRecipientError, before any data is sent.
func (s *smtpSession) deliver(from string, to []string, msg io.WriterTo) error {
	if err := s.client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := s.client.Rcpt(recipient); err != nil {
			return &refusedRecipientError{recipient: recipient, err: err}
		}
	}
	data, err := s.client.Data()
	if err != nil {
		return err
	}
	if _, err := msg.WriteTo(data); err != nil {
		_ = data.Close()
		return err
	}
	// Close ends the data and reads the server's reply. Without a reply the
	// server may have received the whole message and queued it.
	if err := data.Close(); err != nil {
		var reply *textproto.Error
		if !stderrors.As(err, &reply) {
			return &unknownOutcomeError{err: err}
		}
		return err
	}
	return nil
}

// refusedRecipientError is the server's refusal of a recipient, or the
// failure of the connection while sending it.
type refusedRecipientError struct {
	recipient string
	err       error
}

func (e *refusedRecipientError) Error() string {
	var reply *textproto.Error
	if stderrors.As(e.err, &reply) {
		return fmt.Sprintf("the SMTP server refused recipient %s: %s", e.recipient, smtpErrorText(e.err))
	}
	return e.err.Error()
}

func (e *refusedRecipientError) Unwrap() error { return e.err }

// unknownOutcomeError is the failure of a connection after the whole message
// was written and before the server replied to it: the server may have
// accepted the message.
type unknownOutcomeError struct {
	err error
}

func (e *unknownOutcomeError) Error() string {
	switch {
	case connectionClosed(e.err):
		return "the SMTP server closed the connection after the message was sent; it may have been accepted"
	case stderrors.Is(e.err, os.ErrDeadlineExceeded):
		return "the SMTP server did not answer after the message was sent; it may have been accepted"
	default:
		return fmt.Sprintf("the connection failed after the message was sent; it may have been accepted: %v", e.err)
	}
}

func (e *unknownOutcomeError) Unwrap() error { return e.err }

// connectionClosed reports whether err is the end of a connection the server
// closed, or reset.
func connectionClosed(err error) bool {
	if stderrors.Is(err, io.EOF) || stderrors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	for _, reset := range resetErrors {
		if stderrors.Is(err, reset) {
			return true
		}
	}
	return false
}

// quit ends the session, closing the connection whether or not the server
// answers QUIT.
func (s *smtpSession) quit() {
	if err := s.client.Quit(); err != nil {
		s.close()
	}
}

// close closes the connection without QUIT.
func (s *smtpSession) close() {
	_ = s.client.Close()
}

// smtpError is an SMTP failure of action, such as "SMTP send failed". It
// reads as smtpErrorText and unwraps to the failure, so a network failure
// still exits 7.
type smtpError struct {
	action string
	err    error
}

// sessionFailure returns err, the failure of a session, as a network failure
// that says so when the server closed the connection.
func sessionFailure(err error) error {
	if connectionClosed(err) {
		return errors.NewNetworkError("the SMTP server closed the connection", nil)
	}
	return err
}

func (e *smtpError) Error() string { return e.action + ": " + smtpErrorText(e.err) }

func (e *smtpError) Unwrap() error { return e.err }

// smtpErrorText returns the text of an SMTP failure: the server's reply as it
// sent it, after the recipient it refused when it refused one, or the error
// of a connection that got no reply.
func smtpErrorText(err error) string {
	var refused *refusedRecipientError
	if stderrors.As(err, &refused) {
		return refused.Error()
	}
	var reply *textproto.Error
	if stderrors.As(err, &reply) {
		return fmt.Sprintf("%d %s", reply.Code, reply.Msg)
	}
	return err.Error()
}
