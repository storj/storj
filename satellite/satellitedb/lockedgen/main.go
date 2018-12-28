// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/ioutil"
	"sort"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

func main() {
	var outputPath string
	flag.StringVar(&outputPath, "o", "", "output file name")
	flag.Parse()

	var code Code

	code.Imports = map[string]bool{}
	code.Ignore = map[string]bool{
		"error": true,
	}

	code.Config = &packages.Config{
		Mode: packages.LoadAllSyntax,
	}
	code.Package = "storj.io/storj/satellite"

	var err error
	code.Roots, err = packages.Load(code.Config, code.Package)
	if err != nil {
		panic(err)
	}

	code.PrintLocked()
	code.PrintPreamble()

	unformatted := code.Bytes()

	imports.LocalPrefix = "storj.io"
	formatted, err := imports.Process(outputPath, unformatted, nil)
	if err != nil {
		fmt.Println(string(unformatted))
		panic(err)
	}

	if outputPath == "" {
		fmt.Println(string(formatted))
		return
	}

	err = ioutil.WriteFile(outputPath, formatted, 0644)
	if err != nil {
		panic(err)
	}
}

// Methods is the common interface for types having methods.
type Methods interface {
	Method(i int) *types.Func
	NumMethods() int
}

// Code is the information for generating the code.
type Code struct {
	Config  *packages.Config
	Package string
	Roots   []*packages.Package

	Imports map[string]bool
	Ignore  map[string]bool

	Preamble bytes.Buffer
	Source   bytes.Buffer
}

// Bytes returns all code merged together
func (code *Code) Bytes() []byte {
	var all bytes.Buffer
	all.Write(code.Preamble.Bytes())
	all.Write(code.Source.Bytes())
	return all.Bytes()
}

// PrintPreamble creates package header and imports.
func (code *Code) PrintPreamble() {
	w := &code.Preamble
	fmt.Fprintf(w, "// Code generated by lockedgen using 'go generate'. DO NOT EDIT.\n\n")
	fmt.Fprintf(w, "// Copyright (C) 2018 Storj Labs, Inc.\n")
	fmt.Fprintf(w, "// See LICENSE for copying information.\n\n")
	fmt.Fprintf(w, "package satellitedb\n\n")
	fmt.Fprintf(w, "import (\n")

	var imports []string
	for imp := range code.Imports {
		imports = append(imports, imp)
	}
	sort.Strings(imports)
	for _, imp := range imports {
		fmt.Fprintf(w, "	%q\n", imp)
	}
	fmt.Fprintf(w, ")\n\n")
}

// PrintLocked writes locked wrapper and methods.
func (code *Code) PrintLocked() {
	code.Imports["sync"] = true
	code.Imports["storj.io/statellite"] = true

	code.Printf("// Locked implements a locking wrapper around satellite.DB.\n")
	code.Printf("type Locked struct {\n")
	code.Printf("	sync.Locker\n")
	code.Printf("	db satellite.DB\n")
	code.Printf("}\n\n")

	code.Printf("// NewLocked returns database wrapped with locker.\n")
	code.Printf("func NewLocked(db satellite.DB) satellite.DB {\n")
	code.Printf("	return &Locked{&sync.Mutex{}, db}\n")
	code.Printf("}\n\n")

	// find the satellite.DB type info
	dbObject := code.Roots[0].Types.Scope().Lookup("DB")
	methods := dbObject.Type().Underlying().(Methods)

	for i := 0; i < methods.NumMethods(); i++ {
		code.PrintLockedFunc("Locked", methods.Method(i), true)
	}

	for i := 0; i < methods.NumMethods(); i++ {
		if !code.NeedsWrapper(methods.Method(i)) {
			continue
		}
		code.PrintWrapper(methods.Method(i))
	}
}

// Printf writes formatted text to source.
func (code *Code) Printf(format string, a ...interface{}) {
	fmt.Fprintf(&code.Source, format, a...)
}

// PrintSignature prints method signature.
func (code *Code) PrintSignature(sig *types.Signature) {
	code.PrintSignatureTuple(sig.Params(), true)
	if sig.Results().Len() > 0 {
		code.Printf(" ")
		code.PrintSignatureTuple(sig.Results(), false)
	}
}

// PrintSignatureTuple prints method tuple, params or results.
func (code *Code) PrintSignatureTuple(tuple *types.Tuple, needsNames bool) {
	code.Printf("(")
	defer code.Printf(")")

	for i := 0; i < tuple.Len(); i++ {
		if i > 0 {
			code.Printf(", ")
		}

		param := tuple.At(i)
		if code.PrintName(tuple.At(i), i, needsNames) {
			code.Printf(" ")
		}
		code.PrintType(param.Type())
	}
}

