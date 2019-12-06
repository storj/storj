// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"

	"golang.org/x/tools/go/packages"
)

// holds on to the eventual exit code
var exit int

func main() {
	// load up the requested packages
	pkgs, err := packages.Load(&packages.Config{
		Mode: 0 |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes |
			packages.NeedSyntax |
			packages.NeedImports |
			packages.NeedName,
	}, os.Args[1:]...)
	if err != nil {
		log.Fatal(err)
	}

	// check all of their atomic alignment
	for _, pkg := range pkgs {
		for _, arg := range gatherAtomicArguments(pkg) {
			checkArgument(pkg, arg)
		}
	}

	// exit with the correct code
	os.Exit(exit)
}

// gatherAtomicArguments looks for calls to 64bit atomics and gathers their first
// argument as an ast expression.
func gatherAtomicArguments(pkg *packages.Package) (args []ast.Expr) {
	for _, file := range pkg.Syntax {
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
			name, ok := pkg.TypesInfo.Uses[ident].(*types.PkgName)
			if !ok || name.Imported().Path() != "sync/atomic" {
				return true
			}
			switch sel.Sel.Name {
			case "AddInt64", "AddUint64", "LoadInt64", "LoadUint64",
				"StoreInt64", "StoreUint64", "SwapInt64", "SwapUint64",
				"CompareAndSwapInt64", "CompareAndSwapUint64":
				args = append(args, call.Args[0])
			}
			return true
		})
	}
	return args
}

// checkArgument makes sure that the ast expression is not an address of some field
// access into a struct that is not 64 bit aligned.
func checkArgument(pkg *packages.Package, arg ast.Expr) {
	// ensure the expression is an address of expression
	unary, ok := arg.(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return
	}

	// gather the fields through the whole selection (&foo.bar.baz)
	var fields []*types.Var
	var root types.Type
	var x = unary.X
	for {
		sel, ok := x.(*ast.SelectorExpr)
		if !ok {
			break
		}
		field, ok := pkg.TypesInfo.Selections[sel].Obj().(*types.Var)
		if !ok || !field.IsField() {
			return
		}
		fields = append(fields, field)
		root = pkg.TypesInfo.Types[sel.X].Type
		x = sel.X
	}
	if len(fields) == 0 {
		return
	}

	// walk in reverse keeping track of the indicies walked through
	// this helps deal with embedded fields, etc.
	var indicies []int
	for base := root; len(fields) > 0; fields = fields[:len(fields)-1] {
		obj, index, _ := types.LookupFieldOrMethod(base, true, pkg.Types, fields[len(fields)-1].Name())

		field, ok := obj.(*types.Var)
		if !ok {
			return
		}
		base = field.Type()

		indicies = append(indicies, index...)
	}

	// derefrence the root to start off at the base struct
	base, _, ok := deref(root)
	if !ok {
		return
	}

	// now walk forward keeping track of offsets and indirections
	var offset int64
	var sizes = types.SizesFor("gc", "arm")
	for _, index := range indicies {
		// get the next field type and keep track of if it was a pointer. if so
		// then we need to reset our offset (it's guaranteed 64 bit aligned).
		next, wasPtr, ok := deref(base.Field(index).Type())
		if wasPtr {
			offset = 0
		} else {
			offset += sizes.Offsetsof(structFields(base))[index]
		}

		// if we're no longer at a struct, we're done walking.
		if !ok {
			break
		}

		base = next
	}

	// check if the offset is aligned
	if offset&7 == 0 {
		return
	}

	// report an error and update the status code
	file := pkg.Fset.File(arg.Pos())
	line := file.Line(arg.Pos())

	fmt.Fprintf(os.Stderr,
		"%s:%d: address of non 64-bit aligned field passed to atomic (offset: %d)\n",
		file.Name(), line, offset)
	exit = 1
}

// deref takes a type that can be
// 1. an unnamed struct
// 2. a named struct
// 3. a pointer to an unnamed struct
// 4. a pointer to a named struct
// and returns the unnamed struct as well as if it was a pointer.
func deref(in types.Type) (*types.Struct, bool, bool) {
	wasPtr := false
	if ptr, ok := in.(*types.Pointer); ok {
		wasPtr = true
		in = ptr.Elem()
	}
	if named, ok := in.(*types.Named); ok {
		in = named.Underlying()
	}
	out, ok := in.(*types.Struct)
	return out, wasPtr, ok
}

// structFields gathers all of the fields of the passed in struct.
func structFields(in *types.Struct) []*types.Var {
	out := make([]*types.Var, in.NumFields())
	for i := range out {
		out[i] = in.Field(i)
	}
	return out
}
