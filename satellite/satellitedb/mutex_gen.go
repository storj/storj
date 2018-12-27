//+build ignore

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

	finalPkg := types.NewPackage("storj.io/satellite/satellitedb", "satellitedb")

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

	fmt.Fprintf(&mutexDecl, "type Mutex struct {\n")
	fmt.Fprintf(&mutexDecl, "	db satellite.DB\n")
	fmt.Fprintf(&mutexDecl, "	mu sync.Mutex\n")
	fmt.Fprintf(&mutexDecl, "}\n\n")

	for methodIndex := 0; methodIndex < db.NumMethods(); methodIndex++ {
		method := db.Method(methodIndex)
		if writeWrappingFunc(finalPkg, imports, &mutexDecl, method) {

		}
		writeWrapperType(imports, &mutexDecl, method)
	}

	fmt.Println(imports)
	fmt.Println(mutexDecl.String())
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

func writeWrappingFunc(finalPkg *types.Package, imports map[string]bool, body *bytes.Buffer, method *types.Func) bool {
	sig := method.Type().Underlying().(*types.Signature)
	includeSignatureImports(imports, sig)

	needsWrapper := sig.Results().Len() == 1 && !ignoreTypes[sig.Results().At(0).Type().String()]

	fmt.Fprintf(body, "func (mu *Mutex) %s", method.Name())
	types.WriteSignature(body, sig, (*types.Package).Name)
	fmt.Fprintf(body, " {\n")
	fmt.Fprintf(body, "\tmu.mu.Lock(); defer mu.mu.Unlock()\n")
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

	return true
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

func writeWrapperType(imports map[string]bool, body *bytes.Buffer, method *types.Func) {
}

/*
func fqn(t reflect.Type) string {
	if t.Kind() == reflect.Array {
		return "[]" + fqn(t.Elem())
	}
	if t.Kind() == reflect.Ptr {
		return "*" + fqn(t.Elem())
	}

	pkg := t.PkgPath()
	if pkg == "" {
		return t.Name()
	}

	p := strings.LastIndexByte(pkg, '/')
	pkg = pkg[p+1:]
	return pkg + "." + t.Name()
}

func generateFunc(databaseName string, iface reflect.Type) {
	if iface.PkgPath() == "" {
		return
	}

	fmt.Println(`import "` + iface.PkgPath() + `"`)

	fmt.Println()
	fmt.Println("func (mu *Mutex) " + databaseName + "() " + fqn(iface) + " {")
	defer fmt.Println("}")

	fmt.Println("\treturn mu" + databaseName + "{mu: mu, db: mu.db." + databaseName + "()}")
}

func generateWrapper(databaseName string, iface reflect.Type) {
	if iface.PkgPath() == "" {
		return
	}

	fmt.Println(`type mu` + databaseName + " struct {")
	fmt.Println(`	mu *Mutex`)
	fmt.Println(`	db ` + fqn(iface))
	fmt.Println(`}`)
	fmt.Println()

	for methodIndex := 0; methodIndex < iface.NumMethod(); methodIndex++ {
		method := iface.Method(methodIndex)

		fmt.Print(`func (mu *mu` + databaseName + `) ` + method.Name + `(`)
		for inIndex := 0; inIndex < method.Type.NumIn(); inIndex++ {
			if inIndex > 0 {
				fmt.Print(`, `)
			}

			inType := method.Type.In(inIndex)
			fmt.Print(`v`, inIndex, ` `)

			if method.Type.IsVariadic() && inIndex == iface.NumIn()-1 {
				fmt.Print(`...`, fqn(inType.Elem()))
			} else {
				fmt.Print(fqn(inType))
			}
		}
		fmt.Print(`) `)

		if method.Type.NumOut() > 0 {
			fmt.Print(`(`)
			for outIndex := 0; outIndex < method.Type.NumOut(); outIndex++ {
				if outIndex > 0 {
					fmt.Print(`, `)
				}
				outType := method.Type.Out(outIndex)
				fmt.Print(fqn(outType))
			}
			fmt.Print(`)`)
		}

		fmt.Println(`{`)
		fmt.Println(`	defer mu.mu.locked()()`)

		fmt.Print(`	return mu.db.` + method.Name + `(`)
		for inIndex := 0; inIndex < method.Type.NumIn(); inIndex++ {
			if inIndex > 0 {
				fmt.Print(`, `)
			}
			fmt.Print(`v`, inIndex)
		}
		fmt.Println(`)`)
		fmt.Println(`}`)
		fmt.Println()
	}
}
*/
