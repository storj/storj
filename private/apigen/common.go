// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"storj.io/storj/private/api"
)

// OutputRootDirEnvOverride is the name of the environment variable that can be used to override the root directory used for the api.Write... functions.
const OutputRootDirEnvOverride = "STORJ_APIGEN_OUTPUT_TO_DIR"

// groupNameAndPrefixRegExp guarantees that Group name and prefix are empty or have are only formed
// by ASCII letters or digits and not starting with a digit.
var groupNameAndPrefixRegExp = regexp.MustCompile(`^([A-Za-z][0-9A-Za-z]*)?$`)

// API represents specific API's configuration.
type API struct {
	// Version is the corresponding version of the API.
	// It's concatenated to the BasePath, so assuming the base path is "/api" and the version is "v1"
	// the API paths will begin with `/api/v1`.
	// When empty, the version doesn't appear in the API paths. If it starts or ends with one or more
	// "/", they are stripped from the API endpoint paths.
	Version     string
	Description string
	// The package name to use for the Go generated code.
	// If omitted, the last segment of the PackagePath will be used as the package name.
	PackageName string
	// The path of the package that will use the generated Go code.
	// This is used to prevent the code from importing its own package.
	PackagePath string
	// BasePath is the  base path for the API endpoints. E.g. "/api".
	// It doesn't require to begin with "/". When empty, "/" is used.
	BasePath       string
	Auth           api.Auth
	EndpointGroups []*EndpointGroup

	// OutputRootDir is the root directory that functions like WriteGo, WriteTS, and WriteDocs will use.
	// If defined, the OutputRootDirEnvOverride environment variable will be used instead.
	OutputRootDir string
}

// Group adds new endpoints group to API.
// name must be `^([A-Z0-9]\w*)?$“
// prefix must be `^\w*$`.
func (a *API) Group(name, prefix string) *EndpointGroup {
	if !groupNameAndPrefixRegExp.MatchString(name) {
		panic(
			fmt.Sprintf(
				"invalid name for API Endpoint Group. name must fulfill the regular expression %q, got %q",
				groupNameAndPrefixRegExp,
				name,
			),
		)
	}
	if !groupNameAndPrefixRegExp.MatchString(prefix) {
		panic(
			fmt.Sprintf(
				"invalid prefix for API Endpoint Group %q. prefix must fulfill the regular expression %q, got %q",
				name,
				groupNameAndPrefixRegExp,
				prefix,
			),
		)
	}

	for _, g := range a.EndpointGroups {
		if strings.EqualFold(g.Name, name) {
			panic(fmt.Sprintf("name has to be case-insensitive unique across all the groups. name=%q", name))
		}
		if strings.EqualFold(g.Prefix, prefix) {
			panic(fmt.Sprintf("prefix has to be case-insensitive unique across all the groups. prefix=%q", prefix))
		}
	}

	group := &EndpointGroup{
		Name:   name,
		Prefix: prefix,
	}

	a.EndpointGroups = append(a.EndpointGroups, group)

	return group
}

func (a *API) outputRootDir() string {
	rootDir := a.OutputRootDir
	if envRoot := os.Getenv(OutputRootDirEnvOverride); envRoot != "" {
		rootDir = envRoot
	}
	return rootDir
}

func (a *API) endpointBasePath() string {
	if strings.HasPrefix(a.BasePath, "/") {
		return path.Join(a.BasePath, a.Version)
	}

	return "/" + path.Join(a.BasePath, a.Version)
}

// StringBuilder is an extension of strings.Builder that allows for writing formatted lines.
type StringBuilder struct{ strings.Builder }

// Writelnf formats arguments according to a format specifier
// and appends the resulting string to the StringBuilder's buffer.
func (s *StringBuilder) Writelnf(format string, a ...interface{}) {
	s.WriteString(fmt.Sprintf(format+"\n", a...))
}

// getElementaryType simplifies a Go type.
func getElementaryType(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Array, reflect.Chan, reflect.Ptr, reflect.Slice:
		return getElementaryType(t.Elem())
	default:
		return t
	}
}

// isNillableType returns whether instances of the given type can be nil.
func isNillableType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return true
	}
	return false
}

// isJSONOmittableType returns whether the "omitempty" JSON tag option works with struct fields of this type.
func isJSONOmittableType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String,
		reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Interface, reflect.Pointer:
		return true
	}
	return false
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size <= 0 {
		return s
	}

	return string(unicode.ToTitle(r)) + s[size:]
}

func uncapitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size <= 0 {
		return s
	}

	return string(unicode.ToLower(r)) + s[size:]
}

type typeAndName struct {
	Type reflect.Type
	Name string
}

func mapToSlice(typesAndNames map[reflect.Type]string) []typeAndName {
	list := make([]typeAndName, 0, len(typesAndNames))
	for t, n := range typesAndNames {
		list = append(list, typeAndName{Type: t, Name: n})
	}

	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return list
}

// filter returns a new slice of typeAndName values that satisfy the given keep function.
func filter(types []typeAndName, keep func(typeAndName) bool) []typeAndName {
	filtered := make([]typeAndName, 0, len(types))
	for _, t := range types {
		if keep(t) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

type jsonTagInfo struct {
	FieldName string
	OmitEmpty bool
	Skip      bool
}

// parseJSONTag returns the JSON tag information and true if the field has it, otherwise false.
// It returns an error if the JSON tag is malformed.
func parseJSONTag(structType reflect.Type, field reflect.StructField) (_ jsonTagInfo, has bool, _ error) {
	tag, ok := field.Tag.Lookup("json")
	if !ok {
		return jsonTagInfo{}, false, nil
	}

	options := strings.Split(tag, ",")
	for i, opt := range options {
		options[i] = strings.TrimSpace(opt)
	}

	fieldName := options[0]
	if fieldName == "" {
		return jsonTagInfo{}, false, fmt.Errorf("(%s).%s missing json field name", structType.String(), field.Name)
	}
	if fieldName == "-" && len(options) == 1 {
		return jsonTagInfo{Skip: true}, true, nil
	}

	info := jsonTagInfo{FieldName: fieldName}
	for _, opt := range options[1:] {
		if opt == "omitempty" {
			info.OmitEmpty = isJSONOmittableType(field.Type)
			break
		}
	}

	return info, true, nil
}

func typeNameWithoutGenerics(n string) string {
	return strings.SplitN(n, "[", 2)[0]
}
