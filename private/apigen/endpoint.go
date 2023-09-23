// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// Endpoint represents endpoint's configuration.
type Endpoint struct {
	// Name is a free text used to name the endpoint for documentation purpose.
	// It cannot be empty.
	Name string
	// Description is a free text to describe the endpoint for documentation purpose.
	Description string
	// MethodName is the name of method of the service interface which handles the business logic of
	// this endpoint.
	// It must fulfill the Go language specification for method names
	// (https://go.dev/ref/spec#MethodName)
	// TODO: Should we rename this field to be something like ServiceMethodName?
	MethodName string
	// RequestName is the name of the method used to name the method in the client side code. When not
	// set, MethodName is used.
	// TODO: Should we delete this field in favor of always using MethodName?
	RequestName  string
	NoCookieAuth bool
	NoAPIAuth    bool
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

// fullEndpoint represents endpoint with path and method.
type fullEndpoint struct {
	Endpoint
	Path   string
	Method string
}

// requestType guarantees to return a named Go type associated to the Endpoint.Request field.
func (fe fullEndpoint) requestType() reflect.Type {
	t := reflect.TypeOf(fe.Request)
	if t.Name() == "" {
		name := fe.RequestName
		if name == "" {
			name = fe.MethodName
		}

		t = typeCustomName{Type: t, name: compoundTypeName(name, "Request")}
	}

	return t
}

// responseType guarantees to return a named Go type associated to the Endpoint.Response field.
func (fe fullEndpoint) responseType() reflect.Type {
	t := reflect.TypeOf(fe.Response)
	if t.Name() == "" {
		t = typeCustomName{Type: t, name: compoundTypeName(fe.MethodName, "Response")}
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
// It panics if path doesn't begin with '/'.
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

	ep := &fullEndpoint{*endpoint, path, method}
	for i, e := range eg.endpoints {
		if e.Path == path && e.Method == method {
			eg.endpoints[i] = ep
			return
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
			name: compoundTypeName(ep.MethodName, where, "param", p.Name),
		}
	}

	return p.Type
}
