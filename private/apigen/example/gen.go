// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore
// +build ignore

package main

import (
	"time"

	"storj.io/common/uuid"

	"storj.io/storj/private/apigen"
	"storj.io/storj/private/apigen/example/myapi"
)

func main() {
	a := &apigen.API{PackageName: "example", Version: "v0", BasePath: "/api"}

	g := a.Group("Documents", "docs")

	g.Get("/", &apigen.Endpoint{
		Name:        "Get Documents",
		Description: "Get the paths to all the documents under the specified paths",
		MethodName:  "Get",
		Response: []struct {
			ID             uuid.UUID      `json:"id"`
			Path           string         `json:"path"`
			Date           time.Time      `json:"date"`
			Metadata       myapi.Metadata `json:"metadata"`
			LastRetrievals []struct {
				User string    `json:"user"`
				When time.Time `json:"when"`
			} `json:"last_retrievals"`
		}{},
	})

	g.Get("/{path}", &apigen.Endpoint{
		Name:        "Get One",
		Description: "Get the document in the specified path",
		MethodName:  "GetOne",
		Response:    myapi.Document{},
		PathParams: []apigen.Param{
			apigen.NewParam("path", ""),
		},
	})

	g.Get("/{path}/tag/{tagName}", &apigen.Endpoint{
		Name:        "Get a tag",
		Description: "Get the tag of the document in the specified path and tag label ",
		MethodName:  "GetTag",
		Response:    [2]string{},
		PathParams: []apigen.Param{
			apigen.NewParam("path", ""),
			apigen.NewParam("tagName", ""),
		},
	})

	g.Get("/{path}/versions", &apigen.Endpoint{
		Name:        "Get Version",
		Description: "Get all the version of the document in the specified path",
		MethodName:  "GetVersions",
		Response:    []myapi.Version{},
		PathParams: []apigen.Param{
			apigen.NewParam("path", ""),
		},
	})

	g.Post("/{path}", &apigen.Endpoint{
		Name:        "Update Content",
		Description: "Update the content of the document with the specified path and ID if the last update is before the indicated date",
		MethodName:  "UpdateContent",
		Response: struct {
			ID        uuid.UUID `json:"id"`
			Date      time.Time `json:"date"`
			PathParam string    `json:"pathParam"`
			Body      string    `json:"body"`
		}{},
		Request: struct {
			Content string `json:"content"`
		}{},
		QueryParams: []apigen.Param{
			apigen.NewParam("id", uuid.UUID{}),
			apigen.NewParam("date", time.Time{}),
		},
		PathParams: []apigen.Param{
			apigen.NewParam("path", ""),
		},
	})

	a.MustWriteGo("api.gen.go")
	a.MustWriteTS("client-api.gen.ts")
	a.MustWriteDocs("apidocs.gen.md")
}
