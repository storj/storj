// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/internal/storjql"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
)

// CreateSchema creates a schema for satellites console graphql api
func CreateSchema(service *console.Service, mailService *mailservice.Service) (schema graphql.Schema, err error) {
	storjql.WithLock(func() {
		creator := TypeCreator{}

		err := creator.Create(service, mailService)
		if err != nil {
			return
		}

		schema, err = graphql.NewSchema(graphql.SchemaConfig{
			Query:    creator.RootQuery(),
			Mutation: creator.RootMutation(),
		})
	})

	return schema, err
}
