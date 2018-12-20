// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/satellite"
	"storj.io/storj/pkg/utils"
)

const (
	// Mutation is graphql request that modifies data
	Mutation = "mutation"

	createUserMutation         = "createUser"
	updateUserMutation         = "updateUser"
	deleteUserMutation         = "deleteUser"
	changeUserPasswordMutation = "changeUserPassword"

	createProjectMutation            = "createProject"
	deleteProjectMutation            = "deleteProject"
	updateProjectDescriptionMutation = "updateProjectDescription"

	addProjectMemberMutation    = "addProjectMember"
	deleteProjectMemberMutation = "deleteProjectMember"

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
			updateUserMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					fieldID: &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					input: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.UserInput()),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id, err := uuidIDAuthFallback(p, fieldID)
					if err != nil {
						return nil, err
					}

					input, _ := p.Args[input].(map[string]interface{})

					user, err := service.GetUser(p.Context, *id)
					if err != nil {
						return nil, err
					}

					updatedUser := *user
					info := fillUserInfo(&updatedUser, input)

					err = service.UpdateUser(p.Context, *id, info)
					if err != nil {
						return user, err
					}

					return &updatedUser, nil
				},
			},
			changeUserPasswordMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					fieldID: &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					fieldPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldNewPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id, err := uuidIDAuthFallback(p, fieldID)
					if err != nil {
						return nil, err
					}

					pass, _ := p.Args[fieldPassword].(string)
					newPass, _ := p.Args[fieldNewPassword].(string)

					err = service.ChangeUserPassword(p.Context, *id, pass, newPass)
					user, getErr := service.GetUser(p.Context, *id)
					return user, utils.CombineErrors(err, getErr)
				},
			},
			deleteUserMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					fieldID: &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					fieldPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id, err := uuidIDAuthFallback(p, fieldID)
					if err != nil {
						return nil, err
					}

					password, _ := p.Args[fieldPassword].(string)

					user, err := service.GetUser(p.Context, *id)
					if err != nil {
						return nil, err
					}

					err = service.DeleteUser(p.Context, *id, password)
					return user, err
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
			addProjectMemberMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					fieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldUserID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pID, _ := p.Args[fieldProjectID].(string)
					uID, _ := p.Args[fieldUserID].([]interface{})

					projectID, pErr := uuid.Parse(pID)

					var userIDs []*uuid.UUID
					var userErr errs.Group

					for _, userID := range uID {
						id, err := uuid.Parse(userID.(string))
						if err != nil {
							userErr.Add(err)
							continue
						}

						userIDs = append(userIDs, id)
					}

					err := errs.Combine(pErr, userErr.Err())
					if err != nil {
						return nil, err
					}

					var addMemberErr errs.Group
					for _, userID := range userIDs {
						err = service.AddProjectMember(p.Context, *projectID, *userID)
						addMemberErr.Add(err)
					}

					if err = addMemberErr.Err(); err != nil {
						return nil, err
					}

					return service.GetProject(p.Context, *projectID)
				},
			},
			// delete user membership for given project
			deleteProjectMemberMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					fieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					fieldUserID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pID, _ := p.Args[fieldProjectID].(string)
					uID, _ := p.Args[fieldUserID].([]interface{})

					projectID, pErr := uuid.Parse(pID)

					var userIDs []*uuid.UUID
					var userErr errs.Group

					for _, userID := range uID {
						id, err := uuid.Parse(userID.(string))
						if err != nil {
							userErr.Add(err)
							continue
						}

						userIDs = append(userIDs, id)
					}

					err := errs.Combine(pErr, userErr.Err())
					if err != nil {
						return nil, err
					}

					var deleteMemberErr errs.Group
					for _, userID := range userIDs {
						err = service.DeleteProjectMember(p.Context, *projectID, *userID)
						deleteMemberErr.Add(err)
					}

					if err = deleteMemberErr.Err(); err != nil {
						return nil, err
					}

					return service.GetProject(p.Context, *projectID)
				},
			},
		},
	})
}
