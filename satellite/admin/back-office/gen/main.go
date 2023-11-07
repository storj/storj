// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

// Package main defines the satellite administration API through the API generator and generates
// source code of the API server handlers and clients and the documentation markdown document.
package main

import (
	"os"
	"path/filepath"

	"storj.io/storj/private/apigen"
)

func main() {
	api := &apigen.API{PackageName: "admin", Version: "v1", BasePath: "/api"}

	// This is an example and must be deleted when we define the first real endpoint.
	group := api.Group("Example", "example")

	group.Get("/examples", &apigen.Endpoint{
		Name:           "Get examples",
		Description:    "Get a list with the names of the all available examples",
		GoName:         "GetExamples",
		TypeScriptName: "getExamples",
		Response:       []string{},
		ResponseMock:   []string{"example-1", "example-2", "example-3"},
		NoCookieAuth:   false,
		NoAPIAuth:      false,
	})

	modroot := findModuleRootDir()
	api.MustWriteGo(filepath.Join(modroot, "satellite", "admin", "back-office", "handlers.gen.go"))
	api.MustWriteTS(filepath.Join(modroot, "satellite", "admin", "back-office", "ui", "src", "api", "client.gen.ts"))
	api.MustWriteTSMock(filepath.Join(modroot, "satellite", "admin", "back-office", "ui", "src", "api", "client-mock.gen.ts"))
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
