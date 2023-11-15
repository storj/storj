// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

// Package main defines the satellite administration API through the API generator and generates
// source code of the API server handlers and clients and the documentation markdown document.
package main

//go:generate go run $GOFILE

import (
	"os"
	"path"
	"path/filepath"

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

	modroot := findModuleRootDir()
	api.MustWriteGo(filepath.Join(modroot, "satellite", "admin", "back-office", "handlers.gen.go"))
	api.MustWriteTS(filepath.Join(modroot, "satellite", "admin", "back-office", "ui", "src", "api", "client.gen.ts"))
	api.MustWriteDocs(filepath.Join(modroot, "satellite", "admin", "back-office", "api-docs.gen.md"))
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
