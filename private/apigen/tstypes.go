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
	if t.Name() == "" {
		switch t.Kind() {
		case reflect.Array, reflect.Slice, reflect.Ptr:
			if t.Elem().Name() == "" {
				panic(
					fmt.Sprintf("register an %q of elements of an anonymous type is not supported", t.Name()),
				)
			}
		default:
			panic("register an anonymous type is not supported. All the types must have a name")
		}
	}
	types.top[t] = struct{}{}
}

// All returns a map containing every top-level and their dependency types with their associated name.
func (types *Types) All() map[reflect.Type]string {
	all := map[reflect.Type]string{}

	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		if _, ok := all[t]; ok {
			return
		}

		if n, ok := commonClasses[t]; ok {
			all[t] = n
			return
		}

		switch k := t.Kind(); k {
		case reflect.Ptr:
			walk(t.Elem())
		case reflect.Array, reflect.Slice:
			walk(t.Elem())
		case reflect.Struct:
			if t.Name() == "" {
				panic(fmt.Sprintf("BUG: found an anonymous 'struct'. Found type=%q", t))
			}

			all[t] = t.Name()

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				walk(field.Type)
			}
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64,
			reflect.String:
			all[t] = t.Name()
		default:
			panic(fmt.Sprintf("type %q is not supported", t.Kind().String()))
		}
	}

	for t := range types.top {
		walk(t)
	}

	return all
}

// GenerateTypescriptDefinitions returns the TypeScript class definitions corresponding to the registered Go types.
func (types *Types) GenerateTypescriptDefinitions() string {
	var out StringBuilder
	pf := out.Writelnf

	{
		i := types.getTypescriptImports()
		if i != "" {
			pf(i)
		}
	}

	allTypes := types.All()
	namedTypes := mapToSlice(allTypes)
	allStructs := filter(namedTypes, func(tn typeAndName) bool {
		if _, ok := commonClasses[tn.Type]; ok {
			return false
		}

		return tn.Type.Kind() == reflect.Struct
	})

	for _, t := range allStructs {
		func() {
			name := capitalize(t.Name)
			pf("\nexport class %s {", name)
			defer pf("}")

			for i := 0; i < t.Type.NumField(); i++ {
				field := t.Type.Field(i)
				attributes := strings.Fields(field.Tag.Get("json"))
				if len(attributes) == 0 || attributes[0] == "" {
					pathParts := strings.Split(t.Type.PkgPath(), "/")
					pkg := pathParts[len(pathParts)-1]
					panic(fmt.Sprintf("(%s.%s).%s missing json declaration", pkg, name, field.Name))
				}

				jsonField := attributes[0]
				if jsonField == "-" {
					continue
				}

				isOptional := ""
				if isNillableType(field.Type) {
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

	for t := range types.All() {
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
// If the type is an anonymous struct, it returns an empty string.
func TypescriptTypeName(t reflect.Type) string {
	if override, ok := commonClasses[t]; ok {
		return override
	}

	switch t.Kind() {
	case reflect.Ptr:
		return TypescriptTypeName(t.Elem())
	case reflect.Array, reflect.Slice:
		if t.Name() != "" {
			return capitalize(t.Name())
		}

		// []byte ([]uint8) is marshaled as a base64 string
		elem := t.Elem()
		if elem.Kind() == reflect.Uint8 {
			return "string"
		}

		return TypescriptTypeName(elem) + "[]"
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
		if t.Name() == "" {
			panic(fmt.Sprintf(`anonymous struct aren't accepted because their type doesn't have a name. Type="%+v"`, t))
		}
		return capitalize(t.Name())
	default:
		panic(fmt.Sprintf(`unhandled type. Type="%+v"`, t))
	}
}
