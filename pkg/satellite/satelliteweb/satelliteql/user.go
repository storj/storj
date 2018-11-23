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

// graphqlUser creates *graphql.Object type representation of satellite.User
func graphqlUser(service *satellite.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
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
			companyType: &graphql.Field{
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
			},
		},
	})
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
