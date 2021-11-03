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

type Server struct {
	fullpath  string
	sataddr   string
	satendpnt string // "/api/v0/graphql"
}

type Endpoint struct {
	Log      *zap.Logger
	request  []byte
	server   *Server
	response []byte
}

func New(log *zap.Logger, r []byte) (endpoint *Endpoint, err error) {
	return &Endpoint{
		Log:     log,
		request: graphql,
		server: Server{
			satendpnt: "/api/v0/graphql",
		},
	}
}

// introspect takes a url and returns a byte slice of endpoints and an error message.
func Introspect(src string) (endpoints []byte, err error) { // pass in the src url and return []byte slice and error || nil.
	url, err := url.Parse(src) // parse the url to remove https://  and /path/tofile.
	if err != nil {            // this is needed to set the host property of the header.
		fmt.Println("Oh noes!!")
	}
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, "POST", src, bytes.NewBuffer(graphql))
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
