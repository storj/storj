// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteweb

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/satellite/satelliteweb/satelliteql"
	"storj.io/storj/pkg/utils"
)

const (
	authorization = "Authorization"
	contentType   = "Content-Type"

	authorizationBearer = "Bearer "

	applicationJSON    = "application/json"
	applicationGraphql = "application/graphql"
)

// JSON request from graphql clients
type graphqlJSON struct {
	Query         string
	OperationName string
	Variables     map[string]interface{}
}

func (gw *gateway) grapqlHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(contentType, applicationJSON)

	token := getToken(req)
	query, err := getQuery(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := graphql.Do(graphql.Params{
		Schema:         gw.schema,
		Context:        auth.WithAPIKey(context.Background(), []byte(token)),
		RequestString:  query.Query,
		VariableValues: query.Variables,
		OperationName:  query.OperationName,
	})

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		gw.log.Error(err.Error())
		return
	}

	sugar := gw.log.Sugar()
	sugar.Debug(result)
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
		query.Query = req.URL.Query().Get(satelliteql.Query)
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
		return query, utils.CombineErrors(err, req.Body.Close())
	case applicationJSON:
		err := json.NewDecoder(req.Body).Decode(&query)
		return query, utils.CombineErrors(err, req.Body.Close())
	default:
		return query, errs.New("can't parse request body of type %s", typ)
	}
}
