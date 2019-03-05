// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bootstrapql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/bootstrap/bootstrapweb"
	"storj.io/storj/internal/storjql"
)

// CreateSchema creates a schema for bootstrap graphql api
func CreateSchema(service *bootstrapweb.Service) (schema graphql.Schema, err error) {
	storjql.WithLock(func() {
		creator := TypeCreator{}

		err := creator.Create(service)
		if err != nil {
			return
		}

		schema, err = graphql.NewSchema(graphql.SchemaConfig{
			Query: creator.RootQuery(),
		})
	})

	return schema, err
}
