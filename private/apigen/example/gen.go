// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore

package main

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
	"storj.io/storj/private/apigen/example"
	"storj.io/storj/private/apigen/example/myapi"
)

func main() {
	a := &apigen.API{
		PackagePath: "storj.io/storj/private/apigen/example",
		Version:     "v0",
		BasePath:    "/api",
	}

	g := a.Group("Documents", "docs")
	g.Middleware = append(g.Middleware,
		authMiddleware{},
	)

	now := time.Date(2001, 02, 03, 04, 05, 06, 07, time.UTC)

	g.Get("/", &apigen.Endpoint{
		Name:           "Get Documents",
		Description:    "Get the paths to all the documents under the specified paths",
		GoName:         "Get",
		TypeScriptName: "get",
		Response:       []myapi.Document{},
		ResponseMock: []myapi.Document{{
			ID:        uuid.UUID{},
			PathParam: "/workspace/notes.md",
			Metadata: myapi.Metadata{
				Owner: "Storj",
				Tags:  [][2]string{{"category", "general"}},
			},
		}},
		Settings: map[any]any{
			NoAPIKey: true,
			NoCookie: true,
		},
	})

	g.Get("/{path}", &apigen.Endpoint{
		Name:           "Get One",
		Description:    "Get the document in the specified path",
		GoName:         "GetOne",
		TypeScriptName: "getOne",
		Response:       myapi.Document{},
		PathParams: []apigen.Param{
			apigen.NewParam("path", ""),
		},
		ResponseMock: myapi.Document{
			ID:        uuid.UUID{},
			Date:      now.Add(-24 * time.Hour),
			PathParam: "ID",
			Body:      "## Notes",
			Version: myapi.Version{
				Date:   now.Add(-30 * time.Minute),
				Number: 1,
			},
		},
	})

	g.Get("/{path}/tag/{tagName}", &apigen.Endpoint{
		Name:           "Get a tag",
		Description:    "Get the tag of the document in the specified path and tag label ",
		GoName:         "GetTag",
		TypeScriptName: "getTag",
		Response:       [2]string{},
		PathParams: []apigen.Param{
			apigen.NewParam("path", ""),
			apigen.NewParam("tagName", ""),
		},
		ResponseMock: [2]string{"category", "notes"},
	})

	g.Get("/{path}/versions", &apigen.Endpoint{
		Name:           "Get Version",
		Description:    "Get all the version of the document in the specified path",
		GoName:         "GetVersions",
		TypeScriptName: "getVersions",
		Response:       []myapi.Version{},
		PathParams: []apigen.Param{
			apigen.NewParam("path", ""),
		},
		ResponseMock: []myapi.Version{
			{Date: now.Add(-360 * time.Hour), Number: 1},
			{Date: now.Add(-5 * time.Hour), Number: 2},
		},
	})

	g.Post("/{path}", &apigen.Endpoint{
		Name:           "Update Content",
		Description:    "Update the content of the document with the specified path and ID if the last update is before the indicated date",
		GoName:         "UpdateContent",
		TypeScriptName: "updateContent",
		Response:       myapi.Document{},
		Request:        myapi.NewDocument{},
		QueryParams: []apigen.Param{
			apigen.NewParam("id", uuid.UUID{}),
			apigen.NewParam("date", time.Time{}),
		},
		PathParams: []apigen.Param{
			apigen.NewParam("path", ""),
		},
		ResponseMock: myapi.Document{
			ID:        uuid.UUID{},
			Date:      now,
			PathParam: "ID",
			Body:      "## Notes\n### General",
		},
	})

	g = a.Group("Users", "users")

	g.Get("/", &apigen.Endpoint{
		Name:           "Get Users",
		Description:    "Get the list of registered users",
		GoName:         "Get",
		TypeScriptName: "get",
		Response:       []myapi.User{},
		ResponseMock: []myapi.User{
			{
				Name:         "Storj",
				Surname:      "Labs",
				Email:        "storj@storj.test",
				Professional: myapi.Professional{Company: "Test 1", Position: "Tester"},
			},
			{
				Name: "Test1", Surname: "Testing", Email: "test1@example.test",
				Professional: myapi.Professional{Company: "Test 2", Position: "Accountant"},
			},
			{
				Name: "Test2", Surname: "Testing", Email: "test2@example.test",
				Professional: myapi.Professional{Company: "Test 3", Position: "Slacker"},
			},
		},
	})

	g.Post("/", &apigen.Endpoint{
		Name:           "Create Users",
		Description:    "Create users",
		GoName:         "Create",
		TypeScriptName: "create",
		Request:        []myapi.User{},
	})

	g.Get("/age", &apigen.Endpoint{
		Name:           "Get User's age",
		Description:    "Get the user's age",
		GoName:         "GetAge",
		TypeScriptName: "getAge",
		Response:       myapi.UserAge[int16]{},
		ResponseMock:   myapi.UserAge[int16]{Day: 1, Month: 1, Year: 2000},
	})

	g = a.Group("Projects", "projects")

	g.Post("/", &apigen.Endpoint{
		Name:           "Create Projects",
		Description:    "Create projects",
		GoName:         "CreateProject",
		TypeScriptName: "createProject",
		Request:        example.Project{},
		Response:       example.Project{},
		ResponseMock:   example.Project{ID: uuid.UUID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}), OwnerName: "Foo Bar"},
	})

	a.OutputRootDir = findModuleRootDir()
	a.MustWriteGo(filepath.Join("private", "apigen", "example", "api.gen.go"))
	a.MustWriteTS(filepath.Join("private", "apigen", "example", "client-api.gen.ts"))
	a.MustWriteTSMock(filepath.Join("private", "apigen", "example", "client-api-mock.gen.ts"))
	a.MustWriteDocs(filepath.Join("private", "apigen", "example", "apidocs.gen.md"))
}

// authMiddleware customize endpoints to authenticate requests by API Key or Cookie.
type authMiddleware struct {
	log  *zap.Logger
	auth api.Auth
	_    http.ResponseWriter // Import the http package to use its HTTP status constants
}

// Generate satisfies the apigen.Middleware.
func (a authMiddleware) Generate(api *apigen.API, group *apigen.EndpointGroup, ep *apigen.FullEndpoint) string {
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
func (a authMiddleware) ExtraServiceParams(_ *apigen.API, _ *apigen.EndpointGroup, _ *apigen.FullEndpoint) []apigen.Param {
	return nil
}

var _ apigen.Middleware = authMiddleware{}

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
