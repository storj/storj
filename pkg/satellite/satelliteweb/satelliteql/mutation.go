// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/satellite"
	"storj.io/storj/pkg/utils"
)

const (
	// Mutation is graphql request that modifies data
	Mutation = "mutation"

	createUserMutation = "createUser"
	updateUserMutation = "updateUser"
	deleteUserMutation = "deleteUser"

	updateCompanyMutation = "updateCompany"

	createProjectMutation            = "createProject"
	deleteProjectMutation            = "deleteProject"
	updateProjectDescriptionMutation = "updateProjectDescription"

	addProjectMemberMutation    = "addProjectMember"
	deleteProjectMemberMutation = "deleteProjectMember"

	input = "input"

	fieldProjectID = "projectID"
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
					//TODO(yar): separate creation of user and company
					userInput := fromMapUserInfo(p.Args[input].(map[string]interface{}))

					user, err := service.CreateUser(
						p.Context,
						userInput.User,
						userInput.Company,
					)

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
						Type: graphql.NewNonNull(graphql.String),
					},
					input: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.UserInput()),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id, _ := p.Args[fieldID].(string)
					input, _ := p.Args[input].(map[string]interface{})

					idBytes, err := uuid.Parse(id)
					if err != nil {
						return nil, err
					}

					user, err := service.GetUser(p.Context, *idBytes)
					if err != nil {
						return nil, err
					}

					updatedUser := *user
					info := fillUserInfo(&updatedUser, input)

					err = service.UpdateUser(p.Context, *idBytes, info)
					if err != nil {
						return user, err
					}

					return &updatedUser, nil
				},
			},
			deleteUserMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					fieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id, _ := p.Args[fieldID].(string)

					idBytes, err := uuid.Parse(id)
					if err != nil {
						return nil, err
					}

					user, err := service.GetUser(p.Context, *idBytes)
					if err != nil {
						return nil, err
					}

					err = service.DeleteUser(p.Context, *idBytes)
					return user, err
				},
			},
			updateCompanyMutation: &graphql.Field{
				Type: types.Company(),
				Args: graphql.FieldConfigArgument{
					fieldUserID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					input: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.CompanyInput()),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id, _ := p.Args[fieldUserID].(string)
					input, _ := p.Args[input].(map[string]interface{})

					idBytes, err := uuid.Parse(id)
					if err != nil {
						return nil, err
					}

					company, err := service.GetCompany(p.Context, *idBytes)
					if err != nil {
						return nil, err
					}

					updatedCompany := *company
					info := fillCompanyInfo(&updatedCompany, input)

					err = service.UpdateCompany(p.Context, *idBytes, info)
					if err != nil {
						return company, err
					}

					return &updatedCompany, nil
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
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pID, _ := p.Args[fieldProjectID].(string)
					uID, _ := p.Args[fieldUserID].(string)

					projectID, pErr := uuid.Parse(pID)
					userID, uErr := uuid.Parse(uID)

					err := utils.CombineErrors(pErr, uErr)
					if err != nil {
						return nil, err
					}

					err = service.AddProjectMember(p.Context, *projectID, *userID)
					project, getErr := service.GetProject(p.Context, *projectID)
					return project, utils.CombineErrors(err, getErr)
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
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pID, _ := p.Args[fieldProjectID].(string)
					uID, _ := p.Args[fieldUserID].(string)

					projectID, pErr := uuid.Parse(pID)
					userID, uErr := uuid.Parse(uID)

					err := utils.CombineErrors(pErr, uErr)
					if err != nil {
						return nil, err
					}

					err = service.DeleteProjectMember(p.Context, *projectID, *userID)
					project, getErr := service.GetProject(p.Context, *projectID)
					return project, utils.CombineErrors(err, getErr)
				},
			},
		},
	})
}
