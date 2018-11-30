// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"context"

	"github.com/graphql-go/graphql"

	"storj.io/storj/pkg/satellite"
)

const (
	userType      = "user"
	userInputType = "userInput"

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
func graphqlUser(service *satellite.Service, types Types) *graphql.Object {
	config := baseUserConfig()

	config.Fields.(graphql.Fields)[companyType] = &graphql.Field{
		Type: types.Company(),
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			user, _ := p.Source.(*satellite.User)

			// if root value contains context used instead one from params
			// as RootValue seems like the only way to pass additional from parent resolver
			rootValue := p.Info.RootValue.(map[string]interface{})

			ctx := rootValue["context"]
			if ctx != nil {
				return service.GetCompany(ctx.(context.Context), user.ID)
			}

			return service.GetCompany(p.Context, user.ID)
		},
	}

	return graphql.NewObject(config)
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
			//TODO(yar): separate creation of user and company
			companyType: &graphql.InputObjectFieldConfig{
				Type: types.CompanyInput(),
			},
		},
	})
}

// UserInput encapsulates satellite.UserInfo and satellite.CompanyInfo which is used in user related queries
type UserInput struct {
	User    satellite.UserInfo
	Company satellite.CompanyInfo
}

// fromMapUserInfo creates UserInput from input args
func fromMapUserInfo(args map[string]interface{}) (input UserInput) {
	input.User.Email, _ = args[fieldEmail].(string)
	input.User.FirstName, _ = args[fieldFirstName].(string)
	input.User.LastName, _ = args[fieldLastName].(string)
	input.User.Password, _ = args[fieldPassword].(string)

	companyArgs, ok := args[companyType].(map[string]interface{})
	if !ok {
		return
	}

	input.Company = fromMapCompanyInfo(companyArgs)
	return
}

// fillUserInfo fills satellite.UserInfo from satellite.User and input args
func fillUserInfo(user *satellite.User, args map[string]interface{}) satellite.UserInfo {
	info := satellite.UserInfo{
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Password:  "",
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
		case fieldPassword:
			info.Password = value
		}
	}

	return info
}
