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
	"storj.io/storj/satellite/console"
)

type authParamKey int

// passAuthParamKey is a setting key to mark endpoints whose service
// methods should be passed authenticated user's groups.
const passAuthParamKey authParamKey = 0

func main() {
	api := &apigen.API{
		PackageName: "admin",
		PackagePath: "storj.io/storj/satellite/admin/back-office",
		Version:     "v1",
		BasePath:    path.Join(backoffice.PathPrefix, "/api"),
	}

	group := api.Group("Settings", "settings")
	group.Middleware = append(group.Middleware, authMiddleware{})

	group.Get("/", &apigen.Endpoint{
		Name:           "Get settings",
		Description:    "Gets the settings of the service and relevant Storj services settings",
		GoName:         "GetSettings",
		TypeScriptName: "get",
		Response:       backoffice.Settings{},
		Settings: map[any]any{
			passAuthParamKey: true,
		},
	})

	group = api.Group("PlacementManagement", "placements")

	group.Get("/", &apigen.Endpoint{
		Name:           "Get placements",
		Description:    "Gets placement rule IDs and their locations",
		GoName:         "GetPlacements",
		TypeScriptName: "getPlacements",
		Response:       []backoffice.PlacementInfo{},
	})

	group = api.Group("UserManagement", "users")
	group.Middleware = append(group.Middleware, authMiddleware{})

	group.Get("/freeze-event-types", &apigen.Endpoint{
		Name:           "Get freeze event types",
		Description:    "Gets account freeze event types",
		GoName:         "GetFreezeEventTypes",
		TypeScriptName: "getFreezeEventTypes",
		Response:       []backoffice.FreezeEventType{},
	})

	group.Get("/kinds", &apigen.Endpoint{
		Name:           "Get user kinds",
		Description:    "Gets available user kinds",
		GoName:         "GetUserKinds",
		TypeScriptName: "getUserKinds",
		Response:       []console.KindInfo{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermAccountView},
		},
	})

	group.Get("/statuses", &apigen.Endpoint{
		Name:           "Get user statuses",
		Description:    "Gets available user statuses",
		GoName:         "GetUserStatuses",
		TypeScriptName: "getUserStatuses",
		Response:       []console.UserStatusInfo{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermAccountView},
		},
	})

	group.Get("/email/{email}", &apigen.Endpoint{
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

	group.Get("/{userID}", &apigen.Endpoint{
		Name:           "Get user",
		Description:    "Gets user by ID",
		GoName:         "GetUser",
		TypeScriptName: "getUser",
		PathParams: []apigen.Param{
			apigen.NewParam("userID", uuid.UUID{}),
		},
		Response: backoffice.UserAccount{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermAccountView},
		},
	})

	group.Patch("/{userID}", &apigen.Endpoint{
		Name: "Update user",
		Description: "Updates user info by ID. Limit updates will cascade to all projects of the user." +
			"Updating user kind to NFR or Paid without providing limits will set the limits to kind defaults.",
		GoName:         "UpdateUser",
		TypeScriptName: "updateUser",
		PathParams: []apigen.Param{
			apigen.NewParam("userID", uuid.UUID{}),
		},
		Request:  backoffice.UpdateUserRequest{},
		Response: backoffice.UserAccount{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{
				/* permissions are validated dynamically in UpdateUser */
			},
			passAuthParamKey: true,
		},
	})

	group.Delete("/{userID}", &apigen.Endpoint{
		Name: "Delete user",
		Description: "Deletes user by ID. User can only be deleted if they have no active projects" +
			" and pending invoices.",
		GoName:         "DeleteUser",
		TypeScriptName: "deleteUser",
		PathParams: []apigen.Param{
			apigen.NewParam("userID", uuid.UUID{}),
		},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermAccountDeleteNoData},
		},
	})

	group.Post("/{userID}/freeze-events", &apigen.Endpoint{
		Name:           "Freeze User",
		Description:    "Freeze a user account",
		GoName:         "FreezeUser",
		TypeScriptName: "freezeUser",
		PathParams: []apigen.Param{
			apigen.NewParam("userID", uuid.UUID{}),
		},
		Request: backoffice.FreezeUserRequest{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermAccountSuspendTemporary},
		},
	})

	group.Delete("/{userID}/freeze-events", &apigen.Endpoint{
		Name:           "Unfreeze User",
		Description:    "Unfreeze a user account",
		GoName:         "UnfreezeUser",
		TypeScriptName: "unfreezeUser",
		PathParams: []apigen.Param{
			apigen.NewParam("userID", uuid.UUID{}),
		},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermAccountReActivateTemporary},
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

	group.Put("/limits/{publicID}", &apigen.Endpoint{
		Name:           "Update project limits",
		Description:    "Updates project limits by ID",
		GoName:         "UpdateProjectLimits",
		TypeScriptName: "updateProjectLimits",
		PathParams: []apigen.Param{
			apigen.NewParam("publicID", uuid.UUID{}),
		},
		Request: backoffice.ProjectLimitsUpdate{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermProjectSetLimits},
		},
	})

	api.OutputRootDir = findModuleRootDir()
	api.MustWriteGo(filepath.Join("satellite", "admin", "back-office", "handlers.gen.go"))
	api.MustWriteTS(filepath.Join("satellite", "admin", "back-office", "ui", "src", "api", "client.gen.ts"))
	api.MustWriteDocs(filepath.Join("satellite", "admin", "back-office", "api-docs.gen.md"))
}

type authMiddleware struct {
	//lint:ignore U1000 this field is used by the API generator to expose in the handler.
	auth *backoffice.Authorizer
}

func (a authMiddleware) Generate(_ *apigen.API, _ *apigen.EndpointGroup, ep *apigen.FullEndpoint) string {
	format := `
		if err = h.auth.VerifyHost(r); err != nil {
			api.ServeError(h.log, w, http.StatusForbidden, err)
			return
		}
	`
	if apigen.LoadSetting(passAuthParamKey, ep, false) {
		format += `
			authInfo := h.auth.GetAuthInfo(r)
			if authInfo == nil || len(authInfo.Groups) == 0 {
				api.ServeError(h.log, w, http.StatusUnauthorized, errs.New("Unauthorized"))
				return
			}
		`
	}

	perms := apigen.LoadSetting(authPermsKey, ep, []backoffice.Permission{})
	if len(perms) == 0 {
		return format
	}

	verbs := make([]string, 0, len(perms))
	values := make([]any, 0, len(perms))
	for _, p := range perms {
		verbs = append(verbs, "%d")
		values = append(values, p)
	}

	format += fmt.Sprintf(`
		if h.auth.IsRejected(w, r, %s) {
			return
		}`, strings.Join(verbs, ", "))

	return fmt.Sprintf(format, values...)
}

// ExtraServiceParams satisfies the apigen.Middleware interface.
func (a authMiddleware) ExtraServiceParams(_ *apigen.API, _ *apigen.EndpointGroup, ep *apigen.FullEndpoint) []apigen.Param {
	if apigen.LoadSetting(passAuthParamKey, ep, false) {
		return []apigen.Param{
			apigen.NewParam("authInfo", &backoffice.AuthInfo{}),
		}
	}
	return nil
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
