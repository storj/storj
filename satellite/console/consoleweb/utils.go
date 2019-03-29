// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console/consoleweb/consoleql"
)

// JSON request from graphql clients
type graphqlJSON struct {
	Query         string
	OperationName string
	Variables     map[string]interface{}
}

// getToken retrieves token from request
func getToken(req *http.Request) string {
	value := req.Header.Get(authorization)
	if value == "" {
		return ""
	}

	if !strings.HasPrefix(value, authorizationBearer) {
		return ""
	}

	return value[len(authorizationBearer):]
}

// getQuery retrieves graphql query from request
func getQuery(req *http.Request) (query graphqlJSON, err error) {
	switch req.Method {
	case http.MethodGet:
		query.Query = req.URL.Query().Get(consoleql.Query)
		return query, nil
	case http.MethodPost:
		return queryPOST(req)
	default:
		return query, errs.New("wrong http request type")
	}
}

// queryPOST retrieves graphql query from POST request
func queryPOST(req *http.Request) (query graphqlJSON, err error) {
	switch typ := req.Header.Get(contentType); typ {
	case applicationGraphql:
		body, err := ioutil.ReadAll(req.Body)
		query.Query = string(body)
		return query, errs.Combine(err, req.Body.Close())
	case applicationJSON:
		err := json.NewDecoder(req.Body).Decode(&query)
		return query, errs.Combine(err, req.Body.Close())
	default:
		return query, errs.New("can't parse request body of type %s", typ)
	}
}
