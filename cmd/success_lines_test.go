package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// itemViewFiles are the files of the commands whose output shows the item.
var itemViewFiles = []string{
	"create.go", "get.go", "list.go", "update.go", "suspend.go", "unsuspend.go", "unpause_domain.go",
	"duplicate.go", "unschedule.go", "publish.go", "restore.go", "discard.go", "set.go",
}

// TestCreateGetListAndUpdateSuccessLinesNameNoItem checks the success line
// rule: commands whose output shows the item, such as create, get, list,
// update and suspend, print a fixed line, while delete-style commands name
// the item.
func TestCreateGetListAndUpdateSuccessLinesNameNoItem(t *testing.T) {
	fset := token.NewFileSet()
	for _, file := range parseSources(t, fset, "groups") {
		name := filepath.Base(fset.Position(file.Pos()).Filename)
		if !slices.Contains(itemViewFiles, name) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			pair, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			if key, ok := pair.Key.(*ast.Ident); ok && key.Name == "SuccessMessage" {
				_, fixed := stringLiteral(pair.Value)
				assert.True(t, fixed, "%s: the success line names the item", fset.Position(pair.Pos()))
			}
			return true
		})
	}
}

// parseSources parses the non-test Go files under dir.
func parseSources(t *testing.T, fset *token.FileSet, dir string) []*ast.File {
	t.Helper()
	var files []*ast.File
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		files = append(files, file)
		return err
	})
	require.NoError(t, err)
	return files
}

// stringLiteral returns the value of a string literal.
func stringLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}
