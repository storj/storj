// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

//go:generate go run ./

import (
	"storj.io/storj/private/apigen"
	"storj.io/storj/satellite/console"
)

func main() {
	a := &apigen.API{
		Version:     "v1",
		Description: "",
		PackageName: "consoleapi",
	}

	{
		g := a.Group("ProjectManagement", "projects")

		g.Get("/", &apigen.Endpoint{
			Name:        "Get Projects",
			Description: "Gets all projects user has",
			MethodName:  "GetUserProjects",
			Response:    []console.Project{},
		})

	}

	a.MustWrite("satellite/console/consoleweb/consoleapi/api.gen.go")
}
