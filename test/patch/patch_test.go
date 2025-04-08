package patch_test

import (
	"testing"

	"github.com/mvlootman/expr/internal/testify/require"

	"github.com/mvlootman/expr"
	"github.com/mvlootman/expr/ast"
	"github.com/mvlootman/expr/test/mock"
)

type lengthPatcher struct{}

func (p *lengthPatcher) Visit(node *ast.Node) {
	switch n := (*node).(type) {
	case *ast.MemberNode:
		if prop, ok := n.Property.(*ast.StringNode); ok && prop.Value == "length" {
			ast.Patch(
				node, &ast.BuiltinNode{
					Name:      "len",
					Arguments: []ast.Node{n.Node},
				},
			)
		}
	}
}

func TestPatch_length(t *testing.T) {
	program, err := expr.Compile(
		`String.length == 5`,
		expr.Env(mock.Env{}),
		expr.Patch(&lengthPatcher{}),
	)
	require.NoError(t, err)

	env := mock.Env{String: "hello"}
	output, err := expr.Run(program, env)
	require.NoError(t, err)
	require.Equal(t, true, output)
}
