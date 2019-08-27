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

		if fn.FullName() == "github.com/zeebo/errs.Combine" {
			if len(call.Args) == 0 {
				pass.Reportf(call.Lparen, "no arguments for errs.Combine")
			}
			if len(call.Args) == 1 && call.Ellipsis == token.NoPos {
				pass.Reportf(call.Lparen, "remove errs.Combine for one argument")
			}
		}
	})

	return nil, nil
}
