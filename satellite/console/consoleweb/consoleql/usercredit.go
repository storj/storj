// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
)

const (
	// CreditUsageType is a graphql type for user credit
	CreditUsageType = "creditUsage"
	// FieldAvailableCreditInCent is a field name for available credit
	FieldAvailableCreditInCent = "availableCreditInCent"
	// FieldUsedCreditInCent is a field name for used credit
	FieldUsedCreditInCent = "usedCreditInCent"
	// FieldReferred is a field name for total referred number
	FieldReferred = "referred"
)

func graphqlCreditUsage() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: CreditUsageType,
		Fields: graphql.Fields{
			FieldAvailableCreditInCent: &graphql.Field{
				Type: graphql.Int,
			},
			FieldUsedCreditInCent: &graphql.Field{
				Type: graphql.Int,
			},
			FieldReferred: &graphql.Field{
				Type: graphql.Int,
			},
		},
	})
}
