// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"storj.io/storj/private/api"
)

var (
	groupNameRegExp   = regexp.MustCompile(`^([A-Z0-9]\w*)?$`)
	groupPrefixRegExp = regexp.MustCompile(`^\w*$`)
)

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
	PackageName string
	// BasePath is the  base path for the API endpoints. E.g. "/api".
	// It doesn't require to begin with "/". When empty, "/" is used.
	BasePath       string
	Auth           api.Auth
	EndpointGroups []*EndpointGroup
}

// Group adds new endpoints group to API.
// name must be `^([A-Z0-9]\w*)?$“
// prefix must be `^\w*$`.
func (a *API) Group(name, prefix string) *EndpointGroup {
	if !groupNameRegExp.MatchString(name) {
		panic(
			fmt.Sprintf(
				"invalid name for API Endpoint Group. name must fulfill the regular expression %q, got %q",
				groupNameRegExp,
				name,
			),
		)
	}
	if !groupPrefixRegExp.MatchString(prefix) {
		panic(
			fmt.Sprintf(
				"invalid prefix for API Endpoint Group %q. prefix must fulfill the regular expression %q, got %q",
				name,
				groupPrefixRegExp,
				prefix,
			),
		)
	}

	group := &EndpointGroup{
		Name:   name,
		Prefix: prefix,
	}

	a.EndpointGroups = append(a.EndpointGroups, group)

	return group
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

// typeCustomName is a reflect.Type with a customized type's name.
type typeCustomName struct {
	reflect.Type

	name string
}

func (t typeCustomName) Name() string {
	return t.name
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

// compoundTypeName create a name composed with base and parts, by joining base as it's and
// capitalizing each part.
func compoundTypeName(base string, parts ...string) string {
	caser := cases.Title(language.Und)
	titled := make([]string, len(parts))
	for i := 0; i < len(parts); i++ {
		titled[i] = caser.String(parts[i])
	}

	return base + strings.Join(titled, "")
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
