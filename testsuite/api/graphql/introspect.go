//  Copyright (C) 2021 Storj Labs, Inc.
//  See LICENSE for copying information.

package introspect

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

var (
	// graphql introspection query.
	graphql = []byte(`{"query":"query  IntrospectionQuery {  __schema {  queryType { name }  mutationType { name }  subscriptionType { name }  types { ...FullType }  directives {  name  description  args { ...InputValue }  }  } }  fragment FullType on __Type {  kind  name  description  fields(includeDeprecated: true) {  name  description  args { ...InputValue }  type { ...TypeRef }  isDeprecated  deprecationReason  }  inputFields { ...InputValue }  interfaces { ...TypeRef }  enumValues(includeDeprecated: true) {  name  description  isDeprecated  deprecationReason  }  possibleTypes { ...TypeRef } }  fragment InputValue on __InputValue {  name  description  type { ...TypeRef }  defaultValue }  fragment TypeRef on __Type {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  }  }  }  }  }  }  } }  "}`)
	err     = errs.Class("graphql")
)

type server struct {
	fullpath  string
	sataddr   string
	satendpnt string // "/api/v0/graphql"
}

// Endpoint defines a configuration for anintrospection endpoint.
type Endpoint struct {
	Log      *zap.Logger
	request  []byte
	server   *server
	response []byte
}

// New creates a new instance of Endpoint
func newEndpoint(log *zap.Logger, r []byte) *Endpoint {
	return &Endpoint{
		Log:     log,
		request: graphql,
		server: &server{
			satendpnt: "/api/v0/graphql",
		},
	}
}

// Introspect takes a url and returns a byte slice of endpoints and an error message.//dst url := planet.Satellites[0].ConsoleURL()
func Introspect(dst string) (endpoints []byte, err error) { // pass in the dst url and return []byte slice and error or nil.
	e, err := newEndpoint(log*zap.Logger, graphql)

	e.server.fullpath, err = url.Parse(dst) // parse the url to remove https://  and /path/tofile.
	if err != nil {                         // this is needed to set the host property of the header.
		fmt.Println("Oh noes!!")
	}
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, "POST", dst, bytes.NewBuffer(graphql))
	check(err)
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Host", url.Host)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	check(err)

	defer func() {
		if err := resp.Body.Close(); err != nil {
			check(err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	check(err)

	return body, nil
}
