// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storjql

import (
	"sync"

	"github.com/graphql-go/graphql"

	"storj.io/storj/bootstrap/bootstrapweb"
	"storj.io/storj/bootstrap/bootstrapweb/bootstrapserver/bootstrapql"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/mailservice"
)

// creatingSchemaMutex locks graphql.NewSchema method because it's not thread-safe
var creatingSchemaMutex sync.Mutex

// CreateBootstrapSchema creates both type
func CreateBootstrapSchema(service *bootstrapweb.Service) (graphql.Schema, error) {
	creatingSchemaMutex.Lock()
	defer creatingSchemaMutex.Unlock()

	creator := bootstrapql.TypeCreator{}
	err := creator.Create(service)
	if err != nil {
		return graphql.Schema{}, err
	}

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: creator.RootQuery(),
	})
}

// CreateConsoleSchema creates both type
func CreateConsoleSchema(service *console.Service, mailService *mailservice.Service) (graphql.Schema, error) {
	creatingSchemaMutex.Lock()
	defer creatingSchemaMutex.Unlock()

	creator := consoleql.TypeCreator{}
	err := creator.Create(service, mailService)
	if err != nil {
		return graphql.Schema{}, err
	}

	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    creator.RootQuery(),
		Mutation: creator.RootMutation(),
	})
}
