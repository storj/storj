// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

//go:generate go run ./

import (
	"net/http"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"storj.io/storj/private/api"
	"storj.io/storj/private/apigen"
	"storj.io/storj/satellite/console"
)

func main() {
	a := &apigen.API{
		Version:     "v1",
		BasePath:    "/api",
		Description: "Used by the Satellite UI",
		PackagePath: "storj.io/storj/satellite/console/consoleweb/consoleapi/privateapi",
	}

	{
		g := a.Group("AuthManagement", "auth")
		g.UseCORS()
		g.Middleware = append(g.Middleware, AuthMiddleware{})

		g.Get("/account", &apigen.Endpoint{
			Name:           "Get User account",
			Description:    "Returns existing User",
			GoName:         "GetUserAccount",
			TypeScriptName: "getUserAccount",
			Response:       console.UserAccount{},
		})
	}

	a.OutputRootDir = findModuleRootDir()
	a.MustWriteGo(filepath.Join("satellite", "console", "consoleweb", "consoleapi", "privateapi", "api.private.gen.go"))
	a.MustWriteTS(filepath.Join("web", "satellite", "src", "api", "private.gen.ts"))
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
	nocookie := apigen.LoadSetting(NoCookie, ep, false)
	if nocookie {
		return ""
	}

	return `ctx, err = h.auth.IsAuthenticated(ctx, r, true, false)
	if err != nil {
		h.auth.RemoveAuthCookie(w)
		api.ServeError(h.log, w, http.StatusUnauthorized, err)
		return
	}

	authUser, err := console.GetUser(ctx)
	if err != nil {
		h.auth.RemoveAuthCookie(w)
		api.ServeError(h.log, w, http.StatusUnauthorized, err)
		return
	}`
}

// ExtraServiceParams satisfies the apigen.Middleware interface.
func (a AuthMiddleware) ExtraServiceParams(_ *apigen.API, _ *apigen.EndpointGroup, ep *apigen.FullEndpoint) []apigen.Param {
	nocookie := apigen.LoadSetting(NoCookie, ep, false)
	if nocookie {
		return nil
	}

	return []apigen.Param{
		apigen.NewParam("authUser", &console.User{}),
	}
}

var _ apigen.Middleware = AuthMiddleware{}

type tagNoCookie struct{}

// NoCookie is the key for endpoint settings to indicate that it doesn't use cookie authentication
// mechanism.
var NoCookie tagNoCookie
