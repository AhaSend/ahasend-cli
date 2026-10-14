package cmd

import "github.com/spf13/cobra"

// RunForTesting runs args through root the way the binary does, printing
// every error, including Cobra's own, and returns the exit code. Tests outside
// this package use it to check exit codes end to end.
func RunForTesting(root *cobra.Command, args []string) int {
	return run(root, args)
}
