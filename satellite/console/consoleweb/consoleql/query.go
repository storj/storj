// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"errors"
	"fmt"

	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"storj.io/storj/internal/post"
	"storj.io/storj/satellite/mailservice"

	"storj.io/storj/satellite/console"
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
	// TokenQuery is a query name for token
	TokenQuery = "token"
	// ForgotPasswordQuery is a query name for password recovery request
	ForgotPasswordQuery = "forgotPassword"
)

// rootQuery creates query for graphql populated by AccountsClient
func rootQuery(service *console.Service, mailService *mailservice.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Query,
		Fields: graphql.Fields{
			UserQuery: &graphql.Field{
				Type: types.User(),
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

					return service.GetUser(p.Context, *id)
				},
			},
			ProjectQuery: &graphql.Field{
				Type: types.Project(),
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
				Type: graphql.NewList(types.Project()),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return service.GetUsersProjects(p.Context)
				},
			},
			TokenQuery: &graphql.Field{
				Type: types.Token(),
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

					recoveryToken, err := service.GeneratePasswordRecoveryToken(p.Context, user.ID, user.Email)
					if err != nil {
						return false, errors.New("failed to generate password recovery token")
					}

					rootObject := p.Info.RootValue.(map[string]interface{})
					origin := rootObject["origin"].(string)
					link := origin + rootObject[PasswordRecoveryPath].(string) + recoveryToken
					userName := user.ShortName
					if user.ShortName == "" {
						userName = user.FullName
					}

					// TODO: think of a better solution
					go func() {
						_ = mailService.SendRendered(
							p.Context,
							[]post.Address{{Address: user.Email, Name: userName}},
							&ForgotPasswordEmail{
								Origin:    origin,
								ResetLink: link,
								UserName:  userName,
							},
						)
					}()

					return true, nil
				},
			},
		},
	})
}
