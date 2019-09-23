// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
)

var (
	monkitpath = "gopkg.in/spacemonkeygo/monkit.v2"
)

func main() {
	pkgs, _ := packages.Load(&packages.Config{
		Mode: packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedName |
			packages.NeedFiles | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo,
	}, "storj.io/storj/...")

	for _, pkg := range pkgs {
		findLockedMonTaskCalls(pkg)
	}
}

func findLockedMonTaskCalls(pkg *packages.Package) {
	var (
		lockedCalls = []token.Pos{}
		lockedLines = map[int]struct{}{}
		lockedFns   = []*ast.FuncDecl{}
	)

	// collect locked comments and what line they are on
	for _, file := range pkg.Syntax {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if comment.Text == "// locked" {
					lockedLines[pkg.Fset.Position(comment.Pos()).Line] = struct{}{}
				}
			}
		}

		// find calls to mon.Task() on the same line as a locked comment and keep track of their position
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			specialVar, ok := pkg.TypesInfo.Uses[ident].(*types.Var)
			if !ok {
				return true
			}
			pointerType, ok := specialVar.Type().(*types.Pointer)
			if !ok {
				return true
			}
			namedType, ok := pointerType.Elem().(*types.Named)
			if !ok {
				return true
			}
			if namedType.Obj().Pkg().Path() == monkitpath && sel.Sel.Name == "Task" {
				if _, ok := lockedLines[pkg.Fset.Position(node.End()).Line]; ok {
					lockedCalls = append(lockedCalls, node.End())
				}
			}
			return true
		})

		// find all function declarations and see if they include any locked
		ast.Inspect(file, func(node ast.Node) bool {
			fn, ok := node.(*ast.FuncDecl)
			if !ok {
				return true
			}
			for _, locked := range lockedCalls {
				if fn.Pos() < locked && locked < fn.End() {
					lockedFns = append(lockedFns, fn)
				}
			}
			return true
		})

		// transform the ast.FuncDecl to representative string, sort them, unique them, and output them
		var lockedFnInfos []string
		for _, fn := range lockedFns {
			object := pkg.TypesInfo.Defs[fn.Name]

			var receiver string
			if fn.Recv != nil {
				typ := fn.Recv.List[0].Type
				if star, ok := typ.(*ast.StarExpr); ok {
					typ = star.X
				}
				recvObj := pkg.TypesInfo.Uses[typ.(*ast.Ident)]
				receiver = "." + recvObj.Name()
			}

			lockedFnInfos = append(lockedFnInfos, object.Pkg().Path()+receiver+"."+object.Name())

		}
		for _, info := range lockedFnInfos {
			fmt.Println(info)
		}
	}
}
