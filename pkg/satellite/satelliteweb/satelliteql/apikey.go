// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import "github.com/graphql-go/graphql"

const (
	apiKeyType = "apiKey"

	fieldKey = "key"
)

func graphqlAPIKey() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: apiKeyType,
		Fields: graphql.Fields{
			fieldID: &graphql.Field{
				Type: graphql.String,
			},
			fieldProjectID: &graphql.Field{
				Type: graphql.String,
			},
			fieldName: &graphql.Field{
				Type: graphql.String,
			},
			fieldKey: &graphql.Field{
				Type: graphql.String,
			},
			fieldCreatedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
		},
	})
}