// PrintCall prints a call using the specified signature.
func (code *Code) PrintCall(sig *types.Signature) {
	code.Printf("(")
	defer code.Printf(")")

	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		if i != 0 {
			code.Printf(", ")
		}
		code.PrintName(params.At(i), i, true)
	}
}

// PrintName prints an appropriate name from signature tuple.
func (code *Code) PrintName(v *types.Var, index int, needsNames bool) bool {
	name := v.Name()
	if needsNames && name == "" {
		if v.Type().String() == "context.Context" {
			code.Printf("ctx")
			return true
		}
		code.Printf("a%d", index)
		return true
	}
	code.Printf("%s", name)
	return name != ""
}

// PrintType prints short form of type t.
func (code *Code) PrintType(t types.Type) {
	types.WriteType(&code.Source, t, (*types.Package).Name)
}

func typeName(typ types.Type) string {
	var body bytes.Buffer
	types.WriteType(&body, typ, (*types.Package).Name)
	return body.String()
}

// IncludeImports imports all types referenced in the signature.
func (code *Code) IncludeImports(sig *types.Signature) {
	var tmp bytes.Buffer
	types.WriteSignature(&tmp, sig, func(p *types.Package) string {
		code.Imports[p.Path()] = true
		return p.Name()
	})
}

// NeedsWrapper checks whether method result needs a wrapper type.
func (code *Code) NeedsWrapper(method *types.Func) bool {
	sig := method.Type().Underlying().(*types.Signature)
	return sig.Results().Len() == 1 && !code.Ignore[sig.Results().At(0).Type().String()]
}

// WrapperTypeName returns an appropariate name for the wrapper type.
func (code *Code) WrapperTypeName(method *types.Func) string {
	return "locked" + method.Name()
}

// PrintLockedFunc prints a method with locking and defers the actual logic to method.
func (code *Code) PrintLockedFunc(receiverType string, method *types.Func, allowNesting bool) {
	sig := method.Type().Underlying().(*types.Signature)
	code.IncludeImports(sig)

	doc := code.MethodDoc(method)
	if doc != "" {
		code.Printf("// %s", code.MethodDoc(method))
	}
	code.Printf("func (m *%s) %s", receiverType, method.Name())
	code.PrintSignature(sig)
	code.Printf(" {\n")
	defer code.Printf("}\n\n")

	code.Printf("	m.Lock(); defer m.Unlock()\n")
	if code.NeedsWrapper(method) {
		code.Printf("	return &%s{m.lock, ", code.WrapperTypeName(method))
		code.Printf("m.db.%s", method.Name())
		code.PrintCall(sig)
		code.Printf("}\n")
	} else {
		code.Printf("	return m.db.%s", method.Name())
		code.PrintCall(sig)
		code.Printf("\n")
	}
}

// PrintWrapper prints wrapper for the result type of method.
func (code *Code) PrintWrapper(method *types.Func) {
	sig := method.Type().Underlying().(*types.Signature)
	results := sig.Results()
	result := results.At(0).Type()

	receiverType := code.WrapperTypeName(method)
	code.Printf("// %s implements locking wrapper for %s\n", receiverType, typeName(result))
	code.Printf("type %s struct {\n", receiverType)
	code.Printf("	sync.Locker\n")
	code.Printf("	db %s\n", typeName(result))
	code.Printf("}\n\n")

	methods := result.Underlying().(Methods)
	for i := 0; i < methods.NumMethods(); i++ {
		code.PrintLockedFunc(receiverType, methods.Method(i), false)
	}
}

// MethodDoc finds documentation for the specified method.
func (code *Code) MethodDoc(method *types.Func) string {
	file := code.FindASTFile(method.Pos())
	if file == nil {
		return ""
	}

	path, exact := astutil.PathEnclosingInterval(file, method.Pos(), method.Pos())
	if !exact {
		return ""
	}

	for _, p := range path {
		switch decl := p.(type) {
		case *ast.Field:
			return decl.Doc.Text()
		case *ast.GenDecl:
			return decl.Doc.Text()
		case *ast.FuncDecl:
			return decl.Doc.Text()
		}
	}

	return ""
}

// FindASTFile finds the *ast.File at the specified position.
func (code *Code) FindASTFile(pos token.Pos) *ast.File {
	seen := map[*packages.Package]bool{}

	// find searches pos recursively from p and its dependencies.
	var find func(p *packages.Package) *ast.File
	find = func(p *packages.Package) *ast.File {
		if seen[p] {
			return nil
		}
		seen[p] = true

		for _, file := range p.Syntax {
			if file.Pos() <= pos && pos <= file.End() {
				return file
			}
		}

		for _, dep := range p.Imports {
			if file := find(dep); file != nil {
				return file
			}
		}

		return nil
	}

	for _, root := range code.Roots {
		if file := find(root); file != nil {
			return file
		}
	}
	return nil
}
