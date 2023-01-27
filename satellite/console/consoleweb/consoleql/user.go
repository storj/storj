// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
)

const (
	// UserType is a graphql type for user.
	UserType = "user"
	// UserInputType is a graphql type for user input.
	UserInputType = "userInput"
	// FieldID is a field name for id.
	FieldID = "id"
	// FieldEmail is a field name for email.
	FieldEmail = "email"
	// FieldPassword is a field name for password.
	FieldPassword = "password"
	// FieldFullName is a field name for "first name".
	FieldFullName = "fullName"
	// FieldShortName is a field name for "last name".
	FieldShortName = "shortName"
	// FieldCreatedAt is a field name for created at timestamp.
	FieldCreatedAt = "createdAt"
)

// base graphql config for user.
func baseUserConfig() graphql.ObjectConfig {
	return graphql.ObjectConfig{
		Name: UserType,
		Fields: graphql.Fields{
			FieldID: &graphql.Field{
				Type: graphql.String,
			},
			FieldEmail: &graphql.Field{
				Type: graphql.String,
			},
			FieldFullName: &graphql.Field{
				Type: graphql.String,
			},
			FieldShortName: &graphql.Field{
				Type: graphql.String,
			},
			FieldCreatedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
		},
	}
}

// graphqlUser creates *graphql.Object type representation of satellite.User.
func graphqlUser() *graphql.Object {
	// TODO: simplify
	return graphql.NewObject(baseUserConfig())
}

// graphqlUserInput creates graphql.InputObject type needed to register/update satellite.User.
func graphqlUserInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: UserInputType,
		Fields: graphql.InputObjectConfigFieldMap{
			FieldEmail: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			FieldFullName: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			FieldShortName: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			FieldPassword: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
		},
	})
}
