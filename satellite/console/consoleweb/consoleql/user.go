// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/satellite/console"
)

const (
	// UserType is a graphql type for user
	UserType = "user"
	// UserInputType is a graphql type for user input
	UserInputType = "userInput"
	// FieldID is a field name for id
	FieldID = "id"
	// FieldEmail is a field name for email
	FieldEmail = "email"
	// FieldPassword is a field name for password
	FieldPassword = "password"
	// FieldFirstName is a field name for "first name"
	FieldFirstName = "firstName"
	// FieldLastName is a field name for "last name"
	FieldLastName = "lastName"
	// FieldCreatedAt is a field name for created at timestamp
	FieldCreatedAt = "createdAt"
)

// base graphql config for user
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
			FieldFirstName: &graphql.Field{
				Type: graphql.String,
			},
			FieldLastName: &graphql.Field{
				Type: graphql.String,
			},
			FieldCreatedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
		},
	}
}

// graphqlUser creates *graphql.Object type representation of satellite.User
// TODO: simplify
func graphqlUser() *graphql.Object {
	return graphql.NewObject(baseUserConfig())
}

// graphqlUserInput creates graphql.InputObject type needed to register/update satellite.User
func graphqlUserInput(types Types) *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: UserInputType,
		Fields: graphql.InputObjectConfigFieldMap{
			FieldEmail: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			FieldFirstName: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			FieldLastName: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			FieldPassword: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
		},
	})
}

// fromMapUserInfo creates UserInput from input args
func fromMapUserInfo(args map[string]interface{}) (user console.UserInfo) {
	user.Email, _ = args[FieldEmail].(string)
	user.FirstName, _ = args[FieldFirstName].(string)
	user.LastName, _ = args[FieldLastName].(string)
	return
}

func fromMapCreateUser(args map[string]interface{}) (user console.CreateUser) {
	user.UserInfo = fromMapUserInfo(args)
	user.Password, _ = args[FieldPassword].(string)
	return
}

// fillUserInfo fills satellite.UserInfo from satellite.User and input args
func fillUserInfo(user *console.User, args map[string]interface{}) console.UserInfo {
	info := console.UserInfo{
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}

	for fieldName, fieldValue := range args {
		value, ok := fieldValue.(string)
		if !ok {
			continue
		}

		switch fieldName {
		case FieldEmail:
			info.Email = value
			user.Email = value
		case FieldFirstName:
			info.FirstName = value
			user.FirstName = value
		case FieldLastName:
			info.LastName = value
			user.LastName = value
		}
	}

	return info
}
