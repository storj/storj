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
		case reflect.Map:
			walk(t.Key())
			walk(t.Elem())
		case reflect.Struct:
			if t.Name() == "" {
				panic(fmt.Sprintf("BUG: found an anonymous 'struct'. Found type=%q", t))
			}

			all[t] = typeNameWithoutGenerics(t.Name())

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				if field.Anonymous {
					if field.Type.Kind() != reflect.Struct {
						panic(fmt.Sprintf("only embedded struct types are allowed. (%s).%s", field.Type, field.Name))
					}

					_, has, err := parseJSONTag(field.Type, field)
					if err != nil {
						panic(err)
					}

					if !has {
						// We don't want to create Typescript classes of fields which are structs anonymous, and
						// without JSON tag their fields are flatten into the parent.
						continue
					}
				}
				walk(field.Type)
			}
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64,
			reflect.String:
			all[t] = t.Name()
		case reflect.Interface:
			all[t] = "unknown"
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

			fields := GetClassFieldsFromStruct(t.Type)
			for _, f := range fields {
				pf(f.String())
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
	case reflect.Map:
		keyType := TypescriptTypeName(t.Key())
		valueType := TypescriptTypeName(t.Elem())
		return fmt.Sprintf("Record<%s, %s>", keyType, valueType)
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
	case reflect.Interface:
		return "unknown"
	case reflect.Struct:
		if t.Name() == "" {
			panic(fmt.Sprintf(`anonymous struct aren't accepted because their type doesn't have a name. Type="%+v"`, t))
		}
		return capitalize(typeNameWithoutGenerics(t.Name()))
	default:
		panic(fmt.Sprintf(`unhandled type. Type="%+v"`, t))
	}
}

// ClassField is a description of a field to generate a string representation of a TypeScript class
// field.
type ClassField struct {
	Name     string
	Type     reflect.Type
	TypeName string
	Optional bool
	Nullable bool
}

// String returns the c string representation.
func (c *ClassField) String() string {
	isOptional := ""
	if c.Optional {
		isOptional = "?"
	}

	isNullable := ""
	if c.Nullable {
		isNullable = " | null"
	}

	return fmt.Sprintf("\t%s%s: %s%s;", c.Name, isOptional, c.TypeName, isNullable)
}

// GetClassFieldsFromStruct takes a struct type and returns the list of Class fields definition to
// create a TypeScript class based on t JSON representation.
//
// It panics if t is not a struct, it has embedded fields that aren't structs, it has JSON tags
// names which aren't unique (considering that embedded ones are flatten into the class),
// a non-embedded field has no JSON tag, or a JSON tag is malformed.
func GetClassFieldsFromStruct(t reflect.Type) []ClassField {
	fieldNames := map[string]struct{}{}

	var walk func(t reflect.Type) []ClassField
	walk = func(t reflect.Type) []ClassField {
		if t.Kind() != reflect.Struct {
			panic("BUG: getClassFields must only be called with struct types")
		}

		fields := []ClassField{}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonInfo, ok, err := parseJSONTag(t, field)
			if err != nil {
				panic(err)
			}

			if jsonInfo.Skip {
				continue
			}

			if !ok && !field.Anonymous {
				panic(
					fmt.Sprintf(
						"only embedded struct fields are allowed to not have a JSON tag definition. (%s).%s",
						t, field.Name,
					),
				)
			}

			if field.Anonymous {
				if field.Type.Kind() != reflect.Struct {
					panic(fmt.Sprintf("only embedded struct types are allowed. (%s).%s", t, field.Name))
				}

				fields = append(fields, walk(field.Type)...)
				continue
			}

			if _, ok := fieldNames[jsonInfo.FieldName]; ok {
				panic(fmt.Sprintf(
					"duplicated field name for TypeScript class. Go embedded struct fields are only accepted if their JSON field name is unique across all the fields that flatten into the parent struct. Found duplicated on (%s).%s (json name: %s)",
					t,
					field.Name,
					jsonInfo.FieldName,
				))
			}

			fieldNames[jsonInfo.FieldName] = struct{}{}

			fields = append(
				fields,
				ClassField{
					Name:     jsonInfo.FieldName,
					Type:     field.Type,
					TypeName: TypescriptTypeName(field.Type),
					Optional: jsonInfo.OmitEmpty,
					Nullable: isNillableType(field.Type),
				},
			)
		}

		return fields
	}

	return walk(t)
}
