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
//
// TODO: see how to have a better implementation for adding to seen, uniqueNames, and all.
func (types *Types) All() map[reflect.Type]string {
	all := map[reflect.Type]string{}
	uniqueNames := map[string]struct{}{}

	var walk func(t reflect.Type, alternateTypeName string)
	walk = func(t reflect.Type, altTypeName string) {
		if _, ok := all[t]; ok {
			return
		}

		if t.Name() != "" {
			// Type isn't seen it but it has the same name than a seen it one.
			// This cannot be because we would generate more than one TypeScript type with the same name.
			if _, ok := uniqueNames[t.Name()]; ok {
				panic(fmt.Sprintf("Found different types with the same name (%s)", t.Name()))
			}
		}

		if n, ok := commonClasses[t]; ok {
			all[t] = n
			uniqueNames[n] = struct{}{}
			return
		}

		switch k := t.Kind(); k {
		case reflect.Ptr:
			walk(t.Elem(), altTypeName)
		case reflect.Array, reflect.Slice:
			// If element type has a TypeScript name then an array of the element type will be defined
			// otherwise we have to create a compound type.
			if tsen := TypescriptTypeName(t.Elem()); tsen == "" {
				if altTypeName == "" {
					panic(
						fmt.Sprintf(
							"BUG: found a %q with elements of an anonymous type and without an alternative name. Found type=%q",
							t.Kind(),
							t,
						))
				}
				all[t] = altTypeName
				uniqueNames[altTypeName] = struct{}{}
				walk(t.Elem(), compoundTypeName(altTypeName, "item"))
			}
		case reflect.Struct:
			n := t.Name()
			if n == "" {
				if altTypeName == "" {
					panic(
						fmt.Sprintf(
							"BUG: found an anonymous 'struct' and without an alternative name; an alternative name is required. Found type=%q",
							t,
						))
				}

				n = altTypeName
			}

			all[t] = n
			uniqueNames[n] = struct{}{}

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				walk(field.Type, compoundTypeName(altTypeName, field.Name))
			}
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64,
			reflect.String:
			all[t] = t.Name()
			uniqueNames[t.Name()] = struct{}{}
		default:
			panic(fmt.Sprintf("type %q is not supported", t.Kind().String()))
		}
	}

	for t := range types.top {
		walk(t, t.Name())
	}

	return all
}

// GenerateTypescriptDefinitions returns the TypeScript class definitions corresponding to the registered Go types.
func (types *Types) GenerateTypescriptDefinitions() string {
	var out StringBuilder
	pf := out.Writelnf

	pf(types.getTypescriptImports())

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

				if field.Type.Name() != "" {
					pf("\t%s%s: %s;", jsonField, isOptional, TypescriptTypeName(field.Type))
				} else {
					typeName := allTypes[field.Type]
					pf("\t%s%s: %s;", jsonField, isOptional, TypescriptTypeName(typeCustomName{Type: field.Type, name: typeName}))
				}
			}
		}()
	}

	allArraySlices := filter(namedTypes, func(t typeAndName) bool {
		if _, ok := commonClasses[t.Type]; ok {
			return false
		}

		switch t.Type.Kind() {
		case reflect.Array, reflect.Slice:
			return true
		default:
			return false
		}
	})

	for _, t := range allArraySlices {
		elemTypeName, ok := allTypes[t.Type.Elem()]
		if !ok {
			panic("BUG: the element types of an Slice or Array isn't in the all types map")
		}
		pf(
			"\nexport type %s = Array<%s>",
			TypescriptTypeName(
				typeCustomName{Type: t.Type, name: t.Name}),
			TypescriptTypeName(typeCustomName{Type: t.Type.Elem(), name: elemTypeName}),
		)
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
		return capitalize(t.Name())
	default:
		panic(fmt.Sprintf(`unhandled type. Type="%+v"`, t))
	}
}
