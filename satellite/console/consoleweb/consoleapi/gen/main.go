// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

//go:generate go run ./

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/private/apigen"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

// main defines the structure of the API and generates its associated frontend and backend code.
// These API endpoints are not currently used from inside the Satellite UI.
func main() {
	// definition for REST API
	a := &apigen.API{
		Version:     "v1",
		BasePath:    "/public",
		Description: "Interacts with projects",
		PackagePath: "storj.io/storj/satellite/console/consoleweb/consoleapi",
	}

	{
		g := a.Group("ProjectManagement", "projects")
		g.Middleware = append(g.Middleware, AuthMiddleware{})

		g.Post("/create", &apigen.Endpoint{
			Name:           "Create new Project",
			Description:    "Creates new Project with given info",
			GoName:         "GenCreateProject",
			TypeScriptName: "createProject",
			Response:       console.Project{},
			Request:        console.UpsertProjectInfo{},
		})

		g.Patch("/update/{id}", &apigen.Endpoint{
			Name:           "Update Project",
			Description:    "Updates project with given info",
			GoName:         "GenUpdateProject",
			TypeScriptName: "updateProject",
			Response:       console.Project{},
			Request:        console.UpsertProjectInfo{},
			PathParams: []apigen.Param{
				apigen.NewParam("id", uuid.UUID{}),
			},
		})

		g.Delete("/delete/{id}", &apigen.Endpoint{
			Name:           "Delete Project",
			Description:    "Deletes project by id",
			GoName:         "GenDeleteProject",
			TypeScriptName: "deleteProject",
			PathParams: []apigen.Param{
				apigen.NewParam("id", uuid.UUID{}),
			},
		})

		g.Get("/", &apigen.Endpoint{
			Name:           "Get Projects",
			Description:    "Gets all projects user has",
			GoName:         "GenGetUsersProjects",
			TypeScriptName: "getProjects",
			Response:       []console.Project{},
		})

		g.Get("/bucket-rollup", &apigen.Endpoint{
			Name:           "Get Project's Single Bucket Usage",
			Description:    "Gets project's single bucket usage by bucket ID",
			GoName:         "GenGetSingleBucketUsageRollup",
			TypeScriptName: "getBucketRollup",
			Response:       accounting.BucketUsageRollup{},
			QueryParams: []apigen.Param{
				apigen.NewParam("projectID", uuid.UUID{}),
				apigen.NewParam("bucket", ""),
				apigen.NewParam("since", time.Time{}),
				apigen.NewParam("before", time.Time{}),
			},
		})

		g.Get("/bucket-rollups", &apigen.Endpoint{
			Name:           "Get Project's All Buckets Usage",
			Description:    "Gets project's all buckets usage",
			GoName:         "GenGetBucketUsageRollups",
			TypeScriptName: "getBucketRollups",
			Response:       []accounting.BucketUsageRollup{},
			QueryParams: []apigen.Param{
				apigen.NewParam("projectID", uuid.UUID{}),
				apigen.NewParam("since", time.Time{}),
				apigen.NewParam("before", time.Time{}),
			},
		})

		g.Get("/apikeys/{projectID}", &apigen.Endpoint{
			Name:           "Get Project's API Keys",
			Description:    "Gets API keys by project ID",
			GoName:         "GenGetAPIKeys",
			TypeScriptName: "getAPIKeys",
			Response:       console.APIKeyPage{},
			PathParams: []apigen.Param{
				apigen.NewParam("projectID", uuid.UUID{}),
			},
			QueryParams: []apigen.Param{
				apigen.NewParam("search", ""),
				apigen.NewParam("limit", uint(0)),
				apigen.NewParam("page", uint(0)),
				apigen.NewParam("order", console.APIKeyOrder(0)),
				apigen.NewParam("orderDirection", console.OrderDirection(0)),
			},
		})
	}

	{
		g := a.Group("APIKeyManagement", "apikeys")
		g.Middleware = append(g.Middleware, AuthMiddleware{})

		g.Post("/create", &apigen.Endpoint{
			Name:           "Create new macaroon API key",
			Description:    "Creates new macaroon API key with given info",
			GoName:         "GenCreateAPIKey",
			TypeScriptName: "createAPIKey",
			Response:       console.CreateAPIKeyResponse{},
			Request:        console.CreateAPIKeyRequest{},
		})

		g.Delete("/delete/{id}", &apigen.Endpoint{
			Name:           "Delete API Key",
			Description:    "Deletes macaroon API key by id",
			GoName:         "GenDeleteAPIKey",
			TypeScriptName: "deleteAPIKey",
			PathParams: []apigen.Param{
				apigen.NewParam("id", uuid.UUID{}),
			},
		})
	}

	{
		g := a.Group("UserManagement", "users")
		g.Middleware = append(g.Middleware, AuthMiddleware{})

		g.Get("/", &apigen.Endpoint{
			Name:           "Get User",
			Description:    "Gets User by request context",
			GoName:         "GenGetUser",
			TypeScriptName: "getUser",
			Response:       console.ResponseUser{},
		})
	}

	a.OutputRootDir = findModuleRootDir()
	a.MustWriteGo(filepath.Join("satellite", "console", "consoleweb", "consoleapi", "api.gen.go"))
	a.MustWriteTS(filepath.Join("web", "satellite", "src", "api", a.Version+".gen.ts"))
	a.MustWriteDocs(filepath.Join("satellite", "console", "consoleweb", "consoleapi", "apidocs.gen.md"))
}

func findModuleRootDir() string {
	dir, err := os.Getwd()
	if err != nil {
		panic("unable to find current working directory")
	}
	start := dir

	for i := 0; i < 100; i++ {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}

	panic("unable to find go.mod starting from " + start)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// AuthMiddleware customize endpoints to authenticate requests by API Key or Cookie.
type AuthMiddleware struct {
	//lint:ignore U1000 this field is used by the API generator to expose in the handler.
	log *zap.Logger
	//lint:ignore U1000 this field is used by the API generator to expose in the handler.
	auth api.Auth
	_    http.ResponseWriter // Import the http package to use its HTTP status constants
}

// Generate satisfies the apigen.Middleware.
func (a AuthMiddleware) Generate(api *apigen.API, group *apigen.EndpointGroup, ep *apigen.FullEndpoint) string {
	noapikey := apigen.LoadSetting(NoAPIKey, ep, false)
	nocookie := apigen.LoadSetting(NoCookie, ep, false)

	if noapikey && nocookie {
		return ""
	}

	return fmt.Sprintf(`ctx, err = h.auth.IsAuthenticated(ctx, r, %t, %t)
	if err != nil {
		h.auth.RemoveAuthCookie(w)
		api.ServeError(h.log, w, http.StatusUnauthorized, err)
		return
	}`, !nocookie, !noapikey)
}

// ExtraServiceParams satisfies the apigen.Middleware interface.
func (a AuthMiddleware) ExtraServiceParams(_ *apigen.API, _ *apigen.EndpointGroup, _ *apigen.FullEndpoint) []apigen.Param {
	return nil
}

var _ apigen.Middleware = AuthMiddleware{}

type (
	tagNoAPIKey struct{}
	tagNoCookie struct{}
)

var (
	// NoAPIKey is the key for endpoint settings to indicate that it doesn't use API Key
	// authentication mechanism.
	NoAPIKey tagNoAPIKey
	// NoCookie is the key for endpoint settings to indicate that it doesn't use cookie authentication
	// mechanism.
	NoCookie tagNoCookie
)
