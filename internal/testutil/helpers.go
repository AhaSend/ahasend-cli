// Package testutil provides testing utilities and helpers for the AhaSend CLI.
//
// This package contains common testing utilities used across the CLI test suite:
//
//   - Command execution helpers with isolation support
//   - Test fixtures for consistent test data
//   - Output capture utilities for testing command output
//   - Cobra command factory patterns for test isolation
//   - Common test patterns and helper functions
//
// The utilities in this package help ensure consistent testing patterns
// and prevent test contamination by providing proper isolation mechanisms.
package testutil

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// CaptureOutput captures stdout and stderr during command execution
func CaptureOutput(t *testing.T, fn func()) (string, string) {
	// Capture stdout
	oldStdout := os.Stdout
	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = stdoutWriter

	// Capture stderr
	oldStderr := os.Stderr
	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = stderrWriter

	// Channels to receive output
	stdoutC := make(chan string)
	stderrC := make(chan string)

	// Read stdout
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutReader)
		stdoutC <- buf.String()
	}()

	// Read stderr
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrReader)
		stderrC <- buf.String()
	}()

	// Execute function
	fn()

	// Restore stdout and stderr
	stdoutWriter.Close()
	stderrWriter.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	// Get output
	stdout := <-stdoutC
	stderr := <-stderrC

	return stdout, stderr
}

// CreateTempConfigDir creates a temporary config directory for testing
func CreateTempConfigDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "ahasend-test-*")
	require.NoError(t, err)

	configDir := filepath.Join(tempDir, ".ahasend")
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return configDir
}

// ExecuteCommand executes a cobra command with the given arguments
func ExecuteCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()
	output = buf.String()

	// Reset for next test
	root.SetArgs([]string{})

	return output, err
}

// ExecuteCommandIsolated executes a command with complete isolation
// This creates a fresh command instance to avoid state contamination between tests
func ExecuteCommandIsolated(t *testing.T, commandFactory func() *cobra.Command, args ...string) (output string, err error) {
	// Create fresh command instance
	root := commandFactory()

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()
	output = buf.String()

	return output, err
}
