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
		panic("register an anonymous type is not supported. All the types must have a name")
	}
	types.top[t] = struct{}{}
}

// All returns a slice containing every top-level type and their dependencies.
//
// TODO: see how to have a better implementation for adding to seen, uniqueNames, and all.
func (types *Types) All() []reflect.Type {
	seen := map[reflect.Type]struct{}{}
	uniqueNames := map[string]struct{}{}
	all := []reflect.Type{}

	var walk func(t reflect.Type, alternateTypeName string)
	walk = func(t reflect.Type, altTypeName string) {
		if _, ok := seen[t]; ok {
			return
		}

		// Type isn't seen it but it has the same name than a seen it one.
		// This cannot be because we would generate more than one TypeScript type with the same name.
		if _, ok := uniqueNames[t.Name()]; ok {
			panic(fmt.Sprintf("Found different types with the same name (%s)", t.Name()))
		}

		if _, ok := commonClasses[t]; ok {
			seen[t] = struct{}{}
			uniqueNames[t.Name()] = struct{}{}
			all = append(all, t)
			return
		}

		switch k := t.Kind(); k {
		// TODO: Does reflect.Ptr to be registered?, I believe that could skip it and only register
		// the type that points to.
		case reflect.Array, reflect.Ptr, reflect.Slice:
			t = typeCustomName{Type: t, name: compoundTypeName(altTypeName, k.String())}
			seen[t] = struct{}{}
			uniqueNames[t.Name()] = struct{}{}
			all = append(all, t)
			walk(t.Elem(), altTypeName)
		case reflect.Struct:
			if t.Name() == "" {
				t = typeCustomName{Type: t, name: altTypeName}
			}

			seen[t] = struct{}{}
			uniqueNames[t.Name()] = struct{}{}
			all = append(all, t)

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				walk(field.Type, compoundTypeName(altTypeName, field.Name))
			}
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64,
			reflect.String:
			seen[t] = struct{}{}
			uniqueNames[t.Name()] = struct{}{}
			all = append(all, t)
		default:
			panic(fmt.Sprintf("type '%s' is not supported", t.Kind().String()))
		}
	}

	for t := range types.top {
		walk(t, t.Name())
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

		// TODO, we should be able to handle arrays and slices as defined types now
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
// If the type is an anonymous struct, it returns an empty string.
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
