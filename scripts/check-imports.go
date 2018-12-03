// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

/*
This tool verifies whether imports are divided into three blocks:

	std packages
	external packages
	storj.io packages

*/

func main() {
	flag.Parse()

	pkgNames := flag.Args()
	if len(pkgNames) == 0 {
		pkgNames = []string{"."}
	}

	roots, err := packages.Load(&packages.Config{
		Mode: packages.LoadAllSyntax,
		Env:  os.Environ(),
	}, pkgNames...)

	if err != nil {
		panic(err)
	}

	seen := map[*packages.Package]bool{}
	pkgs := []*packages.Package{}

	var visit func(*packages.Package)
	visit = func(p *packages.Package) {
		if seen[p] {
			return
		}
		includeStd(p)

		if strings.HasPrefix(p.ID, "storj.io") {
			pkgs = append(pkgs, p)
		}

		seen[p] = true
		for _, pkg := range p.Imports {
			visit(pkg)
		}
	}
	for _, pkg := range roots {
		visit(pkg)
	}

	sort.Slice(pkgs, func(i, k int) bool { return pkgs[i].ID < pkgs[k].ID })
	for _, pkg := range pkgs {
		process(pkg)
	}
}

func process(pkg *packages.Package) {
	for i, file := range pkg.Syntax {
		checkImports(pkg.Fset, pkg.CompiledGoFiles[i], file)
	}
}

func checkImports(fset *token.FileSet, name string, f *ast.File) {
	for _, d := range f.Decls {
		d, ok := d.(*ast.GenDecl)
		if !ok || d.Tok != token.IMPORT {
			// Not an import declaration, so we're done.
			// Imports are always first.
			break
		}

		if !d.Lparen.IsValid() {
			// Not a block: sorted by default.
			continue
		}

		// Identify and sort runs of specs on successive lines.
		lastGroup := 0
		specgroups := [][]ast.Spec{}
		for i, s := range d.Specs {
			if i > lastGroup && fset.Position(s.Pos()).Line > 1+fset.Position(d.Specs[i-1].End()).Line {
				// i begins a new run. End this one.
				specgroups = append(specgroups, d.Specs[lastGroup:i])
				lastGroup = i
			}
		}

		specgroups = append(specgroups, d.Specs[lastGroup:])

		if !correctOrder(specgroups) {
			fmt.Println(name)
		}
	}
}

func correctOrder(specgroups [][]ast.Spec) bool {
	if len(specgroups) == 0 {
		return true
	}

	// remove std group from beginning
	std, other, storj := countGroup(specgroups[0])
	if std > 0 {
		if other+storj != 0 {
			return false
		}
		specgroups = specgroups[1:]
	}
	if len(specgroups) == 0 {
		return true
	}

	// remove storj.io group from the end
	std, other, storj = countGroup(specgroups[len(specgroups)-1])
	if storj > 0 {
		if std+other > 0 {
			return false
		}
		specgroups = specgroups[:len(specgroups)-1]
	}
	if len(specgroups) == 0 {
		return true
	}

	// check that we have a center group for misc stuff
	if len(specgroups) != 1 {
		return false
	}

	std, other, storj = countGroup(specgroups[0])
	return other > 0 && std+storj == 0
}

func countGroup(p []ast.Spec) (std, other, storj int) {
	for _, imp := range p {
		imp := imp.(*ast.ImportSpec)
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			panic(err)
		}
		if strings.HasPrefix(path, "storj.io/") {
			storj++
		} else if stdlib[path] {
			std++
		} else {
			other++
		}
	}
	return std, other, storj
}

var root = runtime.GOROOT()
var stdlib = map[string]bool{}

func includeStd(p *packages.Package) {
	if len(p.GoFiles) == 0 {
		stdlib[p.ID] = true
		return
	}
	if strings.HasPrefix(p.GoFiles[0], root) {
		stdlib[p.ID] = true
		return
	}
}
