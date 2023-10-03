// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/zeebo/errs"
)

var (
	errsEndpoint = errs.Class("Endpoint")

	goNameRegExp         = regexp.MustCompile(`^[A-Z]\w*$`)
	typeScriptNameRegExp = regexp.MustCompile(`^[a-z][a-zA-Z0-9_$]*$`)
)

// Endpoint represents endpoint's configuration.
type Endpoint struct {
	// Name is a free text used to name the endpoint for documentation purpose.
	// It cannot be empty.
	Name string
	// Description is a free text to describe the endpoint for documentation purpose.
	Description string
	// GoName is an identifier used by the Go generator to generate specific server side code for this
	// endpoint.
	//
	// It must start with an uppercase letter and fulfill the Go language specification for method
	// names (https://go.dev/ref/spec#MethodName).
	// It cannot be empty.
	GoName string
	// TypeScriptName is an identifier used by the TypeScript generator to generate specific client
	// code for this endpoint
	//
	// It must start with a lowercase letter and can only contains letters, digits, _, and $.
	// It cannot be empty.
	TypeScriptName string
	NoCookieAuth   bool
	NoAPIAuth      bool
	// Request is the type that defines the format of the request body.
	Request interface{}
	// Response is the type that defines the format of the response body.
	Response interface{}
	// QueryParams is the list of query parameters that the endpoint accepts.
	QueryParams []Param
	// PathParams is the list of path parameters that appear in the path associated with this
	// endpoint.
	PathParams []Param
}

// CookieAuth returns endpoint's cookie auth status.
func (e *Endpoint) CookieAuth() bool {
	return !e.NoCookieAuth
}

// APIAuth returns endpoint's API auth status.
func (e *Endpoint) APIAuth() bool {
	return !e.NoAPIAuth
}

// Validate validates the endpoint fields values are correct according to the documented constraints.
func (e *Endpoint) Validate() error {
	if e.Name == "" {
		return errsEndpoint.New("Name cannot be empty")
	}

	if e.Description == "" {
		return errsEndpoint.New("Description cannot be empty")
	}

	if !goNameRegExp.MatchString(e.GoName) {
		return errsEndpoint.New("GoName doesn't match the regular expression %q", goNameRegExp)
	}

	if !typeScriptNameRegExp.MatchString(e.TypeScriptName) {
		return errsEndpoint.New("TypeScriptName doesn't match the regular expression %q", typeScriptNameRegExp)
	}

	if e.Request != nil {
		switch k := reflect.TypeOf(e.Request).Kind(); k {
		case reflect.Invalid,
			reflect.Complex64,
			reflect.Complex128,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Pointer,
			reflect.UnsafePointer:
			return errsEndpoint.New("Request cannot be of a type %q", k)
		}
	}

	if e.Response != nil {
		switch k := reflect.TypeOf(e.Response).Kind(); k {
		case reflect.Invalid,
			reflect.Complex64,
			reflect.Complex128,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Pointer,
			reflect.UnsafePointer:
			return errsEndpoint.New("Response cannot be of a type %q", k)
		}
	}

	return nil
}

// fullEndpoint represents endpoint with path and method.
type fullEndpoint struct {
	Endpoint
	Path   string
	Method string
}

// requestType guarantees to return a named Go type associated to the Endpoint.Request field.
func (fe fullEndpoint) requestType() reflect.Type {
	t := reflect.TypeOf(fe.Request)
	if t.Name() != "" {
		return t
	}

	switch k := t.Kind(); k {
	case reflect.Array, reflect.Slice:
		if t.Elem().Name() == "" {
			t = typeCustomName{Type: t, name: compoundTypeName(fe.TypeScriptName, "Request")}
		}
	case reflect.Struct:
		t = typeCustomName{Type: t, name: compoundTypeName(fe.TypeScriptName, "Request")}
	default:
		panic(
			fmt.Sprintf(
				"BUG: Unsupported Request type. Endpoint.Method=%q, Endpoint.Path=%q, found type=%q",
				fe.Method, fe.Path, k,
			),
		)
	}

	return t
}

