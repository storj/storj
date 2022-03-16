// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

//go:generate go run ./

import (
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/private/apigen"
	"storj.io/storj/satellite/accounting"
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
			MethodName:  "GenGetUsersProjects",
			Response:    []console.Project{},
		})

		g.Get("/bucket-rollup", &apigen.Endpoint{
			Name:        "Get Project's Bucket Usage",
			Description: "Gets project's bucket usage by bucket ID",
			MethodName:  "GenGetSingleBucketUsageRollup",
			Response:    &accounting.BucketUsageRollup{},
			Params: []apigen.Param{
				apigen.NewParam("projectID", uuid.UUID{}),
				apigen.NewParam("bucket", ""),
				apigen.NewParam("since", time.Time{}),
				apigen.NewParam("before", time.Time{}),
			},
		})
	}

	a.MustWrite("satellite/console/consoleweb/consoleapi/api.gen.go")
}
