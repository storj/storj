// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/pkg/satellite"
)

const (
	userType      = "user"
	userInputType = "userInput"

	fieldUserID    = "userID"
	fieldID        = "id"
	fieldEmail     = "email"
	fieldPassword  = "password"
	fieldFirstName = "firstName"
	fieldLastName  = "lastName"
	fieldCreatedAt = "createdAt"
)

// base graphql config for user
func baseUserConfig() graphql.ObjectConfig {
	return graphql.ObjectConfig{
		Name: userType,
		Fields: graphql.Fields{
			fieldID: &graphql.Field{
				Type: graphql.String,
			},
			fieldEmail: &graphql.Field{
				Type: graphql.String,
			},
			fieldFirstName: &graphql.Field{
				Type: graphql.String,
			},
			fieldLastName: &graphql.Field{
				Type: graphql.String,
			},
			fieldCreatedAt: &graphql.Field{
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
		Name: userInputType,
		Fields: graphql.InputObjectConfigFieldMap{
			fieldEmail: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldFirstName: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldLastName: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldPassword: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
		},
	})
}

// fromMapUserInfo creates UserInput from input args
func fromMapUserInfo(args map[string]interface{}) (user satellite.UserInfo) {
	user.Email, _ = args[fieldEmail].(string)
	user.FirstName, _ = args[fieldFirstName].(string)
	user.LastName, _ = args[fieldLastName].(string)
	return
}

func fromMapCreateUser(args map[string]interface{}) (user satellite.CreateUser) {
	user.UserInfo = fromMapUserInfo(args)
	user.Password, _ = args[fieldPassword].(string)
	return
}

// fillUserInfo fills satellite.UserInfo from satellite.User and input args
func fillUserInfo(user *satellite.User, args map[string]interface{}) satellite.UserInfo {
	info := satellite.UserInfo{
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
		case fieldEmail:
			info.Email = value
			user.Email = value
		case fieldFirstName:
			info.FirstName = value
			user.FirstName = value
		case fieldLastName:
			info.LastName = value
			user.LastName = value
		}
	}

	return info
}
