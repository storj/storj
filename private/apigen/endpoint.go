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
	"unicode"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
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
	// Settings is the data to pass to the middleware handlers to adapt the generated
	// code to this endpoints.
	//
	// Not all the middlware handlers need extra data. Some of them use this data to disable it in
	// some endpoints.
	Settings map[any]any
}

// Validate validates the endpoint fields values are correct according to the documented constraints.
func (e *Endpoint) Validate() error {
	newErr := func(m string, a ...any) error {
		e := ". Endpoint: " + e.Name
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

// FullEndpoint represents endpoint with path and method.
type FullEndpoint struct {
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
	Prefix string
	// Middleware is a list of additional processing of requests that apply to all the endpoints of this group.
	Middleware []Middleware
	// endpoints is the list of endpoints added to this group through the "HTTP method" methods (e.g.
	// Get, Patch, etc.).
	endpoints []*FullEndpoint
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

// Put adds new PUT endpoint to endpoints group.
// It panics if path doesn't begin with '/'.
func (eg *EndpointGroup) Put(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodPut, endpoint)
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

	ep := &FullEndpoint{*endpoint, path, method}
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

// UseCORS adds CORS middleware to the endpoint group.
// This is a convenience method that appends a CORS middleware to the group.
func (eg *EndpointGroup) UseCORS() {
	eg.Middleware = append(eg.Middleware, corsMiddleware{})
}

// corsMiddleware is a standard CORS middleware implementation.
type corsMiddleware struct {
	//lint:ignore U1000 this field is used by the API generator to expose in the handler.
	cors api.CORS
}

// Generate satisfies the apigen.Middleware interface.
func (c corsMiddleware) Generate(_ *API, _ *EndpointGroup, _ *FullEndpoint) string {
	return `isPreflight := h.cors.Handle(w, r)
	if isPreflight {
		return
	}
	`
}

// ExtraServiceParams satisfies the apigen.Middleware interface.
func (c corsMiddleware) ExtraServiceParams(_ *API, _ *EndpointGroup, _ *FullEndpoint) []Param {
	return nil
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
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.String, reflect.Pointer:
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

// Middleware allows to generate custom code that's executed at the beginning of the handler.
//
// The implementation must declare their dependencies through unexported struct fields which doesn't
// begin with underscore (_), except fields whose name is just underscore (the blank identifier).
// The API generator will add the import those dependencies and allow to pass them through the
// constructor parameters of the group handler implementation, except the fields named with the
// blank identifier that should be only used to import packages that the generated code needs.
//
// The limitation of using fields with the blank identifier as its names is that those packages
// must at least to export a type, hence, it isn't possible to import packages that only export
// constants or variables.
//
// Middleware implementation with the same struct field name and type will be handled as one
// parameter, so the dependency will be shared between them. If they have the same struct field
// name, but a different type, the API generator will panic.
// NOTE types are compared as [package].[type name], hence, package name collision are not handled
// and it will produce code that doesn't compile.
type Middleware interface {
	// Generate generates the code that the API generator adds to a handler endpoint before calling
	// the service.
	//
	// All the dependencies defined as struct fields of the implementation of this interface are
	// available as fields of the struct handler. The generated code is executed inside of the methods
	// of the struct handler, hence it has access to all its fields. The handler instance is available
	// through the variable name h. For example:
	//
	// type middlewareImpl struct {
	// 	 log  *zap.Logger // Import path: "go.uber.org/zap"
	//   auth api.Auth   // Import path: "storj.io/storj/private/api"
	// }
	//
	// The generated code can access to log and auth through h.log and h.auth.
	//
	// Each handler method where the code is executed has access to the following variables names:
	// ctx of type context.Context, w of type http.ResponseWriter, and r of type *http.Request.
	// Make sure to not declare variable with those names in the generated code unless that's wrapped
	// in a scope.
	Generate(api *API, group *EndpointGroup, ep *FullEndpoint) string
	// ExtraServiceParams returns additional parameters that should be passed to the service method.
	// This allows middleware to inject parameters based on the endpoint context.
	ExtraServiceParams(api *API, group *EndpointGroup, ep *FullEndpoint) []Param
}

func middlewareImports(m any) []string {
	imports := []string{}
	middlewareWalkFields(m, func(f reflect.StructField) {
		if p := f.Type.PkgPath(); p != "" {
			imports = append(imports, p)
		}
	})

	return imports
}

// middlewareFields returns the list of fields of a middleware implementation. It panics if m isn't
// a struct type, it has embedded fields, or it has unexported fields.
func middlewareFields(api *API, m any) []middlewareField {
	fields := []middlewareField{}
	middlewareWalkFields(m, func(f reflect.StructField) {
		if f.Name == "_" {
			return
		}

		psymbol := ""
		t := f.Type
		if t.Kind() == reflect.Pointer {
			psymbol = "*"
			t = f.Type.Elem()
		}

		typeref := psymbol + t.Name()
		if p := t.PkgPath(); p != "" && p != api.PackagePath {
			pn, _ := importPath(p).PkgName()
			typeref = fmt.Sprintf("%s%s.%s", psymbol, pn, t.Name())
		}
		fields = append(fields, middlewareField{Name: f.Name, Type: typeref})
	})

	return fields
}

func middlewareWalkFields(m any, walk func(f reflect.StructField)) {
	t := reflect.TypeOf(m)
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("middleware %q isn't a struct type", t.Name()))
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.FieldByIndex([]int{i})
		if f.Anonymous {
			panic(fmt.Sprintf("middleware %q has a embedded field %q", t.Name(), f.Name))
		}

		if f.Name != "_" {
			// Disallow fields that begin with underscore.
			if !unicode.IsLetter([]rune(f.Name)[0]) {
				panic(
					fmt.Sprintf(
						"middleware %q has a field name beginning with no letter %q. Change it to begin with lower case letter",
						t.Name(),
						f.Name,
					),
				)
			}

			if unicode.IsUpper([]rune(f.Name)[0]) {
				panic(
					fmt.Sprintf(
						"middleware %q has a field name beginning with upper case %q. Change it to begin with lower case",
						t.Name(),
						f.Name,
					),
				)
			}
		}

		walk(f)
	}
}

// middlewareField has the name of the field and type for adding to handler structs that the
// API generator generates during the generation phase.
type middlewareField struct {
	// Name is the name of the field. It must fulfill Go identifiers specification
	// https://go.dev/ref/spec#Identifiers
	Name string
	// Type is the type's name of the field.
	Type string
}

// LoadSetting returns from endpoint.Settings the value assigned to key or
// returns defaultValue if the key doesn't exist.
//
// It panics if key doesn't have a value of the type T.
func LoadSetting[T any](key any, endpoint *FullEndpoint, defaultValue T) T {
	v, ok := endpoint.Settings[key]
	if !ok {
		return defaultValue
	}

	vt, vtok := v.(T)
	if !vtok {
		panic(fmt.Sprintf("expected %T got %T", vt, v))
	}

	return vt
}
