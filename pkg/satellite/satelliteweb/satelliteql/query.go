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

	userQuery  = "user"
	tokenQuery = "token"
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

					user, err := service.GetUser(p.Context, *idBytes)
					if err != nil {
						return nil, err
					}

					return user, nil
				},
			},
			tokenQuery: &graphql.Field{
				Type: graphql.String,
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

					return token, nil
				},
			},
		},
	})
}
