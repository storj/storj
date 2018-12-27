package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/types"

	"golang.org/x/tools/go/packages"
)

type Methods interface {
	Method(i int) *types.Func
	NumMethods() int
}

func main() {
	flag.Parse()

	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax,
	}

	roots, err := packages.Load(cfg, "storj.io/storj/satellite")
	if err != nil {
		panic(err)
	}

	dbObject := roots[0].Types.Scope().Lookup("DB")
	db := dbObject.Type().Underlying().(Methods)

	imports := map[string]bool{
		"sync":                true,
		"storj.io/statellite": true,
	}

	var mutexDecl bytes.Buffer
	var wrapperDecl bytes.Buffer

	fmt.Fprintf(&mutexDecl, "type Mutex struct {\n")
	fmt.Fprintf(&mutexDecl, "	mu sync.Mutex\n")
	fmt.Fprintf(&mutexDecl, "	db satellite.DB\n")
	fmt.Fprintf(&mutexDecl, "}\n\n")

	for methodIndex := 0; methodIndex < db.NumMethods(); methodIndex++ {
		method := db.Method(methodIndex)
		if writeWrappingFunc("Mutex", imports, &mutexDecl, method, true) {
			writeWrapperDecl(imports, &wrapperDecl, method)
		}
	}

	fmt.Println(imports)
	fmt.Println(mutexDecl.String())
	fmt.Println(wrapperDecl.String())
}

var ignoreTypes = map[string]bool{
	"error": true,
}

func includeSignatureImports(imports map[string]bool, sig *types.Signature) {
	var tmp bytes.Buffer
	types.WriteSignature(&tmp, sig, func(p *types.Package) string {
		imports[p.Path()] = true
		return p.Name()
	})
}

func writeWrappingFunc(receiver string, imports map[string]bool, body *bytes.Buffer, method *types.Func, nested bool) bool {
	sig := method.Type().Underlying().(*types.Signature)
	includeSignatureImports(imports, sig)

	needsWrapper := nested && sig.Results().Len() == 1 && !ignoreTypes[sig.Results().At(0).Type().String()]

	fmt.Fprintf(body, "func (mu *%s) %s", receiver, method.Name())
	types.WriteSignature(body, sig, (*types.Package).Name)
	fmt.Fprintf(body, " {\n")
	if !needsWrapper {
		fmt.Fprintf(body, "\tmu.mu.Lock(); defer mu.mu.Unlock()\n")
	}
	fmt.Fprintf(body, "\treturn ")
	if needsWrapper {
		fmt.Fprintf(body, "&mu%s{mu:&mu.mu, db:", method.Name())
	}

	fmt.Fprintf(body, "mu.db.%s", method.Name())
	writeCallSignature(body, sig)
	if needsWrapper {
		fmt.Fprintf(body, "}")
	}
	fmt.Fprintf(body, "\n}\n\n")

	return needsWrapper
}

func writeCallSignature(body *bytes.Buffer, sig *types.Signature) {
	fmt.Fprintf(body, "(")
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		if i != 0 {
			fmt.Fprintf(body, ", ")
		}
		fmt.Fprintf(body, "%s", params.At(i).Name())
	}
	fmt.Fprintf(body, ")")
}

func writeWrapperDecl(imports map[string]bool, body *bytes.Buffer, method *types.Func) {
	sig := method.Type().Underlying().(*types.Signature)
	results := sig.Results()
	result := results.At(0).Type()

	recvName := fmt.Sprintf("mu%s", method.Name())

	fmt.Fprintf(body, "// %s implements locking wrapper for %s\n", recvName, typeName(result))
	fmt.Fprintf(body, "type %s struct {\n", recvName)
	fmt.Fprintf(body, "\tmu *sync.Mutex\n")
	fmt.Fprintf(body, "\tdb %s\n", typeName(result))
	fmt.Fprintf(body, "}\n\n")

	methodSet := result.Underlying().(Methods)
	for i := 0; i < methodSet.NumMethods(); i++ {
		method := methodSet.Method(i)
		writeWrappingFunc(recvName, imports, body, method, false)
	}
}

func typeName(typ types.Type) string {
	var body bytes.Buffer
	types.WriteType(&body, typ, (*types.Package).Name)
	return body.String()
}
