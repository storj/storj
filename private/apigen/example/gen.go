// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore
// +build ignore

package main

import (
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/private/apigen"
)

func main() {
	a := &apigen.API{PackageName: "example"}

	g := a.Group("TestAPI", "testapi")

	g.Post("/{path}", &apigen.Endpoint{
		MethodName: "GenTestAPI",
		Response: struct {
			ID        uuid.UUID
			Date      time.Time
			PathParam string
			Body      string
		}{},
		Request: struct{ Content string }{},
		QueryParams: []apigen.Param{
			apigen.NewParam("id", uuid.UUID{}),
			apigen.NewParam("date", time.Time{}),
		},
		PathParams: []apigen.Param{
			apigen.NewParam("path", ""),
		},
	})

	a.MustWriteGo("api.gen.go")
}
