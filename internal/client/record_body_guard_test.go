package client

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_EverySDKCallKeepsItsResponse fails when a wrapper method assigns
// an SDK call's *http.Response to _, because that response's body is what
// JSON output prints. Wrapper methods should return recordBody(sdkCall(...)).
func TestClient_EverySDKCallKeepsItsResponse(t *testing.T) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "client.go", nil, 0)
	require.NoError(t, err)

	var discarded []string
	ast.Inspect(file, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 3 || len(assign.Rhs) != 1 || !isSDKCall(assign.Rhs[0]) {
			return true
		}
		if response, ok := assign.Lhs[1].(*ast.Ident); ok && response.Name == "_" {
			discarded = append(discarded, fileSet.Position(assign.Pos()).String())
		}
		return true
	})

	assert.Empty(t, discarded, "SDK calls that discard their *http.Response")
}

// isSDKCall reports whether expr is a call such as c.DomainsAPI.GetDomain(...).
func isSDKCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	service, ok := method.X.(*ast.SelectorExpr)
	return ok && strings.HasSuffix(service.Sel.Name, "API")
}
