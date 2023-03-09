// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/satellite/console"
)

const (
	// APIKeyInfoType is graphql type name for api key.
	APIKeyInfoType = "keyInfo"
	// CreateAPIKeyType is graphql type name for createAPIKey struct
	// which incapsulates the actual key and it's info.
	CreateAPIKeyType = "graphqlCreateAPIKey"
	// FieldKey is field name for the actual key in createAPIKey.
	FieldKey = "key"
)

// graphqlAPIKeyInfo creates satellite.APIKeyInfo graphql object.
func graphqlAPIKeyInfo() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: APIKeyInfoType,
		Fields: graphql.Fields{
			FieldID: &graphql.Field{
				Type: graphql.String,
			},
			FieldProjectID: &graphql.Field{
				Type: graphql.String,
			},
			FieldName: &graphql.Field{
				Type: graphql.String,
			},
			FieldCreatedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
		},
	})
}

// graphqlCreateAPIKey creates createAPIKey graphql object.
func graphqlCreateAPIKey(types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: CreateAPIKeyType,
		Fields: graphql.Fields{
			FieldKey: &graphql.Field{
				Type: graphql.String,
			},
			APIKeyInfoType: &graphql.Field{
				Type: types.apiKeyInfo,
			},
		},
	})
}

func graphqlAPIKeysCursor() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: APIKeysCursorInputType,
		Fields: graphql.InputObjectConfigFieldMap{
			SearchArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.String),
			},
			LimitArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
			PageArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
			OrderArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
			OrderDirectionArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
		},
	})
}

func graphqlAPIKeysPage(types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: APIKeysPageType,
		Fields: graphql.Fields{
			FieldAPIKeys: &graphql.Field{
				Type: graphql.NewList(types.apiKeyInfo),
			},
			SearchArg: &graphql.Field{
				Type: graphql.String,
			},
			LimitArg: &graphql.Field{
				Type: graphql.Int,
			},
			OrderArg: &graphql.Field{
				Type: graphql.Int,
			},
			OrderDirectionArg: &graphql.Field{
				Type: graphql.Int,
			},
			OffsetArg: &graphql.Field{
				Type: graphql.Int,
			},
			FieldPageCount: &graphql.Field{
				Type: graphql.Int,
			},
			FieldCurrentPage: &graphql.Field{
				Type: graphql.Int,
			},
			FieldTotalCount: &graphql.Field{
				Type: graphql.Int,
			},
		},
	})
}

// createAPIKey holds macaroon.APIKey and console.APIKeyInfo.
type createAPIKey struct {
	Key     string
	KeyInfo *console.APIKeyInfo
}

type apiKeysPage struct {
	APIKeys []console.APIKeyInfo

	Search         string
	Limit          uint
	Order          int
	OrderDirection int
	Offset         uint64

	PageCount   uint
	CurrentPage uint
	TotalCount  uint64
}
