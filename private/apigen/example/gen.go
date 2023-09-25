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

	g.Get("/{path}", &apigen.Endpoint{
		Name:        "Get One",
		Description: "Get one document with the specified version",
		MethodName:  "GetOne",
		Response:    myapi.Document{},
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
