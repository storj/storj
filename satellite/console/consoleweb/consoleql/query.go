// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"errors"
	"fmt"

	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/internal/post"
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
						return nil, err
					}
					_, err = console.GetAuth(p.Context)
					if err != nil {
						return nil, err
					}

					return service.GetUser(p.Context, *id)
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

					return service.GetProject(p.Context, *id)
				},
			},
			MyProjectsQuery: &graphql.Field{
				Type: graphql.NewList(types.project),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return service.GetUsersProjects(p.Context)
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

					return service.GetCurrentRewardByType(p.Context, rewards.OfferType(rewardType))
				},
			},
			CreditUsageQuery: &graphql.Field{
				Type: types.creditUsage,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return service.GetUserCreditUsage(p.Context)
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
						return nil, err
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
						return false, fmt.Errorf("%s is not found", email)
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

					mailService.SendRenderedAsync(
						p.Context,
						[]post.Address{{Address: user.Email, Name: userName}},
						&ForgotPasswordEmail{
							Origin:                     origin,
							ResetLink:                  passwordRecoveryLink,
							CancelPasswordRecoveryLink: cancelPasswordRecoveryLink,
							UserName:                   userName,
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
						return false, err
					}

					token, err := service.GenerateActivationToken(p.Context, user.ID, user.Email)
					if err != nil {
						return false, err
					}

					rootObject := p.Info.RootValue.(map[string]interface{})
					origin := rootObject["origin"].(string)
					link := origin + rootObject[ActivationPath].(string) + token
					userName := user.ShortName
					if user.ShortName == "" {
						userName = user.FullName
					}

					// TODO: think of a better solution
					mailService.SendRenderedAsync(
						p.Context,
						[]post.Address{{Address: user.Email, Name: userName}},
						&AccountActivationEmail{
							Origin:         origin,
							ActivationLink: link,
						},
					)

					return true, nil
				},
			},
		},
	})
}
