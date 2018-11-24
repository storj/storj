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

	createUserMutation = "createUser"

	input = "input"
)

// rootMutation creates mutation for graphql populated by AccountsClient
func rootMutation(service *satellite.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Mutation,
		Fields: graphql.Fields{
			createUserMutation: &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					input: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.UserInput()),
					},
				},
				// creates user and company from input params and returns userID if succeed
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					var userInput = fromMapUserInfo(p.Args[input].(map[string]interface{}))

					user, err := service.CreateUser(
						p.Context,
						userInput.User,
						userInput.Company,
					)

					if err != nil {
						return "", err
					}

					return user.ID.String(), nil
				},
			},
		},
	})
}