// responseType guarantees to return a named Go type associated to the Endpoint.Response field.
func (fe fullEndpoint) responseType() reflect.Type {
	t := reflect.TypeOf(fe.Response)
	if t.Name() != "" {
		return t
	}

	switch k := t.Kind(); k {
	case reflect.Array, reflect.Slice:
		if t.Elem().Name() == "" {
			t = typeCustomName{Type: t, name: compoundTypeName(fe.TypeScriptName, "Response")}
		}
	case reflect.Struct:
		t = typeCustomName{Type: t, name: compoundTypeName(fe.TypeScriptName, "Response")}
	default:
		panic(
			fmt.Sprintf(
				"BUG: Unsupported Response type. Endpoint.Method=%q, Endpoint.Path=%q, found type=%q",
				fe.Method, fe.Path, k,
			),
		)
	}

	return t
}

// EndpointGroup represents endpoints group.
// You should always create a group using API.Group because it validates the field values to
// guarantee correct code generation.
type EndpointGroup struct {
	Name      string
	Prefix    string
	endpoints []*fullEndpoint
}

// Get adds new GET endpoint to endpoints group.
// It panics if path doesn't begin with '/'.
func (eg *EndpointGroup) Get(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodGet, endpoint)
}

// Patch adds new PATCH endpoint to endpoints group.
// It panics if path doesn't begin with '/'.
func (eg *EndpointGroup) Patch(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodPatch, endpoint)
}

// Post adds new POST endpoint to endpoints group.
// It panics if path doesn't begin with '/'.
func (eg *EndpointGroup) Post(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodPost, endpoint)
}

// Delete adds new DELETE endpoint to endpoints group.
// It panics if path doesn't begin with '/'.
func (eg *EndpointGroup) Delete(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodDelete, endpoint)
}

// addEndpoint adds new endpoint to endpoints list.
// It panics if:
//   - path doesn't begin with '/'.
//   - endpoint.Validate() returns an error.
//   - An Endpoint with the same path and method already exists.
func (eg *EndpointGroup) addEndpoint(path, method string, endpoint *Endpoint) {
	if !strings.HasPrefix(path, "/") {
		panic(
			fmt.Sprintf(
				"invalid path for method %q of EndpointGroup %q. path must start with slash, got %q",
				method,
				eg.Name,
				path,
			),
		)
	}

	if err := endpoint.Validate(); err != nil {
		panic(err)
	}

	ep := &fullEndpoint{*endpoint, path, method}
	for _, e := range eg.endpoints {
		if e.Path == path && e.Method == method {
			panic(fmt.Sprintf("there is already an endpoint defined with path %q and method %q", path, method))
		}

		if e.GoName == ep.GoName {
			panic(
				fmt.Sprintf("GoName %q is already used by the endpoint with path %q and method %q", e.GoName, e.Path, e.Method),
			)
		}

		if e.TypeScriptName == ep.TypeScriptName {
			panic(
				fmt.Sprintf(
					"TypeScriptName %q is already used by the endpoint with path %q and method %q",
					e.TypeScriptName,
					e.Path,
					e.Method,
				),
			)
		}
	}
	eg.endpoints = append(eg.endpoints, ep)
}

// Param represents string interpretation of param's name and type.
type Param struct {
	Name string
	Type reflect.Type
}

// NewParam constructor which creates new Param entity by given name and type.
func NewParam(name string, instance interface{}) Param {
	return Param{
		Name: name,
		Type: reflect.TypeOf(instance),
	}
}

// namedType guarantees to return a named Go type. where defines where the param is  defined (e.g.
// path, query, etc.).
func (p Param) namedType(ep Endpoint, where string) reflect.Type {
	if p.Type.Name() == "" {
		return typeCustomName{
			Type: p.Type,
			name: compoundTypeName(ep.TypeScriptName, where, "param", p.Name),
		}
	}

	return p.Type
}
