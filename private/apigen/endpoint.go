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
	RequestName  string
	NoCookieAuth bool
	NoAPIAuth    bool
	Request      interface{}
	Response     interface{}
	QueryParams  []Param
	PathParams   []Param
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

// EndpointGroup represents endpoints group.
type EndpointGroup struct {
	Name      string
	Prefix    string
	endpoints []*fullEndpoint
}

// Get adds new GET endpoint to endpoints group.
func (eg *EndpointGroup) Get(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodGet, endpoint)
}

// Patch adds new PATCH endpoint to endpoints group.
func (eg *EndpointGroup) Patch(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodPatch, endpoint)
}

// Post adds new POST endpoint to endpoints group.
func (eg *EndpointGroup) Post(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodPost, endpoint)
}

// Delete adds new DELETE endpoint to endpoints group.
func (eg *EndpointGroup) Delete(path string, endpoint *Endpoint) {
	eg.addEndpoint(path, http.MethodDelete, endpoint)
}

// addEndpoint adds new endpoint to endpoints list.
func (eg *EndpointGroup) addEndpoint(path, method string, endpoint *Endpoint) {
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
