// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"storj.io/common/memory"
	"storj.io/common/uuid"
)

// commonPath is the path to the TypeScript module that common classes are imported from.
const commonPath = "@/types/common"

// commonClasses is a mapping of Go types to their corresponding TypeScript class names.
var commonClasses = map[reflect.Type]string{
	reflect.TypeOf(memory.Size(0)): "MemorySize",
	reflect.TypeOf(time.Time{}):    "Time",
	reflect.TypeOf(uuid.UUID{}):    "UUID",
}

// NewTypes creates a new type definition generator.
func NewTypes() Types {
	return Types{top: make(map[reflect.Type]struct{})}
}

// Types handles generating definitions from types.
type Types struct {
	top map[reflect.Type]struct{}
}

// Register registers a type for generation.
func (types *Types) Register(t reflect.Type) {
	types.top[t] = struct{}{}
}

// All returns a slice containing every top-level type and their dependencies.
func (types *Types) All() []reflect.Type {
	seen := map[reflect.Type]struct{}{}
	all := []reflect.Type{}

	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		if _, ok := seen[t]; ok {
			return
		}
		seen[t] = struct{}{}
		all = append(all, t)

		if _, ok := commonClasses[t]; ok {
			return
		}

		switch t.Kind() {
		case reflect.Array, reflect.Ptr, reflect.Slice:
			walk(t.Elem())
		case reflect.Struct:
			for i := 0; i < t.NumField(); i++ {
				walk(t.Field(i).Type)
			}
		case reflect.Bool:
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		case reflect.Float32, reflect.Float64:
		case reflect.String:
			break
		default:
			panic(fmt.Sprintf("type '%s' is not supported", t.Kind().String()))
		}
	}

	for t := range types.top {
		walk(t)
	}

	sort.Slice(all, func(i, j int) bool {
		return strings.Compare(all[i].Name(), all[j].Name()) < 0
	})

	return all
}

// GenerateTypescriptDefinitions returns the TypeScript class definitions corresponding to the registered Go types.
func (types *Types) GenerateTypescriptDefinitions() string {
	var out StringBuilder
	pf := out.Writelnf

	pf(types.getTypescriptImports())

	all := filter(types.All(), func(t reflect.Type) bool {
		if _, ok := commonClasses[t]; ok {
			return false
		}
		return t.Kind() == reflect.Struct
	})

	for _, t := range all {
		func() {
			pf("\nexport class %s {", t.Name())
			defer pf("}")

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				attributes := strings.Fields(field.Tag.Get("json"))
				if len(attributes) == 0 || attributes[0] == "" {
					pathParts := strings.Split(t.PkgPath(), "/")
					pkg := pathParts[len(pathParts)-1]
					panic(fmt.Sprintf("(%s.%s).%s missing json declaration", pkg, t.Name(), field.Name))
				}

				jsonField := attributes[0]
				if jsonField == "-" {
					continue
				}

				isOptional := ""
				if isNillableType(t) {
					isOptional = "?"
				}

				pf("\t%s%s: %s;", jsonField, isOptional, TypescriptTypeName(field.Type))
			}
		}()
	}

	return out.String()
}

// getTypescriptImports returns the TypeScript import directive for the registered Go types.
func (types *Types) getTypescriptImports() string {
	classes := []string{}

	all := types.All()
	for _, t := range all {
		if tsClass, ok := commonClasses[t]; ok {
			classes = append(classes, tsClass)
		}
	}

	if len(classes) == 0 {
		return ""
	}

	sort.Slice(classes, func(i, j int) bool {
		return strings.Compare(classes[i], classes[j]) < 0
	})

	return fmt.Sprintf("import { %s } from '%s';", strings.Join(classes, ", "), commonPath)
}

// TypescriptTypeName gets the corresponding TypeScript type for a provided reflect.Type.
func TypescriptTypeName(t reflect.Type) string {
	if override, ok := commonClasses[t]; ok {
		return override
	}

	switch t.Kind() {
	case reflect.Ptr:
		return TypescriptTypeName(t.Elem())
	case reflect.Slice:
		// []byte ([]uint8) is marshaled as a base64 string
		elem := t.Elem()
		if elem.Kind() == reflect.Uint8 {
			return "string"
		}
		fallthrough
	case reflect.Array:
		return TypescriptTypeName(t.Elem()) + "[]"
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "number"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "number"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Struct:
		return t.Name()
	default:
		panic("unhandled type: " + t.Name())
	}
}
