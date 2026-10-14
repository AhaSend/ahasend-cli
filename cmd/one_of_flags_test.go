package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneOfReaders are the cmdutil functions that read a flag added by
// cmdutil.AddOneOfFlag, by the name they are given.
var oneOfReaders = map[string]bool{"MatchOneOf": true, "ReadOneOf": true, "ReadFilterOneOf": true}

// oneOfRead is a call of one of the oneOfReaders: the flag name it reads,
// the directory of its source file and where in it the call is.
type oneOfRead struct {
	name, dir, where string
}

// oneOfReads returns the calls of the oneOfReaders in the CLI's source.
// Every call names its flag with a string literal, which the test can read,
// except the readers' own calls of each other in enum.go, which pass the
// name they were given.
func oneOfReads(t *testing.T) []oneOfRead {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	var reads []oneOfRead
	fset := token.NewFileSet()
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !oneOfReaders[calledName(call)] || len(call.Args) < 2 {
					return true
				}
				where := fset.Position(call.Pos()).String()
				literal, ok := call.Args[1].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					assert.True(t, strings.HasSuffix(path, filepath.Join("internal", "cmdutil", "enum.go")),
						"%s: name the flag with a string literal, so this test can check it", where)
					return true
				}
				name, err := strconv.Unquote(literal.Value)
				require.NoError(t, err, where)
				reads = append(reads, oneOfRead{name: name, dir: filepath.Dir(path), where: where})
				return true
			})
			return nil
		})
		require.NoError(t, err)
	}
	return reads
}

// calledName returns the name of the function call calls, without its
// package, such as MatchOneOf for cmdutil.MatchOneOf.
func calledName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// sourceDir returns the directory of the source file of the function that
// runs command, or "" when command does not run, such as a command group.
func sourceDir(command *cobra.Command) string {
	if command.RunE == nil {
		return ""
	}
	function := runtime.FuncForPC(reflect.ValueOf(command.RunE).Pointer())
	file, _ := function.FileLine(function.Entry())
	return filepath.Dir(file)
}

// TestEveryOneOfReadNamesAFlagWithValues checks that every flag name the
// source passes to MatchOneOf, ReadOneOf or ReadFilterOneOf is a flag added
// by AddOneOfFlag, with the values it is checked against, on the commands
// that can make that call: those of the command group whose source makes it,
// or, for a call in cmdutil, every command with a flag of that name. A
// reader given another flag fails every run of its command.
func TestEveryOneOfReadNamesAFlagWithValues(t *testing.T) {
	reads := oneOfReads(t)
	require.NotEmpty(t, reads)
	for _, read := range reads {
		var commands, withoutValues []string
		walkCommands(NewRootCmdForTesting(), func(command *cobra.Command) {
			flag := command.LocalFlags().Lookup(read.name)
			if flag == nil {
				return
			}
			if filepath.Base(read.dir) != "cmdutil" && sourceDir(command) != read.dir {
				return
			}
			commands = append(commands, command.CommandPath())
			if len(flag.Annotations["ahasend_one_of"]) == 0 {
				withoutValues = append(withoutValues, command.CommandPath())
			}
		})

		assert.NotEmpty(t, commands, "%s: --%s is no flag of a command that can read it", read.where, read.name)
		assert.Empty(t, withoutValues, "%s: --%s has no values on these commands", read.where, read.name)
	}
}
