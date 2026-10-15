package integration

import (
	"bytes"
	"strings"

	"github.com/stretchr/testify/suite"

	"github.com/AhaSend/ahasend-cli/cmd"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
)

// commandSuite is the base for suites that drive commands end to end through
// cmd.NewRootCmdForTesting against a MockClient wired in via the auth resolver.
type commandSuite struct {
	suite.Suite
	mockClient *mocks.MockClient
}

func (suite *commandSuite) SetupTest() {
	suite.mockClient = &mocks.MockClient{}
}

// execRoot runs a fresh root command the way the binary does, in the given
// output format with input on stdin, and returns stdout, stderr and the exit
// code.
func execRoot(format, input string, args ...string) (string, string, int) {
	root := cmd.NewRootCmdForTesting()

	var stdout, stderr bytes.Buffer
	root.SetIn(strings.NewReader(input))
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	code := cmd.RunForTesting(root, append(append([]string{}, args...), "--output", format))
	return stdout.String(), stderr.String(), code
}
