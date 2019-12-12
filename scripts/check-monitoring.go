// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate go run check-monitoring.go ../monkit.lock

// +build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

var (
	monkitPath    = "gopkg.in/spacemonkeygo/monkit.v2"
	lockFilePerms = os.FileMode(0644)
)

func main() {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo,
	}, "storj.io/storj/...")
	if err != nil {
		log.Fatalf("error while loading packages: %s", err)
	}

	var lockedFnNames []string
	for _, pkg := range pkgs {
		lockedFnNames = append(lockedFnNames, findLockedFnNames(pkg)...)
	}
	sortedNames := sortAndUnique(lockedFnNames)

	outputStr := strings.Join(sortedNames, "\n")
	if len(os.Args) == 2 {
		file := os.Args[1]
		if err := ioutil.WriteFile(file, []byte(outputStr+"\n"), lockFilePerms); err != nil {
			log.Fatalf("error while writing to file \"%s\": %s", file, err)
		}
	} else {
		fmt.Println(outputStr)
	}
}

func findLockedFnNames(pkg *packages.Package) []string {
	var (
		lockedTasksPos []token.Pos
		lockedTaskFns  []*ast.FuncDecl
		lockedFnInfos  []string
	)

	// Collect locked comments and what line they are on.
	for _, file := range pkg.Syntax {
		lockedLines := make(map[int]struct{})
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if comment.Text == "//locked" {
					commentLine := pkg.Fset.Position(comment.Pos()).Line
					lockedLines[commentLine] = struct{}{}
				}
			}
		}
		if len(lockedLines) == 0 {
			continue
		}

		// Find calls to monkit functions we're interested in that are on the
		// same line as a "locked" comment and keep track of their position.
		// NB: always return true to walk entire node tree.
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				return true
			}
			if !isMonkitCall(pkg, node) {
				return true
			}

			// Ensure call line matches a "locked" comment line.
			callLine := pkg.Fset.Position(node.End()).Line
			if _, ok := lockedLines[callLine]; !ok {
				return true
			}

			// We are already checking to ensure that these type assertions are valid in `isMonkitCall`.
			sel := node.(*ast.CallExpr).Fun.(*ast.SelectorExpr)

			// Track `mon.Task` calls.
			if sel.Sel.Name == "Task" {
				lockedTasksPos = append(lockedTasksPos, node.End())
				return true
			}

			// Track other monkit calls that have one string argument (e.g. monkit.FloatVal, etc.)
			// and transform them to representative string.
			if len(node.(*ast.CallExpr).Args) != 1 {
				return true
			}
			argLiteral, ok := node.(*ast.CallExpr).Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			if argLiteral.Kind == token.STRING {
				lockedFnInfo := pkg.PkgPath + "." + argLiteral.Value + " " + sel.Sel.Name
				lockedFnInfos = append(lockedFnInfos, lockedFnInfo)
			}
			return true
		})

		// Track all function declarations containing locked `mon.Task` calls.
		ast.Inspect(file, func(node ast.Node) bool {
			fn, ok := node.(*ast.FuncDecl)
			if !ok {
				return true
			}
			for _, locked := range lockedTasksPos {
				if fn.Pos() < locked && locked < fn.End() {
					lockedTaskFns = append(lockedTaskFns, fn)
				}
			}
			return true
		})

	}

	// Transform the ast.FuncDecls containing locked `mon.Task` calls to representative string.
	for _, fn := range lockedTaskFns {
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

		lockedFnInfo := object.Pkg().Path() + receiver + "." + object.Name() + " Task"
		lockedFnInfos = append(lockedFnInfos, lockedFnInfo)

	}
	return lockedFnInfos
}

// isMonkitCall returns whether the node is a call to a function in the monkit package.
func isMonkitCall(pkg *packages.Package, in ast.Node) bool {
	defer func() { recover() }()

	ident := in.(*ast.CallExpr).Fun.(*ast.SelectorExpr).X.(*ast.Ident)

	return pkg.TypesInfo.Uses[ident].(*types.Var).Type().(*types.Pointer).Elem().(*types.Named).Obj().Pkg().Path() == monkitPath
}

func sortAndUnique(input []string) (unique []string) {
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
	sort.Strings(unique)
	return unique
}
