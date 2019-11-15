// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"errors"

	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/private/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/rewards"
)

const (
	// Query is immutable graphql request
	Query = "query"
	// UserQuery is a query name for user
	UserQuery = "user"
	// ProjectQuery is a query name for project
	ProjectQuery = "project"
	// MyProjectsQuery is a query name for projects related to account
	MyProjectsQuery = "myProjects"
	// ActiveRewardQuery is a query name for current active reward offer
	ActiveRewardQuery = "activeReward"
	// CreditUsageQuery is a query name for credit usage related to an user
	CreditUsageQuery = "creditUsage"
	// TokenQuery is a query name for token
	TokenQuery = "token"
	// ForgotPasswordQuery is a query name for password recovery request
	ForgotPasswordQuery = "forgotPassword"
	// ResendAccountActivationEmailQuery is a query name for password recovery request
	ResendAccountActivationEmailQuery = "resendAccountActivationEmail"
)

// rootQuery creates query for graphql populated by AccountsClient
func rootQuery(service *console.Service, mailService *mailservice.Service, types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Query,
		Fields: graphql.Fields{
			UserQuery: &graphql.Field{
				Type: types.user,
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id, err := uuidIDAuthFallback(p, FieldID)
					if err != nil {
						return nil, HandleError(err)
					}
					_, err = console.GetAuth(p.Context)
					if err != nil {
						return nil, HandleError(err)
					}

					user, err := service.GetUser(p.Context, *id)
					if err != nil {
						return nil, HandleError(err)
					}

					return user, nil
				},
			},
			ProjectQuery: &graphql.Field{
				Type: types.project,
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					inputID, _ := p.Args[FieldID].(string)

					id, err := uuid.Parse(inputID)
					if err != nil {
						return nil, err
					}

					project, err := service.GetProject(p.Context, *id)
					if err != nil {
						return nil, HandleError(err)
					}

					return project, nil
				},
			},
			MyProjectsQuery: &graphql.Field{
				Type: graphql.NewList(types.project),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					projects, err := service.GetUsersProjects(p.Context)
					if err != nil {
						return nil, HandleError(err)
					}

					return projects, nil
				},
			},
			ActiveRewardQuery: &graphql.Field{
				Type: types.reward,
				Args: graphql.FieldConfigArgument{
					FieldType: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					rewardType, _ := p.Args[FieldType].(int)

					offer, err := service.GetCurrentRewardByType(p.Context, rewards.OfferType(rewardType))
					if err != nil {
						return nil, HandleError(err)
					}

					return offer, nil
				},
			},
			CreditUsageQuery: &graphql.Field{
				Type: types.creditUsage,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					usage, err := service.GetUserCreditUsage(p.Context)
					if err != nil {
						return nil, HandleError(err)
					}

					return usage, nil
				},
			},
			TokenQuery: &graphql.Field{
				Type: types.token,
				Args: graphql.FieldConfigArgument{
					FieldEmail: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					FieldPassword: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					email, _ := p.Args[FieldEmail].(string)
					pass, _ := p.Args[FieldPassword].(string)

					token, err := service.Token(p.Context, email, pass)
					if err != nil {
						return nil, HandleError(err)
					}

					return tokenWrapper{Token: token}, nil
				},
			},
			ForgotPasswordQuery: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					FieldEmail: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					email, _ := p.Args[FieldEmail].(string)

					user, err := service.GetUserByEmail(p.Context, email)
					if err != nil {
						return true, nil
					}

					recoveryToken, err := service.GeneratePasswordRecoveryToken(p.Context, user.ID)
					if err != nil {
						return false, errors.New("failed to generate password recovery token")
					}

					rootObject := p.Info.RootValue.(map[string]interface{})
					origin := rootObject["origin"].(string)
					passwordRecoveryLink := origin + rootObject[PasswordRecoveryPath].(string) + recoveryToken
					cancelPasswordRecoveryLink := origin + rootObject[CancelPasswordRecoveryPath].(string) + recoveryToken
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
						&ForgotPasswordEmail{
							Origin:                     origin,
							ResetLink:                  passwordRecoveryLink,
							CancelPasswordRecoveryLink: cancelPasswordRecoveryLink,
							UserName:                   userName,
							LetUsKnowURL:               letUsKnowURL,
							TermsAndConditionsURL:      termsAndConditionsURL,
							ContactInfoURL:             contactInfoURL,
						},
					)

					return true, nil
				},
			},
			ResendAccountActivationEmailQuery: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id, _ := p.Args[FieldID].(string)

					userID, err := uuid.Parse(id)
					if err != nil {
						return false, err
					}

					user, err := service.GetUser(p.Context, *userID)
					if err != nil {
						return false, HandleError(err)
					}

					token, err := service.GenerateActivationToken(p.Context, user.ID, user.Email)
					if err != nil {
						return false, HandleError(err)
					}

					rootObject := p.Info.RootValue.(map[string]interface{})
					origin := rootObject["origin"].(string)
					link := origin + rootObject[ActivationPath].(string) + token
					userName := user.ShortName
					if user.ShortName == "" {
						userName = user.FullName
					}

					contactInfoURL := rootObject[ContactInfoURL].(string)
					termsAndConditionsURL := rootObject[TermsAndConditionsURL].(string)

					// TODO: think of a better solution
					mailService.SendRenderedAsync(
						p.Context,
						[]post.Address{{Address: user.Email, Name: userName}},
						&AccountActivationEmail{
							Origin:                origin,
							ActivationLink:        link,
							TermsAndConditionsURL: termsAndConditionsURL,
							ContactInfoURL:        contactInfoURL,
						},
					)

					return true, nil
				},
			},
		},
	})
}
