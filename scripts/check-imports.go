// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

/*
check-imports verifies whether imports are divided into three blocks:

	std packages
	external packages
	storj.io packages

*/

var race = flag.Bool("race", false, "load with race tag")

func main() {
	flag.Parse()

	pkgNames := flag.Args()
	if len(pkgNames) == 0 {
		pkgNames = []string{"."}
	}

	var buildFlags []string
	if *race {
		buildFlags = append(buildFlags, "-race")
	}

	roots, err := packages.Load(&packages.Config{
		Mode:       packages.LoadAllSyntax,
		Env:        os.Environ(),
		BuildFlags: buildFlags,
		Tests:      true,
	}, pkgNames...)

	if err != nil {
		panic(err)
	}

	fmt.Println("checking import order:")

	// load all packages
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

	// sort the packages
	sort.Slice(pkgs, func(i, k int) bool { return pkgs[i].ID < pkgs[k].ID })

	var misgrouped, unsorted []Imports
	for _, pkg := range pkgs {
		pkgmisgrouped, pkgunsorted := verifyPackage(os.Stderr, pkg)

		misgrouped = append(misgrouped, pkgmisgrouped...)
		unsorted = append(unsorted, pkgunsorted...)
	}

	exitCode := 0
	if len(misgrouped) > 0 {
		exitCode = 1

		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Imports are not in the standard grouping [std storj other]:")
		for _, imports := range misgrouped {
			fmt.Fprintln(os.Stderr, "\t"+imports.Path, imports.Classes())
		}
	}

	if len(unsorted) > 0 {
		exitCode = 1

		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Imports are not sorted:")
		for _, imports := range unsorted {
			fmt.Fprintln(os.Stderr, "\t"+imports.Path)
		}
	}

	os.Exit(exitCode)
}

func verifyPackage(stderr io.Writer, pkg *packages.Package) (misgrouped, unsorted []Imports) {
	// ignore generated test binaries
	if strings.HasSuffix(pkg.ID, ".test") {
		return
	}

	for i, file := range pkg.Syntax {
		path := pkg.CompiledGoFiles[i]

		imports := LoadImports(pkg.Fset, path, file)

		ordered := true
		sorted := true
		for _, section := range imports.Decls {
			if !section.IsGrouped() {
				ordered = false
			}
			if !section.IsSorted() {
				sorted = false
			}
		}

		if !ordered || !sorted {
			if isGenerated(path) {
				fmt.Fprintln(stderr, "(ignoring generated)", path)
				continue
			}
		}

		if !ordered {
			misgrouped = append(misgrouped, imports)
		}
		if !sorted {
			unsorted = append(unsorted, imports)
		}
	}

	return
}

// Imports defines all imports for a single file.
type Imports struct {
	Path      string
	Generated bool
	Decls     []ImportDecl
}

// Classes returns all import groupings
func (imports Imports) Classes() [][]Class {
	var classes [][]Class
	for _, decl := range imports.Decls {
		classes = append(classes, decl.Classes())
	}
	return classes
}

// ImportDecl defines a single import declaration
type ImportDecl []ImportGroup

// allowedGroups lists all valid groupings
var allowedGroups = [][]Class{
	{Standard},
	{Storj},
	{Other},
	{Standard, Storj},
	{Standard, Other},
	{Other, Storj},
	{Standard, Other, Storj},
}

// IsGrouped returns whether the grouping is allowed.
func (decls ImportDecl) IsGrouped() bool {
	classes := decls.Classes()
	for _, allowedGroup := range allowedGroups {
		if reflect.DeepEqual(allowedGroup, classes) {
			return true
		}
	}
	return false
}

// Classes returns each group class.
func (decl ImportDecl) Classes() []Class {
	classes := make([]Class, len(decl))
	for i := range classes {
		classes[i] = decl[i].Class()
	}
	return classes
}

// IsSorted returns whether the group is sorted.
func (decls ImportDecl) IsSorted() bool {
	for _, decl := range decls {
		if !decl.IsSorted() {
			return false
		}
	}
	return true
}

// ImportGroup defines a single import statement.
type ImportGroup struct {
	Specs []*ast.ImportSpec
	Paths []string
}

// IsSorted returns whether the group is sorted.
func (group ImportGroup) IsSorted() bool {
	return sort.StringsAreSorted(group.Paths)
}

// Class returns the classification of this import group.
func (group ImportGroup) Class() Class {
	var class Class
	for _, path := range group.Paths {
		class |= ClassifyImport(path)
	}
	return class
}

// Class defines a bitset of import classification
type Class byte

// Class defines three different groups
const (
	// Standard is all go standard packages
	Standard Class = 1 << iota
	// Storj is imports that start with `storj.io`
	Storj
	// Other is everything else
	Other
)

// ClassifyImport classifies an import path to a class.
func ClassifyImport(pkgPath string) Class {
	if strings.HasPrefix(pkgPath, "storj.io/") {
		return Storj
	}
	if stdlib[pkgPath] {
		return Standard
	}
	return Other
}

// String returns contents of the class.
func (class Class) String() string {
	var s []string
	if class&Standard != 0 {
		s = append(s, "std")
	}
	if class&Storj != 0 {
		s = append(s, "storj")
	}
	if class&Other != 0 {
		s = append(s, "other")
	}
	return strings.Join(s, "|")
}

// LoadImports loads import groups from a given fileset.
func LoadImports(fset *token.FileSet, name string, f *ast.File) Imports {
	var imports Imports
	imports.Path = name

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

		// identify specs on successive lines
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

		// convert ast.Spec-s groups into import groups
		var decl ImportDecl
		for _, specgroup := range specgroups {
			var group ImportGroup
			for _, importSpec := range specgroup {
				importSpec := importSpec.(*ast.ImportSpec)
				path, err := strconv.Unquote(importSpec.Path.Value)
				if err != nil {
					panic(err)
				}
				group.Specs = append(group.Specs, importSpec)
				group.Paths = append(group.Paths, path)
			}
			decl = append(decl, group)
		}

		imports.Decls = append(imports.Decls, decl)
	}

	return imports
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

func isGenerated(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read %v: %v\n", path, err)
		return false
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	var header [256]byte
	n, err := file.Read(header[:])
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "failed to read %v: %v\n", path, err)
		return false
	}

	return bytes.Contains(header[:n], []byte(`AUTOGENERATED`)) ||
		bytes.Contains(header[:n], []byte(`Code generated`))
}
