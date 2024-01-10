// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

// Package main defines the satellite administration API through the API generator and generates
// source code of the API server handlers and clients and the documentation markdown document.
package main

//go:generate go run $GOFILE

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"storj.io/common/uuid"
	"storj.io/storj/private/apigen"
	backoffice "storj.io/storj/satellite/admin/back-office"
)

func main() {
	api := &apigen.API{
		PackageName: "admin",
		PackagePath: "storj.io/storj/satellite/admin/back-office",
		Version:     "v1",
		BasePath:    path.Join(backoffice.PathPrefix, "/api"),
	}

	group := api.Group("PlacementManagement", "placements")

	group.Get("/", &apigen.Endpoint{
		Name:           "Get placements",
		Description:    "Gets placement rule IDs and their locations",
		GoName:         "GetPlacements",
		TypeScriptName: "getPlacements",
		Response:       []backoffice.PlacementInfo{},
	})

	group = api.Group("UserManagement", "users")
	group.Middleware = append(group.Middleware, authMiddleware{})

	group.Get("/{email}", &apigen.Endpoint{
		Name:           "Get user",
		Description:    "Gets user by email address",
		GoName:         "GetUserByEmail",
		TypeScriptName: "getUserByEmail",
		PathParams: []apigen.Param{
			apigen.NewParam("email", ""),
		},
		Response: backoffice.UserAccount{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermAccountView},
		},
	})

	group = api.Group("ProjectManagement", "projects")
	group.Middleware = append(group.Middleware, authMiddleware{})

	group.Get("/{publicID}", &apigen.Endpoint{
		Name:           "Get project",
		Description:    "Gets project by ID",
		GoName:         "GetProject",
		TypeScriptName: "getProject",
		PathParams: []apigen.Param{
			apigen.NewParam("publicID", uuid.UUID{}),
		},
		Response: backoffice.Project{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermProjectView},
		},
	})

	modroot := findModuleRootDir()
	api.MustWriteGo(filepath.Join(modroot, "satellite", "admin", "back-office", "handlers.gen.go"))
	api.MustWriteTS(filepath.Join(modroot, "satellite", "admin", "back-office", "ui", "src", "api", "client.gen.ts"))
	api.MustWriteDocs(filepath.Join(modroot, "satellite", "admin", "back-office", "api-docs.gen.md"))
}

type authMiddleware struct {
	//lint:ignore U1000 this field is used by the API generator to expose in the handler.
	auth *backoffice.Authorizer
}

func (a authMiddleware) Generate(api *apigen.API, group *apigen.EndpointGroup, ep *apigen.FullEndpoint) string {
	perms := apigen.LoadSetting(authPermsKey, ep, []backoffice.Permission{})
	if len(perms) == 0 {
		return ""
	}

	verbs := make([]string, 0, len(perms))
	values := make([]any, 0, len(perms))
	for _, p := range perms {
		verbs = append(verbs, "%d")
		values = append(values, p)
	}

	format := fmt.Sprintf(`if h.auth.IsRejected(w, r, %s) {
		return
	}`, strings.Join(verbs, ", "))

	return fmt.Sprintf(format, values...)
}

var _ apigen.Middleware = authMiddleware{}

type tagAuthPerms struct{}

var authPermsKey = tagAuthPerms{}

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
