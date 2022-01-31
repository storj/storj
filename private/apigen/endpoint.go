// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import "net/http"

// Endpoint represents endpoint's configuration.
type Endpoint struct {
	Name         string
	Description  string
	MethodName   string
	NoCookieAuth bool
	NoAPIAuth    bool
	Request      interface{}
	Response     interface{}
	Params       []string
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

// addEndpoint adds new endpoint to endpoints list.
func (eg *EndpointGroup) addEndpoint(path, method string, endpoint *Endpoint) {
	pathMethod := PathMethod{
		Path:   path,
		Method: method,
	}

	eg.Endpoints[pathMethod] = endpoint
}
