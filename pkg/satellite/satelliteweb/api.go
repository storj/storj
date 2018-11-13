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

func (gw *gateway) grapqlHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(contentType, applicationJSON)

	token := getToken(req)
	query, err := getQuery(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := graphql.Do(graphql.Params{
		Schema:        gw.schema,
		Context:       auth.WithAPIKey(context.Background(), []byte(token)),
		RequestString: query,
	})

	if result.HasErrors() {
		err = json.NewEncoder(w).Encode(result.Errors)
	} else {
		err = json.NewEncoder(w).Encode(result)
	}

	if err != nil {
		gw.logger.Error(err)
		return
	}

	gw.logger.Debug(result)
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
func getQuery(req *http.Request) (query string, err error) {
	switch req.Method {
	case http.MethodGet:
		return req.URL.Query().Get(satelliteql.Query), nil
	case http.MethodPost:
		return queryPOST(req)
	default:
		return "", errs.New("wrong http request type")
	}
}

// queryPOST retrieves query from POST request
func queryPOST(req *http.Request) (query string, err error) {
	switch typ := req.Header.Get(contentType); typ {
	case applicationGraphql:
		body, err := ioutil.ReadAll(req.Body)
		return string(body), utils.CombineErrors(err, req.Body.Close())
	//TODO(yar): test more precisely
	case applicationJSON:
		var query struct {
			Query string
		}

		err := json.NewDecoder(req.Body).Decode(&query)
		return query.Query, utils.CombineErrors(err, req.Body.Close())
	default:
		return "", errs.New("can't parse request body of type %s", typ)
	}
}
