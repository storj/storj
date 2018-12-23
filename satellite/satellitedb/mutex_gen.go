//+build ignore

package main

import (
	"flag"
	"fmt"
	"reflect"
	"strings"

	"storj.io/storj/satellite/satellitedb"
)

func main() {
	flag.Parse()

	typ := reflect.TypeOf((*satellitedb.DB)(nil))
	for methodIndex := 0; methodIndex < typ.NumMethod(); methodIndex++ {
		method := typ.Method(methodIndex)
		methodType := method.Type
		if methodType.NumIn() == 1 && methodType.NumOut() == 1 {
			outType := methodType.Out(0)
			if outType.Kind() == reflect.Interface {
				generateFunc(method.Name, outType)
				generateWrapper(method.Name, outType)
			}
		}
	}
}

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
