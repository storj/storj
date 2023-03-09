// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

//go:generate go run ./

import (
	"os"
	"path/filepath"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/private/apigen"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

// main defines the structure of the API and generates its associated frontend and backend code.
// These API endpoints are not currently used from inside the Satellite UI.
func main() {
	// definition for REST API
	a := &apigen.API{
		Version:     "v0",
		Description: "",
		PackageName: "consoleapi",
	}

	{
		g := a.Group("ProjectManagement", "projects")

		g.Post("/create", &apigen.Endpoint{
			Name:        "Create new Project",
			Description: "Creates new Project with given info",
			MethodName:  "GenCreateProject",
			RequestName: "createProject",
			Response:    &console.Project{},
			Request:     console.ProjectInfo{},
		})

		g.Patch("/update/{id}", &apigen.Endpoint{
			Name:        "Update Project",
			Description: "Updates project with given info",
			MethodName:  "GenUpdateProject",
			RequestName: "updateProject",
			Response:    console.Project{},
			Request:     console.ProjectInfo{},
			PathParams: []apigen.Param{
				apigen.NewParam("id", uuid.UUID{}),
			},
		})

		g.Delete("/delete/{id}", &apigen.Endpoint{
			Name:        "Delete Project",
			Description: "Deletes project by id",
			MethodName:  "GenDeleteProject",
			RequestName: "deleteProject",
			PathParams: []apigen.Param{
				apigen.NewParam("id", uuid.UUID{}),
			},
		})

		g.Get("/", &apigen.Endpoint{
			Name:        "Get Projects",
			Description: "Gets all projects user has",
			MethodName:  "GenGetUsersProjects",
			RequestName: "getProjects",
			Response:    []console.Project{},
		})

		g.Get("/bucket-rollup", &apigen.Endpoint{
			Name:        "Get Project's Single Bucket Usage",
			Description: "Gets project's single bucket usage by bucket ID",
			MethodName:  "GenGetSingleBucketUsageRollup",
			RequestName: "getBucketRollup",
			Response:    accounting.BucketUsageRollup{},
			QueryParams: []apigen.Param{
				apigen.NewParam("projectID", uuid.UUID{}),
				apigen.NewParam("bucket", ""),
				apigen.NewParam("since", time.Time{}),
				apigen.NewParam("before", time.Time{}),
			},
		})

		g.Get("/bucket-rollups", &apigen.Endpoint{
			Name:        "Get Project's All Buckets Usage",
			Description: "Gets project's all buckets usage",
			MethodName:  "GenGetBucketUsageRollups",
			RequestName: "getBucketRollups",
			Response:    []accounting.BucketUsageRollup{},
			QueryParams: []apigen.Param{
				apigen.NewParam("projectID", uuid.UUID{}),
				apigen.NewParam("since", time.Time{}),
				apigen.NewParam("before", time.Time{}),
			},
		})

		g.Get("/apikeys/{projectID}", &apigen.Endpoint{
			Name:        "Get Project's API Keys",
			Description: "Gets API keys by project ID",
			MethodName:  "GenGetAPIKeys",
			RequestName: "getAPIKeys",
			Response:    console.APIKeyPage{},
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

		g.Post("/create", &apigen.Endpoint{
			Name:        "Create new macaroon API key",
			Description: "Creates new macaroon API key with given info",
			MethodName:  "GenCreateAPIKey",
			RequestName: "createAPIKey",
			Response:    console.CreateAPIKeyResponse{},
			Request:     console.CreateAPIKeyRequest{},
		})

		g.Delete("/delete/{id}", &apigen.Endpoint{
			Name:        "Delete API Key",
			Description: "Deletes macaroon API key by id",
			MethodName:  "GenDeleteAPIKey",
			RequestName: "deleteAPIKey",
			PathParams: []apigen.Param{
				apigen.NewParam("id", uuid.UUID{}),
			},
		})
	}

	{
		g := a.Group("UserManagement", "users")

		g.Get("/", &apigen.Endpoint{
			Name:        "Get User",
			Description: "Gets User by request context",
			MethodName:  "GenGetUser",
			RequestName: "getUser",
			Response:    console.ResponseUser{},
		})
	}

	modroot := findModuleRootDir()
	a.MustWriteGo(filepath.Join(modroot, "satellite", "console", "consoleweb", "consoleapi", "api.gen.go"))
	a.MustWriteTS(filepath.Join(modroot, "web", "satellite", "src", "api", a.Version+".gen.ts"))
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
