// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

//go:generate go run ./

import (
	"storj.io/storj/private/apigen"
	"storj.io/storj/satellite/console"
)

func main() {
	api := apigen.New("v1", "")

	{
		g := api.Group("Projects", "projects")

		g.Get("/", &apigen.Endpoint{
			Name:        "List Projects",
			Description: "Lists all projects user has",
			MethodName:  "ListUserProjects",
			Response:    []console.Project{},
		})
	}
}
