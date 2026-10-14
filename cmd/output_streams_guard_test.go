package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandsWriteOnlyToTheCommandStreams fails when command code writes to
// the process's stdout or stderr directly: a Print, Printf or Println call on
// any receiver (fmt, a color, a command) or a use of os.Stdout or os.Stderr.
// Commands write through cmd.OutOrStdout() and cmd.ErrOrStderr(), so tests
// capture everything and JSON output carries nothing but the response.
func TestCommandsWriteOnlyToTheCommandStreams(t *testing.T) {
	var found []string
	for _, root := range []string{".", "../internal/dns"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			found = append(found, directWrites(t, path)...)
			return nil
		})
		require.NoError(t, err)
	}

	assert.Empty(t, found, "direct writes to the process's stdout or stderr")
}

// directWrites returns the position of every direct write in the file at path.
func directWrites(t *testing.T, path string) []string {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	require.NoError(t, err)

	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "Print", "Printf", "Println":
			found = append(found, fileSet.Position(selector.Pos()).String())
		case "Stdout", "Stderr":
			if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "os" {
				found = append(found, fileSet.Position(selector.Pos()).String())
			}
		}
		return true
	})
	return found
}
