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
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/private/apigen"
	backoffice "storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/admin/back-office/changehistory"
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

	group = api.Group("ProductManagement", "products")

	group.Get("/", &apigen.Endpoint{
		Name:           "Get products",
		Description:    "Gets all defined product definitions",
		GoName:         "GetProducts",
		TypeScriptName: "getProducts",
		Response:       []backoffice.ProductInfo{},
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

	group.Get("/", &apigen.Endpoint{
		Name:           "Search users",
		Description:    "Search users by email or name. Results are limited to 100 users.",
		GoName:         "SearchUsers",
		TypeScriptName: "searchUsers",
		QueryParams: []apigen.Param{
			apigen.NewParam("term", ""),
		},
		Response: []backoffice.AccountMin{},
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

	group.Put("/{userID}", &apigen.Endpoint{
		Name: "Disable user",
		Description: "Disables user by ID. User can only be disabled if they have no active projects" +
			" and pending invoices. It can also set status to pending deletion.",
		GoName:         "DisableUser",
		TypeScriptName: "disableUser",
		PathParams: []apigen.Param{
			apigen.NewParam("userID", uuid.UUID{}),
		},
		Request:  backoffice.DisableUserRequest{},
		Response: backoffice.UserAccount{},
		Settings: map[any]any{
			authPermsKey:     []backoffice.Permission{},
			passAuthParamKey: true,
		},
	})

	group.Put("/{userID}/freeze-events", &apigen.Endpoint{
		Name:           "Freeze/Unfreeze User",
		Description:    "Freeze or unfreeze a user account",
		GoName:         "ToggleFreezeUser",
		TypeScriptName: "toggleFreezeUser",
		PathParams: []apigen.Param{
			apigen.NewParam("userID", uuid.UUID{}),
		},
		Request: backoffice.ToggleFreezeUserRequest{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{
				/* permissions are validated dynamically in FreezeUser */
			},
			passAuthParamKey: true,
		},
	})

	group.Put("/{userID}/mfa", &apigen.Endpoint{
		Name:           "Toggle MFA",
		Description:    "Toggles MFA for a user. Only disabling is supported.",
		GoName:         "ToggleMFA",
		TypeScriptName: "toggleMFA",
		PathParams: []apigen.Param{
			apigen.NewParam("userID", uuid.UUID{}),
		},
		Request: backoffice.ToggleMfaRequest{},
		Settings: map[any]any{
			authPermsKey:     []backoffice.Permission{backoffice.PermAccountDisableMFA},
			passAuthParamKey: true,
		},
	})

	group.Post("/rest-keys/{userID}", &apigen.Endpoint{
		Name:           "Create Rest Key",
		Description:    "Creates a rest API key a user",
		GoName:         "CreateRestKey",
		TypeScriptName: "createRestKey",
		PathParams: []apigen.Param{
			apigen.NewParam("userID", uuid.UUID{}),
		},
		Request:  backoffice.CreateRestKeyRequest{},
		Response: "",
		Settings: map[any]any{
			authPermsKey:     []backoffice.Permission{backoffice.PermAccountCreateRestKey},
			passAuthParamKey: true,
		},
	})

	group = api.Group("ProjectManagement", "projects")
	group.Middleware = append(group.Middleware, authMiddleware{})

	group.Get("/statuses", &apigen.Endpoint{
		Name:           "Get project statuses",
		Description:    "Gets available project statuses",
		GoName:         "GetProjectStatuses",
		TypeScriptName: "getProjectStatuses",
		Response:       []backoffice.ProjectStatusInfo{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermProjectView},
		},
	})

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

	group.Get("/{publicID}/buckets", &apigen.Endpoint{
		Name:           "Get project buckets",
		Description:    "Gets a project's buckets",
		GoName:         "GetProjectBuckets",
		TypeScriptName: "getProjectBuckets",
		PathParams: []apigen.Param{
			apigen.NewParam("publicID", uuid.UUID{}),
		},
		QueryParams: []apigen.Param{
			apigen.NewParam("search", ""),
			apigen.NewParam("page", ""),
			apigen.NewParam("limit", ""),
			apigen.NewParam("since", time.Time{}),
			apigen.NewParam("before", time.Time{}),
		},
		Response: backoffice.BucketInfoPage{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermProjectView, backoffice.PermBucketView},
		},
	})

	group.Patch("/{publicID}/buckets/{bucketName}", &apigen.Endpoint{
		Name:           "Update bucket",
		Description:    "Updates a bucket's user agent, and placement if the bucket is empty",
		GoName:         "UpdateBucket",
		TypeScriptName: "updateBucket",
		PathParams: []apigen.Param{
			apigen.NewParam("publicID", uuid.UUID{}),
			apigen.NewParam("bucketName", ""),
		},
		Request: backoffice.UpdateBucketRequest{},
		Settings: map[any]any{
			authPermsKey:     []backoffice.Permission{},
			passAuthParamKey: true,
		},
	})

	group.Get("/{publicID}/buckets/{bucketName}/state", &apigen.Endpoint{
		Name:           "Get bucket state",
		Description:    "Gets a bucket's state that is not stored in the buckets table and requires additional queries.",
		GoName:         "GetBucketState",
		TypeScriptName: "getBucketState",
		PathParams: []apigen.Param{
			apigen.NewParam("publicID", uuid.UUID{}),
			apigen.NewParam("bucketName", ""),
		},
		Response: backoffice.BucketState{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermProjectView, backoffice.PermBucketView},
		},
	})

	group.Patch("/{publicID}", &apigen.Endpoint{
		Name:           "Update project",
		Description:    "Updates project name, user agent and default placement by ID",
		GoName:         "UpdateProject",
		TypeScriptName: "updateProject",
		PathParams: []apigen.Param{
			apigen.NewParam("publicID", uuid.UUID{}),
		},
		Request:  backoffice.UpdateProjectRequest{},
		Response: backoffice.Project{},
		Settings: map[any]any{
			authPermsKey:     []backoffice.Permission{},
			passAuthParamKey: true,
		},
	})

	group.Put("/{publicID}", &apigen.Endpoint{
		Name:           "Disable project",
		Description:    "Disables a project by ID. It can also set status to pending deletion.",
		GoName:         "DisableProject",
		TypeScriptName: "disableProject",
		PathParams: []apigen.Param{
			apigen.NewParam("publicID", uuid.UUID{}),
		},
		Request: backoffice.DisableProjectRequest{},
		Settings: map[any]any{
			authPermsKey:     []backoffice.Permission{},
			passAuthParamKey: true,
		},
	})

	group.Patch("/{publicID}/limits", &apigen.Endpoint{
		Name:           "Update project limits",
		Description:    "Updates project limits by ID",
		GoName:         "UpdateProjectLimits",
		TypeScriptName: "updateProjectLimits",
		PathParams: []apigen.Param{
			apigen.NewParam("publicID", uuid.UUID{}),
		},
		Request:  backoffice.ProjectLimitsUpdateRequest{},
		Response: backoffice.Project{},
		Settings: map[any]any{
			authPermsKey:     []backoffice.Permission{backoffice.PermProjectSetLimits},
			passAuthParamKey: true,
		},
	})

	group.Patch("/{publicID}/entitlements", &apigen.Endpoint{
		Name:           "Update project entitlements",
		Description:    "Updates project entitlements by ID. Only one entitlement can be updated at a time.",
		GoName:         "UpdateProjectEntitlements",
		TypeScriptName: "updateProjectEntitlements",
		PathParams: []apigen.Param{
			apigen.NewParam("publicID", uuid.UUID{}),
		},
		Request:  backoffice.UpdateProjectEntitlementsRequest{},
		Response: backoffice.ProjectEntitlements{},
		Settings: map[any]any{
			authPermsKey:     []backoffice.Permission{backoffice.PermProjectSetEntitlements},
			passAuthParamKey: true,
		},
	})

	// generic api group that handles searching for users and projects together
	group = api.Group("Search", "search")
	group.Middleware = append(group.Middleware, authMiddleware{})

	group.Get("/", &apigen.Endpoint{
		Name:           "Search users or projects",
		Description:    "Searches for users by email or name and projects by ID. Results are limited to 100 users.",
		GoName:         "SearchUsersOrProjects",
		TypeScriptName: "searchUsersOrProjects",
		QueryParams: []apigen.Param{
			apigen.NewParam("term", ""),
		},
		Response: backoffice.SearchResult{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{
				/* permissions are validated dynamically in SearchUsersOrProjects */
			},
			passAuthParamKey: true,
		},
	})

	// generic api group that handles retrieving change history for users, projects and buckets
	group = api.Group("ChangeHistory", "changehistory")
	group.Middleware = append(group.Middleware, authMiddleware{})

	group.Get("/", &apigen.Endpoint{
		Name: "Get change history",
		Description: "Retrieves change history for users, projects and buckets. If the exact parameter is `true`, this would" +
			"fetch changes strictly on the user, project or bucket. It'll do otherwise if it's `false`.",
		GoName:         "GetChangeHistory",
		TypeScriptName: "getChangeHistory",
		QueryParams: []apigen.Param{
			apigen.NewParam("exact", "true"),    // string because API gen doesn't support bool query params
			apigen.NewParam("itemType", "user"), // user, project, bucket
			apigen.NewParam("id", ""),           // userID, projectID, bucketName
		},
		Response: []changehistory.ChangeLog{},
		Settings: map[any]any{
			authPermsKey: []backoffice.Permission{backoffice.PermViewChangeHistory},
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
			if authInfo == nil || len(authInfo.Groups) == 0 || authInfo.Email == "" {
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
