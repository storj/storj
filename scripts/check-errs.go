// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/analysis/singlechecker"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

func main() { singlechecker.Main(Analyzer) }

var Analyzer = &analysis.Analyzer{
	Name: "errs",
	Doc:  "check for proper usage of errs package",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
	FactTypes: []analysis.Fact{},
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)
		fn := typeutil.StaticCallee(pass.TypesInfo, call)
		if fn == nil {
			return // not a static call
		}

		switch fn.FullName() {
		case "github.com/zeebo/errs.Combine":
			if len(call.Args) == 0 {
				pass.Reportf(call.Lparen, "errs.Combine() can be simplified to nil")
			}
			if len(call.Args) == 1 && call.Ellipsis == token.NoPos {
				pass.Reportf(call.Lparen, "errs.Combine(x) can be simplified to x")
			}
		case "(*github.com/zeebo/errs.Class).New":
			if len(call.Args) == 0 {
				return
			}
			// Disallow things like Error.New(err.Error())

			switch arg := call.Args[0].(type) {
			case *ast.BasicLit: // allow string constants
			case *ast.Ident: // allow string variables
			default:
				// allow "alpha" + "beta" + "gamma"
				if IsConcatString(arg) {
					return
				}

				pass.Reportf(call.Lparen, "(*errs.Class).New with non-obvious format string")
			}
		}
	})

	return nil, nil
}

func IsConcatString(arg ast.Expr) bool {
	switch arg := arg.(type) {
	case *ast.BasicLit:
		return arg.Kind == token.STRING
	case *ast.BinaryExpr:
		return arg.Op == token.ADD && IsConcatString(arg.X) && IsConcatString(arg.Y)
	default:
		return false
	}
}
