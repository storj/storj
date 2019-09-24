// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"sort"
	"strings"

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

	var lockedFnNames []string
	for _, pkg := range pkgs {
		_lockedFnNames := findLockedFnNames(pkg)
		lockedFnNames = append(lockedFnNames, _lockedFnNames...)
	}
	sortedNames := sortAndUnique(lockedFnNames)

	outputStr := strings.Join(sortedNames, "\n")
	if len(os.Args) == 2 {
		ioutil.WriteFile(os.Args[1], []byte(outputStr+"\n"), 0644)
	} else {
		fmt.Println(outputStr)
	}
}

func findLockedFnNames(pkg *packages.Package) []string {
	var (
		lockedCalls   []token.Pos
		lockedFns     []*ast.FuncDecl
		lockedFnNames []string
		lockedLines   = make(map[int]struct{})
	)

	// collect locked comments and what line they are on
	for _, file := range pkg.Syntax {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if comment.Text == "// locked" {
					commentLine := pkg.Fset.Position(comment.Pos()).Line
					lockedLines[commentLine] = struct{}{}
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
			varType, ok := pkg.TypesInfo.Uses[ident].(*types.Var)
			if !ok {
				return true
			}
			pointerType, ok := varType.Type().(*types.Pointer)
			if !ok {
				return true
			}
			namedType, ok := pointerType.Elem().(*types.Named)
			if !ok {
				return true
			}

			callLine := pkg.Fset.Position(node.End()).Line
			if namedType.Obj().Pkg().Path() == monkitpath && sel.Sel.Name == "Task" {
				if _, ok := lockedLines[callLine]; ok {
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
		for _, fn := range lockedFns {
			object := pkg.TypesInfo.Defs[fn.Name]

			var receiver string
			if fn.Recv != nil {
				typ := fn.Recv.List[0].Type
				if star, ok := typ.(*ast.StarExpr); ok {
					receiver = ".*"
					typ = star.X
				} else {
					receiver = "."
				}
				recvObj := pkg.TypesInfo.Uses[typ.(*ast.Ident)]
				receiver += recvObj.Name()
			}

			lockedFnInfo := object.Pkg().Path() + receiver + "." + object.Name()
			lockedFnNames = append(lockedFnNames, lockedFnInfo)

		}
	}
	return lockedFnNames
}

func sortAndUnique(input []string) (unique []string) {
	sort.Strings(input)
	set := make(map[string]struct{})
	for _, item := range input {
		if _, ok := set[item]; ok {
			continue
		} else {
			set[item] = struct{}{}
		}
	}
	for item := range set {
		unique = append(unique, item)
	}
	return unique
}
