//go:build !unix

package cmd

// ignoreSIGPIPE does nothing: only Unix systems end a process that writes to
// a closed pipe.
func ignoreSIGPIPE() {}
