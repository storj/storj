// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/satellite"
)

const (
	// Mutation is graphql request that modifies data
	Mutation = "mutation"

	createUserMutation     = "createUser"
	updateAccountMutation  = "updateAccount"
	deleteAccountMutation  = "deleteAccount"
	changePasswordMutation = "changePassword"

	createProjectMutation            = "createProject"
	deleteProjectMutation            = "deleteProject"
	updateProjectDescriptionMutation = "updateProjectDescription"

	addProjectMembersMutation    = "addProjectMembers"
	deleteProjectMembersMutation = "deleteProjectMembers"

	createAPIKeyMutation = "createAPIKey"
	deleteAPIKeyMutation = "deleteAPIKey"

	input = "input"

	fieldProjectID = "projectID"

	fieldNewPassword = "newPassword"
)

// rootMutation creates mutation for graphql populated by AccountsClient
func rootMutation(service *satellite.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Mutation,
		Fields: graphql.Fields{
			createUserMutation: &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					input: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.UserInput()),
					},
				},
				// creates user and company from input params and returns userID if succeed
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					input, _ := p.Args[input].(map[string]interface{})
					createUser := fromMapCreateUser(input)

					user, err := service.CreateUser(p.Context, createUser)
					if err != nil {
						return "", err
					}

					return user.ID.String(), nil
				},
			},
			updateAccountMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					input: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.UserInput()),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					input, _ := p.Args[input].(map[string]interface{})

					auth, err := satellite.GetAuth(p.Context)
					if err != nil {
						return nil, err
					}

					info := fillUserInfo(&auth.User, input)

					err = service.UpdateAccount(p.Context, info)
					if err != nil {
						return nil, err
					}

					return auth.User, nil
				},
			},
			changePasswordMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					fieldPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldNewPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pass, _ := p.Args[fieldPassword].(string)
					newPass, _ := p.Args[fieldNewPassword].(string)

					auth, err := satellite.GetAuth(p.Context)
					if err != nil {
						return nil, err
					}

					err = service.ChangePassword(p.Context, pass, newPass)
					if err != nil {
						return nil, err
					}

					return auth.User, nil
				},
			},
			deleteAccountMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					fieldPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					password, _ := p.Args[fieldPassword].(string)

					auth, err := satellite.GetAuth(p.Context)
					if err != nil {
						return nil, err
					}

					err = service.DeleteAccount(p.Context, password)
					if err != nil {
						return nil, err
					}

					return auth.User, nil
				},
			},
			// creates project from input params
			createProjectMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					input: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.ProjectInput()),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					var projectInput = fromMapProjectInfo(p.Args[input].(map[string]interface{}))

					return service.CreateProject(p.Context, projectInput)
				},
			},
			// deletes project by id, taken from input params
			deleteProjectMutation: &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					fieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					inputID := p.Args[fieldID].(string)
					projectID, err := uuid.Parse(inputID)
					if err != nil {
						return nil, err
					}

					return nil, service.DeleteProject(p.Context, *projectID)
				},
			},
			// updates project description
			updateProjectDescriptionMutation: &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					fieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldDescription: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					description := p.Args[fieldDescription].(string)

					inputID := p.Args[fieldID].(string)
					projectID, err := uuid.Parse(inputID)
					if err != nil {
						return nil, err
					}

					return service.UpdateProject(p.Context, *projectID, description)
				},
			},
			// add user as member of given project
			addProjectMembersMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					fieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldEmail: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pID, _ := p.Args[fieldProjectID].(string)
					emails, _ := p.Args[fieldEmail].([]interface{})

					projectID, err := uuid.Parse(pID)
					if err != nil {
						return nil, err
					}

					var userEmails []string
					for _, email := range emails {
						userEmails = append(userEmails, email.(string))
					}

					err = service.AddProjectMembers(p.Context, *projectID, userEmails)
					if err != nil {
						return nil, err
					}

					return service.GetProject(p.Context, *projectID)
				},
			},
			// delete user membership for given project
			deleteProjectMembersMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					fieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldEmail: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pID, _ := p.Args[fieldProjectID].(string)
					emails, _ := p.Args[fieldEmail].([]interface{})

					projectID, err := uuid.Parse(pID)
					if err != nil {
						return nil, err
					}

					var userEmails []string
					for _, email := range emails {
						userEmails = append(userEmails, email.(string))
					}

					err = service.DeleteProjectMembers(p.Context, *projectID, userEmails)
					if err != nil {
						return nil, err
					}

					return service.GetProject(p.Context, *projectID)
				},
			},
			// creates new api key
			createAPIKeyMutation: &graphql.Field{
				Type: types.APIKey(),
				Args: graphql.FieldConfigArgument{
					fieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldName: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					projectID, _ := p.Args[fieldProjectID].(string)
					name, _ := p.Args[fieldName].(string)

					pID, err := uuid.Parse(projectID)
					if err != nil {
						return nil, err
					}

					key, err := service.CreateAPIKey(p.Context, *pID, name)
					if err != nil {
						return nil, err
					}

					return key, nil
				},
			},
			// deletes api key
			deleteAPIKeyMutation: &graphql.Field{
				Type: types.APIKey(),
				Args: graphql.FieldConfigArgument{
					fieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					keyID, _ := p.Args[fieldID].(string)

					id, err := uuid.Parse(keyID)
					if err != nil {
						return nil, err
					}

					key, err := service.GetAPIKey(p.Context, *id)
					if err != nil {
						return nil, err
					}

					err = service.DeleteAPIKey(p.Context, *id)
					if err != nil {
						return nil, err
					}

					return key, nil
				},
			},
		},
	})
}
