package cmd

// Test-support accessors for the package-private globalExitCode.
//
// The exit-code contract is exercised at the cmd-package level in
// root_error_test.go (which can read globalExitCode directly), but the
// out-of-package integration suite must also assert it end-to-end through the
// real Cobra root path (see test/integration/subaccounts_integration_test.go):
// JSON-mode raw API errors are pass-through responses that leave the exit code
// at 0, while table/plain equivalents must remain nonzero. These helpers follow
// the package's existing test-seam convention (NewRootCmdForTesting,
// auth.SetAuthenticatedClientResolverForTesting) and intentionally live in a
// dedicated file so root.go stays read-only context for this task.

// GlobalExitCodeForTesting reports the exit code recorded by the most recent
// command execution. It exists so external test packages can verify the
// JSON-vs-human exit-code contract that handleError implements.
func GlobalExitCodeForTesting() int {
	return globalExitCode
}

// ResetGlobalExitCodeForTesting clears the recorded exit code. External tests
// call it before each command execution so a value left by a prior run cannot
// leak into the assertion.
func ResetGlobalExitCodeForTesting() {
	globalExitCode = 0
}
