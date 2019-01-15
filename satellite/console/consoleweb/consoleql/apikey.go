// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
	"storj.io/storj/satellite/console"
)

const (
	apiKeyInfoType   = "keyInfo"
	createAPIKeyType = "graphqlCreateAPIKey"

	fieldKey = "key"
)

// graphqlAPIKeyInfo creates satellite.APIKeyInfo graphql object
func graphqlAPIKeyInfo() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: apiKeyInfoType,
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
			fieldCreatedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
		},
	})
}

// graphqlCreateAPIKey creates createAPIKey graphql object
func graphqlCreateAPIKey(types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: createAPIKeyType,
		Fields: graphql.Fields{
			fieldKey: &graphql.Field{
				Type: graphql.String,
			},
			apiKeyInfoType: &graphql.Field{
				Type: types.APIKeyInfo(),
			},
		},
	})
}

// createAPIKey holds satellite.APIKey and satellite.APIKeyInfo
type createAPIKey struct {
	Key     *console.APIKey
	KeyInfo *console.APIKeyInfo
}
