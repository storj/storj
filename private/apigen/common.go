// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"reflect"
	"strings"

	"storj.io/storj/private/api"
)

// API represents specific API's configuration.
type API struct {
	Version        string
	Description    string
	PackageName    string
	Auth           api.Auth
	EndpointGroups []*EndpointGroup
}

// Group adds new endpoints group to API.
func (a *API) Group(name, prefix string) *EndpointGroup {
	group := &EndpointGroup{
		Name:   name,
		Prefix: prefix,
	}

	a.EndpointGroups = append(a.EndpointGroups, group)

	return group
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

// filter returns a new slice of reflect.Type values that satisfy the given keep function.
func filter(types []reflect.Type, keep func(reflect.Type) bool) []reflect.Type {
	filtered := make([]reflect.Type, 0, len(types))
	for _, t := range types {
		if keep(t) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// isNillableType returns whether instances of the given type can be nil.
func isNillableType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return true
	}
	return false
}
