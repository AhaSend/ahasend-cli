//go:build clitest

package client

import "time"

// A CLI binary built with the clitest build tag, as the integration tests
// build it, retries a failed request after a millisecond, so their runs
// against a test API that answers 429, a 5xx status or hangs up do not wait
// through the backoff. Only a build tag can turn this on: the CLI that is
// released is built without it, so no setting a user makes can shorten the
// delays.
func init() {
	retryBaseDelay = time.Millisecond
}
