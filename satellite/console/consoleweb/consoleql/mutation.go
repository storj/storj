// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"

	"storj.io/storj/internal/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
)

const (
	// Mutation is graphql request that modifies data
	Mutation = "mutation"

	// CreateUserMutation is a user creation mutation name
	CreateUserMutation = "createUser"
	// UpdateAccountMutation is a mutation name for account updating
	UpdateAccountMutation = "updateAccount"
	// DeleteAccountMutation is a mutation name for account deletion
	DeleteAccountMutation = "deleteAccount"
	// ChangePasswordMutation is a mutation name for password changing
	ChangePasswordMutation = "changePassword"
	// CreateProjectMutation is a mutation name for project creation
	CreateProjectMutation = "createProject"
	// DeleteProjectMutation is a mutation name for project deletion
	DeleteProjectMutation = "deleteProject"
	// UpdateProjectDescriptionMutation is a mutation name for project updating
	UpdateProjectDescriptionMutation = "updateProjectDescription"

	// AddProjectMembersMutation is a mutation name for adding new project members
	AddProjectMembersMutation = "addProjectMembers"
	// DeleteProjectMembersMutation is a mutation name for deleting project members
	DeleteProjectMembersMutation = "deleteProjectMembers"

	// CreateAPIKeyMutation is a mutation name for api key creation
	CreateAPIKeyMutation = "createAPIKey"
	// DeleteAPIKeysMutation is a mutation name for api key deleting
	DeleteAPIKeysMutation = "deleteAPIKeys"

	// InputArg is argument name for all input types
	InputArg = "input"
	// FieldProjectID is field name for projectID
	FieldProjectID = "projectID"
	// FieldNewPassword is a field name for new password
	FieldNewPassword = "newPassword"
	// Secret is a field name for registration token for user creation during Vanguard release
	Secret = "secret"
)

