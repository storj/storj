// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"net/http"
	"reflect"
)

// Endpoint represents endpoint's configuration.
type Endpoint struct {
	Name         string
	Description  string
	MethodName   string
	NoCookieAuth bool
	NoAPIAuth    bool
	Request      interface{}
	Response     interface{}
	Params       []Param
}

// CookieAuth returns endpoint's cookie auth status.
func (e *Endpoint) CookieAuth() bool {
	return !e.NoCookieAuth
}

// APIAuth returns endpoint's API auth status.
func (e *Endpoint) APIAuth() bool {
	return !e.NoAPIAuth
}

// PathMethod represents endpoint's path and method type.
type PathMethod struct {
	Path   string
	Method string
}

// EndpointGroup represents endpoints group.
type EndpointGroup struct {
	Name      string
	Prefix    string
	Endpoints map[PathMethod]*Endpoint
}

// Get adds new GET endpoint to endpoints group.
func (eg *EndpointGroup) Get(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodGet, endpoint)
}

// Put adds new PUT endpoint to endpoints group.
func (eg *EndpointGroup) Put(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodPut, endpoint)
}

// addEndpoint adds new endpoint to endpoints list.
func (eg *EndpointGroup) addEndpoint(path, method string, endpoint *Endpoint) {
	pathMethod := PathMethod{
		Path:   path,
		Method: method,
	}

	eg.Endpoints[pathMethod] = endpoint
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
