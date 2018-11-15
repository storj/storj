// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/pkg/satellite"
)

const (
	// Mutation is graphql request that modifies data
	Mutation = "mutation"

	registerMutation = "register"
)

// rootMutation creates mutation for graphql populated by AccountsClient
func rootMutation(service *satellite.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Mutation,
		Fields: graphql.Fields{
			registerMutation: &graphql.Field{
				Type: types.UserType(),
				Args: graphql.FieldConfigArgument{
					fieldEmail: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldFirstName: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldLastName: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					email, _ := p.Args[fieldEmail].(string)
					password, _ := p.Args[fieldPassword].(string)
					firstName, _ := p.Args[fieldFirstName].(string)
					lastName, _ := p.Args[fieldLastName].(string)

					user, err := service.Register(
						p.Context,
						&satellite.User{
							Email:        email,
							FirstName:    firstName,
							LastName:     lastName,
							PasswordHash: []byte(password),
						},
					)

					if err != nil {
						return nil, err
					}

					return user, nil
				},
			},
		},
	})
}
