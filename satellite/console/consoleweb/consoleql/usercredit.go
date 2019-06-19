// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
)

const (
	UserCreditType       = "creditUsage"
	FieldAvailableCredit = "availableCredit"
	FieldUsedCredit      = "usedCredit"
	FieldReferred        = "referred"
)

func graphqlCreditUsage() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: UserCreditType,
		Fields: graphql.Fields{
			FieldAvailableCredit: &graphql.Field{
				Type: graphql.Int,
			},
			FieldUsedCredit: &graphql.Field{
				Type: graphql.Int,
			},
			FieldReferred: &graphql.Field{
				Type: graphql.Int,
			},
		},
	})
}
