// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bootstrapql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/bootstrap/bootstrapweb"
)

// CreateSchema creates a schema for bootstrap graphql api
func CreateSchema(service *bootstrapweb.Service) (graphql.Schema, error) {
	creator := TypeCreator{}

	err := creator.Create(service)
	if err != nil {
		return graphql.Schema{}, err
	}

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: creator.RootQuery(),
	})
}
