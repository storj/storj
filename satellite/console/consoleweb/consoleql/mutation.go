// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
)

const (
	// Mutation is graphql request that modifies data.
	Mutation = "mutation"

	// CreateProjectMutation is a mutation name for project creation.
	CreateProjectMutation = "createProject"
	// DeleteProjectMutation is a mutation name for project deletion.
	DeleteProjectMutation = "deleteProject"
	// UpdateProjectMutation is a mutation name for project name and description updating.
	UpdateProjectMutation = "updateProject"

	// AddProjectMembersMutation is a mutation name for adding new project members.
	AddProjectMembersMutation = "addProjectMembers"
	// DeleteProjectMembersMutation is a mutation name for deleting project members.
	DeleteProjectMembersMutation = "deleteProjectMembers"

	// CreateAPIKeyMutation is a mutation name for api key creation.
	CreateAPIKeyMutation = "createAPIKey"
	// DeleteAPIKeysMutation is a mutation name for api key deleting.
	DeleteAPIKeysMutation = "deleteAPIKeys"

	// AddPaymentMethodMutation is mutation name for adding new payment method.
	AddPaymentMethodMutation = "addPaymentMethod"
	// DeletePaymentMethodMutation is mutation name for deleting payment method.
	DeletePaymentMethodMutation = "deletePaymentMethod"
	// SetDefaultPaymentMethodMutation is mutation name setting payment method as default payment method.
	SetDefaultPaymentMethodMutation = "setDefaultPaymentMethod"

	// InputArg is argument name for all input types.
	InputArg = "input"
	// ProjectFields is a field name for project specific fields.
	ProjectFields = "projectFields"
	// ProjectLimits is a field name for project specific limits.
	ProjectLimits = "projectLimits"
	// FieldProjectID is field name for projectID.
	FieldProjectID = "projectID"
	// FieldNewPassword is a field name for new password.
	FieldNewPassword = "newPassword"
	// Secret is a field name for registration token for user creation during Vanguard release.
	Secret = "secret"
	// ReferrerUserID is a field name for passing referrer's user id.
	ReferrerUserID = "referrerUserId"
)

// rootMutation creates mutation for graphql populated by AccountsClient.
func rootMutation(log *zap.Logger, service *console.Service, mailService *mailservice.Service, types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Mutation,
		Fields: graphql.Fields{
			// creates project from input params
			CreateProjectMutation: &graphql.Field{
				Type: types.project,
				Args: graphql.FieldConfigArgument{
					InputArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.projectInput),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					var projectInput = fromMapProjectInfo(p.Args[InputArg].(map[string]interface{}))

					project, err := service.CreateProject(p.Context, projectInput)
					if err != nil {
						return nil, err
					}

					return project, nil
				},
			},
			// deletes project by id, taken from input params
			DeleteProjectMutation: &graphql.Field{
				Type: types.project,
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return nil, console.ErrUnauthorized.New("not implemented")
				},
			},
			// updates project name and description.
			UpdateProjectMutation: &graphql.Field{
				Type: types.project,
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					ProjectFields: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.projectInput),
					},
					ProjectLimits: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.projectLimit),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					var projectInput, err = fromMapProjectInfoProjectLimits(p.Args[ProjectFields].(map[string]interface{}), p.Args[ProjectLimits].(map[string]interface{}))
					if err != nil {
						return nil, err
					}

					inputID := p.Args[FieldID].(string)
					projectID, err := uuid.FromString(inputID)
					if err != nil {
						return nil, err
					}

					project, err := service.UpdateProject(p.Context, projectID, projectInput)
					if err != nil {
						return nil, err
					}

					return project, nil
				},
			},
			// add user as member of given project
			AddProjectMembersMutation: &graphql.Field{
				Type: types.project,
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

					projectID, err := uuid.FromString(pID)
					if err != nil {
						return nil, err
					}

					var userEmails []string
					for _, email := range emails {
						userEmails = append(userEmails, email.(string))
					}

					project, err := service.GetProject(p.Context, projectID)
					if err != nil {
						return nil, err
					}

					users, err := service.AddProjectMembers(p.Context, projectID, userEmails)
					if err != nil {
						return nil, err
					}

					rootObject := p.Info.RootValue.(map[string]interface{})
					origin := rootObject["origin"].(string)
					signIn := origin + rootObject[SignInPath].(string)

					for _, user := range users {
						userName := user.ShortName
						if user.ShortName == "" {
							userName = user.FullName
						}

						contactInfoURL := rootObject[ContactInfoURL].(string)
						letUsKnowURL := rootObject[LetUsKnowURL].(string)
						termsAndConditionsURL := rootObject[TermsAndConditionsURL].(string)

						mailService.SendRenderedAsync(
							p.Context,
							[]post.Address{{Address: user.Email, Name: userName}},
							&console.ProjectInvitationEmail{
								Origin:                origin,
								UserName:              userName,
								ProjectName:           project.Name,
								SignInLink:            signIn,
								LetUsKnowURL:          letUsKnowURL,
								TermsAndConditionsURL: termsAndConditionsURL,
								ContactInfoURL:        contactInfoURL,
							},
						)
					}

					return project, nil
				},
			},
			// delete user membership for given project
			DeleteProjectMembersMutation: &graphql.Field{
				Type: types.project,
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

					projectID, err := uuid.FromString(pID)
					if err != nil {
						return nil, err
					}

					var userEmails []string
					for _, email := range emails {
						userEmails = append(userEmails, email.(string))
					}

					project, err := service.GetProject(p.Context, projectID)
					if err != nil {
						return nil, err
					}

					err = service.DeleteProjectMembers(p.Context, project.ID, userEmails)
					if err != nil {
						return nil, err
					}

					return project, nil
				},
			},
			// creates new api key
			CreateAPIKeyMutation: &graphql.Field{
				Type: types.createAPIKey,
				Args: graphql.FieldConfigArgument{
					FieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldName: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					projectIDField, _ := p.Args[FieldProjectID].(string)
					name, _ := p.Args[FieldName].(string)

					projectID, err := uuid.FromString(projectIDField)
					if err != nil {
						return nil, err
					}

					info, key, err := service.CreateAPIKey(p.Context, projectID, name)
					if err != nil {
						return nil, err
					}

					return createAPIKey{
						Key:     key.Serialize(),
						KeyInfo: info,
					}, nil
				},
			},
			// deletes api key
			DeleteAPIKeysMutation: &graphql.Field{
				Type: graphql.NewList(types.apiKeyInfo),
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
						keyID, err := uuid.FromString(id.(string))
						if err != nil {
							return nil, err
						}

						key, err := service.GetAPIKeyInfo(p.Context, keyID)
						if err != nil {
							return nil, err
						}

						keyIds = append(keyIds, keyID)
						keys = append(keys, *key)
					}

					err := service.DeleteAPIKeys(p.Context, keyIds)
					if err != nil {
						return nil, err
					}

					return keys, nil
				},
			},
			AddPaymentMethodMutation: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return nil, nil
				},
			},
			DeletePaymentMethodMutation: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return nil, nil
				},
			},
			SetDefaultPaymentMethodMutation: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return nil, nil
				},
			},
		},
	})
}
