//go:build !unix && !windows

package config

// lockDir does not lock on systems with neither flock nor LockFileEx; two
// commands that change the configuration at once may lose a change there.
func lockDir(string) (unlock func(), err error) {
	return func() {}, nil
}
