// Package testenv keeps test binaries away from the developer's environment.
//
// Every test package runs its tests through Main:
//
//	func TestMain(m *testing.M) {
//		testenv.Main(m)
//	}
//
// Main points the home directory, HOME and on Windows USERPROFILE, at a new
// empty directory, so tests never read or write the developer's ~/.ahasend,
// and routes every request to a non-loopback host to a closed local port, so
// no test can reach the AhaSend API. Requests to loopback hosts are never
// proxied, so httptest servers keep working. Main also fails every DNS
// lookup, so connections the proxy does not cover, such as the SMTP
// connection of smtp send, reach only IP addresses and the hosts in
// /etc/hosts. A TestMain that cleans up after the tests calls Run instead.
package testenv

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// BlockedProxy is a local port nothing listens on. Every request to a
// non-loopback host goes through it and fails at once.
const BlockedProxy = "http://127.0.0.1:1"

// ErrDNSBlocked is the error of every DNS lookup in a test binary.
var ErrDNSBlocked = errors.New("testenv: DNS lookups are blocked; tests may only connect to loopback servers")

// goDirs are the go command's settings that default to paths under HOME.
// They are pinned before HOME changes, so a test that builds the CLI keeps
// using the developer's module and build caches.
var goDirs = []string{"GOPATH", "GOMODCACHE", "GOCACHE", "GOENV"}

// Main runs the tests of a package in an isolated environment and exits with
// their result.
func Main(m *testing.M) {
	os.Exit(Run(m))
}

// Run is Main without the exit: it returns the exit code of the tests, for a
// TestMain that cleans up after them.
func Run(m *testing.M) int {
	home, err := os.MkdirTemp("", "ahasend-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv: failed to create the test HOME:", err)
		return 1
	}
	defer os.RemoveAll(home)

	if err := isolate(home); err != nil {
		fmt.Fprintln(os.Stderr, "testenv:", err)
		return 1
	}
	blockDNS()
	return m.Run()
}

// blockDNS makes every DNS lookup fail with ErrDNSBlocked. The resolver still
// reads /etc/hosts, so localhost keeps working.
func blockDNS() {
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, ErrDNSBlocked
		},
	}
}

// isolate pins the go command's directories, then points the home directory
// at home, unsets every AHASEND_ variable, so no developer setting reaches a
// test, and routes all non-loopback HTTP to BlockedProxy.
func isolate(home string) error {
	if err := pinGoDirs(); err != nil {
		return err
	}

	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if strings.HasPrefix(name, "AHASEND_") {
			if err := os.Unsetenv(name); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"NO_PROXY", "no_proxy"} {
		if err := os.Unsetenv(name); err != nil {
			return err
		}
	}

	settings := map[string]string{
		"HOME":            home,
		"USERPROFILE":     home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"HTTP_PROXY":      BlockedProxy,
		"HTTPS_PROXY":     BlockedProxy,
	}
	for name, value := range settings {
		if err := os.Setenv(name, value); err != nil {
			return err
		}
	}
	return nil
}

// SetHome points the home directory at dir for the rest of the test: HOME,
// and USERPROFILE, which Windows reads it from.
func SetHome(t testing.TB, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func pinGoDirs() error {
	output, err := exec.Command("go", append([]string{"env"}, goDirs...)...).Output()
	if err != nil {
		return fmt.Errorf("failed to read the go command's directories: %w", err)
	}
	values := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(values) != len(goDirs) {
		return fmt.Errorf("unexpected go env output: %q", output)
	}
	for i, name := range goDirs {
		if err := os.Setenv(name, values[i]); err != nil {
			return err
		}
	}
	return nil
}
