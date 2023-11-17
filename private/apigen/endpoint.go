// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

var (
	errsEndpoint = errs.Class("Endpoint")

	goNameRegExp         = regexp.MustCompile(`^[A-Z]\w*$`)
	typeScriptNameRegExp = regexp.MustCompile(`^[a-z][a-zA-Z0-9_$]*$`)
)

// Endpoint represents endpoint's configuration.
//
// Passing an anonymous type to the fields that define the request or response will make the API
// generator to panic. Anonymous types aren't allowed such as named structs that have fields with
// direct or indirect of anonymous types, slices or arrays whose direct or indirect elements are of
// anonymous types.
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
	// ResponseMock is the data to use as a response for the generated mocks.
	// It must be of the same type than Response.
	// If a mock generator is called it must not be nil unless Response is nil.
	ResponseMock interface{}
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
	newErr := func(m string, a ...any) error {
		e := fmt.Sprintf(". Endpoint: %s", e.Name)
		m += e
		return errsEndpoint.New(m, a...)
	}

	if e.Name == "" {
		return newErr("Name cannot be empty")
	}

	if e.Description == "" {
		return newErr("Description cannot be empty")
	}

	if !goNameRegExp.MatchString(e.GoName) {
		return newErr("GoName doesn't match the regular expression %q", goNameRegExp)
	}

	if !typeScriptNameRegExp.MatchString(e.TypeScriptName) {
		return newErr("TypeScriptName doesn't match the regular expression %q", typeScriptNameRegExp)
	}

	if e.Request != nil {
		switch t := reflect.TypeOf(e.Request); t.Kind() {
		case reflect.Invalid,
			reflect.Complex64,
			reflect.Complex128,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Pointer,
			reflect.UnsafePointer:
			return newErr("Request cannot be of a type %q", t.Kind())
		case reflect.Array, reflect.Slice:
			if t.Elem().Name() == "" {
				return newErr("Request cannot be of %q of anonymous struct elements", t.Kind())
			}
		case reflect.Struct:
			if t.Name() == "" {
				return newErr("Request cannot be of an anonymous struct")
			}
		}
	}

	if e.Response != nil {
		switch t := reflect.TypeOf(e.Response); t.Kind() {
		case reflect.Invalid,
			reflect.Complex64,
			reflect.Complex128,
			reflect.Chan,
			reflect.Func,
			reflect.Interface,
			reflect.Map,
			reflect.Pointer,
			reflect.UnsafePointer:
			return newErr("Response cannot be of a type %q", t.Kind())
		case reflect.Array, reflect.Slice:
			if t.Elem().Name() == "" {
				return newErr("Response cannot be of %q of anonymous struct elements", t.Kind())
			}
		case reflect.Struct:
			if t.Name() == "" {
				return newErr("Response cannot be of an anonymous struct")
			}
		}

		if e.ResponseMock != nil {
			if m, r := reflect.TypeOf(e.ResponseMock), reflect.TypeOf(e.Response); m != r {
				return newErr(
					"ResponseMock isn't of the same type than Response. Have=%q Want=%q", m, r,
				)
			}
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

// EndpointGroup represents endpoints group.
// You should always create a group using API.Group because it validates the field values to
// guarantee correct code generation.
type EndpointGroup struct {
	// Name is the group name.
	//
	// Go generator uses it as part of type, functions, interfaces names, and in code comments.
	// The casing is adjusted according where it's used.
	//
	// TypeScript generator uses it as part of types names for the API functionality of this group.
	// The casing is adjusted according where it's used.
	//
	// Document generator uses as it is.
	Name string
	// Prefix is a prefix used for
	//
	// Go generator uses it as part of variables names, error messages, and the URL base path for the group.
	// The casing is adjusted according where it's used, but for the URL base path, lowercase is used.
	//
	// TypeScript generator uses it for composing the URL base path (lowercase).
	//
	// Document generator uses as it is.
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

// NewParam constructor which creates new Param entity by given name and type through instance.
//
// instance can only be a unsigned integer (of any size), string, uuid.UUID or time.Time, otherwise
// it panics.
func NewParam(name string, instance interface{}) Param {
	switch t := reflect.TypeOf(instance); t {
	case reflect.TypeOf(uuid.UUID{}), reflect.TypeOf(time.Time{}):
	default:
		switch k := t.Kind(); k {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.String:
		default:
			panic(
				fmt.Sprintf(
					`Unsupported parameter, only types: %q, %q, string, and "unsigned numbers" are supported . Found type=%q, Kind=%q`,
					reflect.TypeOf(uuid.UUID{}),
					reflect.TypeOf(time.Time{}),
					t,
					k,
				),
			)
		}
	}

	return Param{
		Name: name,
		Type: reflect.TypeOf(instance),
	}
}