// rootMutation creates mutation for graphql populated by AccountsClient
func rootMutation(log *zap.Logger, service *console.Service, mailService *mailservice.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Mutation,
		Fields: graphql.Fields{
			CreateUserMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					InputArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.UserInput()),
					},
					Secret: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				// creates user and company from input params and returns userID if succeed
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					input, _ := p.Args[InputArg].(map[string]interface{})
					secretInput, _ := p.Args[Secret].(string)
					createUser := fromMapCreateUser(input)

					secret, err := console.RegistrationSecretFromBase64(secretInput)
					if err != nil {
						return nil, err
					}

					user, err := service.CreateUser(p.Context, createUser, secret)
					if err != nil {
						return nil, err
					}

					token, err := service.GenerateActivationToken(p.Context, user.ID, user.Email)
					if err != nil {
						log.Error("register: failed to generate activation token",
							zap.String("id", user.ID.String()),
							zap.String("email", user.Email),
							zap.Error(err))

						return user, nil
					}

					rootObject := p.Info.RootValue.(map[string]interface{})
					origin := rootObject["origin"].(string)
					link := origin + rootObject[ActivationPath].(string) + token
					userName := user.ShortName
					if user.ShortName == "" {
						userName = user.FullName
					}

					// TODO: think of a better solution
					go func() {
						_ = mailService.SendRendered(
							p.Context,
							[]post.Address{{Address: user.Email, Name: userName}},
							&AccountActivationEmail{
								Origin:         origin,
								ActivationLink: link,
							},
						)
					}()

					return user, nil
				},
			},
			UpdateAccountMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					InputArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.UserInput()),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					input, _ := p.Args[InputArg].(map[string]interface{})

					auth, err := console.GetAuth(p.Context)
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
			ChangePasswordMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					FieldPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldNewPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pass, _ := p.Args[FieldPassword].(string)
					newPass, _ := p.Args[FieldNewPassword].(string)

					auth, err := console.GetAuth(p.Context)
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
			DeleteAccountMutation: &graphql.Field{
				Type: types.User(),
				Args: graphql.FieldConfigArgument{
					FieldPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					password, _ := p.Args[FieldPassword].(string)

					auth, err := console.GetAuth(p.Context)
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
			CreateProjectMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					InputArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.ProjectInput()),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					var projectInput = fromMapProjectInfo(p.Args[InputArg].(map[string]interface{}))

					return service.CreateProject(p.Context, projectInput)
				},
			},
			// deletes project by id, taken from input params
			DeleteProjectMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					inputID := p.Args[FieldID].(string)
					projectID, err := uuid.Parse(inputID)
					if err != nil {
						return nil, err
					}

					project, err := service.GetProject(p.Context, *projectID)
					if err != nil {
						return nil, err
					}

					if err = service.DeleteProject(p.Context, project.ID); err != nil {
						return nil, err
					}

					return project, nil
				},
			},
			// updates project description
			UpdateProjectDescriptionMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldDescription: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					description := p.Args[FieldDescription].(string)

					inputID := p.Args[FieldID].(string)
					projectID, err := uuid.Parse(inputID)
					if err != nil {
						return nil, err
					}

					return service.UpdateProject(p.Context, *projectID, description)
				},
			},
			// add user as member of given project
			AddProjectMembersMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					FieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldEmail: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pID, _ := p.Args[FieldProjectID].(string)
					emails, _ := p.Args[FieldEmail].([]interface{})

					projectID, err := uuid.Parse(pID)
					if err != nil {
						return nil, err
					}

					var userEmails []string
					for _, email := range emails {
						userEmails = append(userEmails, email.(string))
					}

					project, err := service.GetProject(p.Context, *projectID)
					if err != nil {
						return nil, err
					}

					users, err := service.AddProjectMembers(p.Context, *projectID, userEmails)
					if err != nil {
						return nil, err
					}

					rootObject := p.Info.RootValue.(map[string]interface{})
					origin := rootObject["origin"].(string)
					signIn := origin + rootObject[SignInPath].(string)

					// TODO: think of a better solution
					go func() {
						for _, user := range users {
							userName := user.ShortName
							if user.ShortName == "" {
								userName = user.FullName
							}

							_ = mailService.SendRendered(
								p.Context,
								[]post.Address{{Address: user.Email, Name: userName}},
								&ProjectInvitationEmail{
									Origin:      origin,
									UserName:    userName,
									ProjectName: project.Name,
									SignInLink:  signIn,
								},
							)
						}
					}()

					return project, nil
				},
			},
			// delete user membership for given project
			DeleteProjectMembersMutation: &graphql.Field{
				Type: types.Project(),
				Args: graphql.FieldConfigArgument{
					FieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldEmail: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					pID, _ := p.Args[FieldProjectID].(string)
					emails, _ := p.Args[FieldEmail].([]interface{})

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
			CreateAPIKeyMutation: &graphql.Field{
				Type: types.CreateAPIKey(),
				Args: graphql.FieldConfigArgument{
					FieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldName: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					projectID, _ := p.Args[FieldProjectID].(string)
					name, _ := p.Args[FieldName].(string)

					pID, err := uuid.Parse(projectID)
					if err != nil {
						return nil, err
					}

					info, key, err := service.CreateAPIKey(p.Context, *pID, name)
					if err != nil {
						return nil, err
					}

					return createAPIKey{
						Key:     key,
						KeyInfo: info,
					}, nil
				},
			},
			// deletes api key
			DeleteAPIKeysMutation: &graphql.Field{
				Type: graphql.NewList(types.APIKeyInfo()),
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					paramKeysID, _ := p.Args[FieldID].([]interface{})

					var keyIds []uuid.UUID
					var keys []console.APIKeyInfo
					for _, id := range paramKeysID {
						keyID, err := uuid.Parse(id.(string))
						if err != nil {
							return nil, err
						}

						key, err := service.GetAPIKeyInfo(p.Context, *keyID)
						if err != nil {
							return nil, err
						}

						keyIds = append(keyIds, *keyID)
						keys = append(keys, *key)
					}

					err := service.DeleteAPIKeys(p.Context, keyIds)
					if err != nil {
						return nil, err
					}

					return keys, nil
				},
			},
		},
	})
}
