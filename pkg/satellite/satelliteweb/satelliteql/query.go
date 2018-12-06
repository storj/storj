// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"storj.io/storj/pkg/satellite"
)

const (
	// Query is immutable graphql request
	Query = "query"

	userQuery       = "user"
	projectQuery    = "project"
	myProjectsQuery = "myProjects"
	tokenQuery      = "token"
)

// rootQuery creates query for graphql populated by AccountsClient
func rootQuery(service *satellite.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Query,
		Fields: graphql.Fields{
			userQuery: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					fieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id, _ := p.Args[fieldID].(string)

					idBytes, err := uuid.Parse(id)
					if err != nil {
						return nil, err
					}

					return service.GetUser(p.Context, *idBytes)
				},
			},
			projectQuery: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					fieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					inputID, _ := p.Args[fieldID].(string)

					id, err := uuid.Parse(inputID)
					if err != nil {
						return nil, err
					}

					return service.GetProject(p.Context, *id)
				},
			},
			myProjectsQuery: &graphql.Field{
				Type: graphql.NewList(types.Project()),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return service.GetUsersProjects(p.Context)
				},
			},
			tokenQuery: &graphql.Field{
				Type: types.Token(),
				Args: graphql.FieldConfigArgument{
					fieldEmail: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					email, _ := p.Args[fieldEmail].(string)
					pass, _ := p.Args[fieldPassword].(string)

					token, err := service.Token(p.Context, email, pass)
					if err != nil {
						return nil, err
					}

					return tokenWrapper{Token: token}, nil
				},
			},
		},
	})
}
