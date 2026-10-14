package integration

import (
	"os"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/testenv"
)

func TestMain(m *testing.M) {
	code := testenv.Run(m)
	removeCLIBinary()
	os.Exit(code)
}
