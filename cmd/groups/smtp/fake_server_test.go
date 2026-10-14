package smtp

import (
	"encoding/base64"
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeSMTPServer is a loopback SMTP server that accepts every message and
// keeps the data of each one. It offers AUTH PLAIN, or the mechanisms in
// authMechanisms, and accepts every login, only when offerAuth is set.
type fakeSMTPServer struct {
	Addr       string
	offerAuth  bool
	refuseAuth bool
	refuseRcpt bool
	// refuseRcptTo is an address whose RCPT the server refuses, or "".
	refuseRcptTo   string
	authMechanisms string
	// closeOn is the verb on which the server closes the connection without
	// a reply, or "." to close it after reading a message's data; silentOn
	// is the one on which it stops replying and waits for the client to
	// close.
	closeOn, silentOn string
	// resetOn is the verb on which the server resets the connection.
	resetOn string

	mu       sync.Mutex
	messages []string
	logins   []string
	verbs    []string
	rcpts    []string
}

// newFakeSMTPServer starts a fakeSMTPServer that offers AUTH and stops when
// the test ends.
func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	return startFakeSMTPServer(t, &fakeSMTPServer{offerAuth: true})
}

// newFakeSMTPServerWithoutAuth starts a fakeSMTPServer that offers no AUTH
// and stops when the test ends.
func newFakeSMTPServerWithoutAuth(t *testing.T) *fakeSMTPServer {
	return startFakeSMTPServer(t, &fakeSMTPServer{})
}

// fakeSMTPLoginRefusal is the reply of a fakeSMTPServer that refuses every
// login, and fakeSMTPRcptRefusal of one that refuses every recipient.
const (
	fakeSMTPLoginRefusal = "535 5.7.8 Authentication credentials invalid"
	fakeSMTPRcptRefusal  = "550 5.1.1 no such user"
)

// newFakeSMTPServerRefusingRecipients starts a fakeSMTPServer that offers
// AUTH, refuses every recipient and stops when the test ends.
func newFakeSMTPServerRefusingRecipients(t *testing.T) *fakeSMTPServer {
	return startFakeSMTPServer(t, &fakeSMTPServer{offerAuth: true, refuseRcpt: true})
}

// newFakeSMTPServerRefusingLogins starts a fakeSMTPServer that offers AUTH,
// refuses every login and stops when the test ends.
func newFakeSMTPServerRefusingLogins(t *testing.T) *fakeSMTPServer {
	return startFakeSMTPServer(t, &fakeSMTPServer{offerAuth: true, refuseAuth: true})
}

// newFakeSMTPServerWithLoginAuth starts a fakeSMTPServer that offers only
// AUTH LOGIN and stops when the test ends.
func newFakeSMTPServerWithLoginAuth(t *testing.T) *fakeSMTPServer {
	return startFakeSMTPServer(t, &fakeSMTPServer{offerAuth: true, authMechanisms: "LOGIN"})
}

// startFakeSMTPServer starts server on a loopback port and sets its Addr.
func startFakeSMTPServer(t *testing.T, server *fakeSMTPServer) *fakeSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	server.Addr = listener.Addr().String()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go server.serve(conn)
		}
	}()
	return server
}

// Messages returns the data of every message the server accepted.
func (s *fakeSMTPServer) Messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.messages...)
}

// Logins returns the "username:password" of every login the server accepted
// with AUTH LOGIN.
func (s *fakeSMTPServer) Logins() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.logins...)
}

// Rcpts returns the line of every RCPT command the server received, such as
// "RCPT TO:<recipient@example.com>", in order.
func (s *fakeSMTPServer) Rcpts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.rcpts...)
}

// Verbs returns the verb of every command the server received, such as
// EHLO, AUTH or MAIL, in order.
func (s *fakeSMTPServer) Verbs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.verbs...)
}

// serve answers one connection. It never advertises STARTTLS, so the client
// talks in plain text, and advertises AUTH only when offerAuth is set.
func (s *fakeSMTPServer) serve(conn net.Conn) {
	defer conn.Close()
	text := textproto.NewConn(conn)
	reply := func(line string) bool { return text.PrintfLine("%s", line) == nil }

	if !reply("220 127.0.0.1 ESMTP fake") {
		return
	}
	for {
		line, err := text.ReadLine()
		if err != nil {
			return
		}
		verb := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		s.mu.Lock()
		s.verbs = append(s.verbs, verb)
		s.mu.Unlock()
		if verb == s.closeOn {
			return
		}
		if verb == s.resetOn {
			// Closing with no linger time sends a reset, not an end of file
			if tcp, ok := conn.(*net.TCPConn); ok {
				_ = tcp.SetLinger(0)
			}
			return
		}
		if verb == s.silentOn {
			_, _ = io.Copy(io.Discard, conn)
			return
		}
		switch verb {
		case "EHLO", "HELO":
			if s.offerAuth {
				mechanisms := s.authMechanisms
				if mechanisms == "" {
					mechanisms = "PLAIN"
				}
				reply("250-127.0.0.1")
				reply("250 AUTH " + mechanisms)
			} else {
				reply("250 127.0.0.1")
			}
		case "AUTH":
			switch {
			case s.refuseAuth:
				reply(fakeSMTPLoginRefusal)
			case strings.EqualFold(line, "AUTH LOGIN"):
				s.serveLogin(text)
			default:
				reply("235 Authentication successful")
			}
		case "DATA":
			if !reply("354 End data with <CR><LF>.<CR><LF>") {
				return
			}
			data, err := text.ReadDotBytes()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.messages = append(s.messages, string(data))
			s.mu.Unlock()
			if s.closeOn == "." {
				return
			}
			if s.silentOn == "." {
				_, _ = io.Copy(io.Discard, conn)
				return
			}
			reply("250 OK: queued")
		case "RCPT":
			s.mu.Lock()
			s.rcpts = append(s.rcpts, line)
			s.mu.Unlock()
			if s.refuseRcpt || (s.refuseRcptTo != "" && strings.Contains(line, "<"+s.refuseRcptTo+">")) {
				reply(fakeSMTPRcptRefusal)
			} else {
				reply("250 OK")
			}
		case "QUIT":
			reply("221 Bye")
			return
		default:
			// MAIL, RSET and NOOP
			reply("250 OK")
		}
	}
}

// serveLogin answers AUTH LOGIN: it asks for the username and the password,
// keeps them and accepts the login.
func (s *fakeSMTPServer) serveLogin(text *textproto.Conn) {
	var answers []string
	for _, challenge := range []string{"Username:", "Password:"} {
		if text.PrintfLine("334 %s", base64.StdEncoding.EncodeToString([]byte(challenge))) != nil {
			return
		}
		line, err := text.ReadLine()
		if err != nil {
			return
		}
		answer, err := base64.StdEncoding.DecodeString(line)
		if err != nil {
			_ = text.PrintfLine("501 bad encoding")
			return
		}
		answers = append(answers, string(answer))
	}
	s.mu.Lock()
	s.logins = append(s.logins, strings.Join(answers, ":"))
	s.mu.Unlock()
	_ = text.PrintfLine("235 Authentication successful")
}
