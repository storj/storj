// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"

	"storj.io/storj/internal/currency"
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

	// AddPaymentMethodMutation is mutation name for adding new payment method
	AddPaymentMethodMutation = "addPaymentMethod"
	// DeletePaymentMethodMutation is mutation name for deleting payment method
	DeletePaymentMethodMutation = "deletePaymentMethod"
	// SetDefaultPaymentMethodMutation is mutation name setting payment method as default payment method
	SetDefaultPaymentMethodMutation = "setDefaultPaymentMethod"

	// InputArg is argument name for all input types
	InputArg = "input"
	// CurrentReward is a field name for current reward offer
	CurrentReward = "currentReward"
	// FieldProjectID is field name for projectID
	FieldProjectID = "projectID"
	// FieldNewPassword is a field name for new password
	FieldNewPassword = "newPassword"
	// Secret is a field name for registration token for user creation during Vanguard release
	Secret = "secret"
	// ReferrerID is a field name for referrer user ID
	ReferrerID = "referrerID"
)

// rootMutation creates mutation for graphql populated by AccountsClient
func rootMutation(log *zap.Logger, service *console.Service, mailService *mailservice.Service, types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Mutation,
		Fields: graphql.Fields{
			CreateUserMutation: &graphql.Field{
				Type: types.user,
				Args: graphql.FieldConfigArgument{
					InputArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.userInput),
					},
					Secret: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					CurrentReward: &graphql.ArgumentConfig{
						Type: types.rewardInput,
					},
					FieldPartnerID: &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					ReferrerID: &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				// creates user and company from input params and returns userID if succeed
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					input, _ := p.Args[InputArg].(map[string]interface{})
					secretInput, _ := p.Args[Secret].(string)
					partnerInput := p.Args[FieldPartnerID].(string)
					referrerInput := p.Args[ReferrerID].(string)
					rewardInput := p.Args[CurrentReward].(map[string]interface{})
					reward, err := fromMapRewardInfo(rewardInput)
					if err != nil {
						log.Error("register: failed to parse current reward",
							zap.Error(err))

						return nil, err
					}

					createUser := fromMapCreateUser(input)

					secret, err := console.RegistrationSecretFromBase64(secretInput)
					if err != nil {
						log.Error("register: failed to parse secret",
							zap.String("rawSecret", secretInput),
							zap.Error(err))

						return nil, err
					}

					if len(partnerInput) > 1 {
						partnerID, err := uuid.Parse(partnerInput)
						if err != nil {
							return nil, err
						}
						createUser.PartnerID = *partnerID
					}

					user, err := service.CreateUser(p.Context, createUser, secret)
					if err != nil {
						log.Error("register: failed to create account",
							zap.String("rawSecret", secretInput),
							zap.Error(err))

						return nil, err
					}

					if len(referrerInput) > 1 {
						referrerID, err := uuid.Parse(referrerInput)
						if err != nil {
							log.Error("register: failed to parse referrer ID",
								zap.String("rawReferralID", referrerInput),
								zap.Error(err))

							return nil, err
						}

						// User can only earn credits after adding payment info. Therefore, we set the credits to 0 on registration
						reward.InviteeCredit = currency.Cents(0)

						err = service.RedeemRewards(p.Context, reward, referrerID, user.ID)
						if err != nil {
							log.Error("register: failed to redeem credits",
								zap.String("rawSecret", secretInput),
								zap.Error(err))

							return nil, err
						}
					}

					token, err := service.GenerateActivationToken(p.Context, user.ID, user.Email)
					if err != nil {
						log.Error("register: failed to generate activation token",
							zap.Stringer("id", user.ID),
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

					mailService.SendRenderedAsync(
						p.Context,
						[]post.Address{{Address: user.Email, Name: userName}},
						&AccountActivationEmail{
							Origin:         origin,
							ActivationLink: link,
						},
					)

					return user, nil
				},
			},
			UpdateAccountMutation: &graphql.Field{
				Type: types.user,
				Args: graphql.FieldConfigArgument{
					InputArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.userInput),
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
				Type: types.user,
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
				Type: types.user,
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
				Type: types.project,
				Args: graphql.FieldConfigArgument{
					InputArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.projectInput),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					var projectInput = fromMapProjectInfo(p.Args[InputArg].(map[string]interface{}))

					return service.CreateProject(p.Context, projectInput)
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
				Type: types.project,
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

					for _, user := range users {
						userName := user.ShortName
						if user.ShortName == "" {
							userName = user.FullName
						}

						mailService.SendRenderedAsync(
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
			AddPaymentMethodMutation: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					FieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldCardToken: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldIsDefault: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Boolean),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					projectID, _ := p.Args[FieldProjectID].(string)
					cardToken, _ := p.Args[FieldCardToken].(string)
					isDefault, _ := p.Args[FieldIsDefault].(bool)

					projID, err := uuid.Parse(projectID)
					if err != nil {
						return false, err
					}

					_, err = service.AddNewPaymentMethod(p.Context, cardToken, isDefault, *projID)
					if err != nil {
						return false, err
					}

					return true, nil
				},
			},
			DeletePaymentMethodMutation: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					fieldProjectPaymentID, _ := p.Args[FieldID].(string)

					paymentID, err := uuid.Parse(fieldProjectPaymentID)
					if err != nil {
						return false, err
					}

					err = service.DeleteProjectPaymentMethod(p.Context, *paymentID)
					if err != nil {
						return false, err
					}

					return true, nil
				},
			},
			SetDefaultPaymentMethodMutation: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					FieldProjectID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					fieldProjectID, _ := p.Args[FieldProjectID].(string)
					fieldProjectPaymentID, _ := p.Args[FieldID].(string)

					paymentID, err := uuid.Parse(fieldProjectPaymentID)
					if err != nil {
						return false, err
					}

					projectID, err := uuid.Parse(fieldProjectID)
					if err != nil {
						return false, err
					}

					err = service.SetDefaultPaymentMethod(p.Context, *paymentID, *projectID)
					if err != nil {
						return false, err
					}

					return true, nil
				},
			},
		},
	})
}
