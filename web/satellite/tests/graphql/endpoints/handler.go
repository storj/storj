// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package endpoints

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
)

var (
	//graphql introspection query
	graphql = []byte(`{"query":"query  IntrospectionQuery {  __schema {  queryType { name }  mutationType { name }  subscriptionType { name }  types { ...FullType }  directives {  name  description  args { ...InputValue }  }  } }  fragment FullType on __Type {  kind  name  description  fields(includeDeprecated: true) {  name  description  args { ...InputValue }  type { ...TypeRef }  isDeprecated  deprecationReason  }  inputFields { ...InputValue }  interfaces { ...TypeRef }  enumValues(includeDeprecated: true) {  name  description  isDeprecated  deprecationReason  }  possibleTypes { ...TypeRef } }  fragment InputValue on __InputValue {  name  description  type { ...TypeRef }  defaultValue }  fragment TypeRef on __Type {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  ofType {  kind  name  }  }  }  }  }  }  } }  "}`)
)

func check(e error) { //easy error handler
	if e != nil {
		panic(e)
	}
}

//src string = "https://satellite.qa.storj.io/api/v0/graphql"
func introspect(src string) []byte { //pass in the src url and return []byte

	u, err := url.Parse(src) //parse the url to remove https:// and /path/tofile
	check(err)               //this is needed to set the host property of the header

	req, err := http.NewRequest("POST", src, bytes.NewBuffer(graphql))
	check(err)
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Host", u.Host)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	check(err)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	check(err)

	return body
}
