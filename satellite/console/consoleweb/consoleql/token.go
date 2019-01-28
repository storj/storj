// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/satellite/console"
)

const (
	// TokenType is graphql type name for token
	TokenType = "token"
)

// graphqlToken creates *graphql.Object type that encapsulates user and token string
func graphqlToken(service *console.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: TokenType,
		Fields: graphql.Fields{
			TokenType: &graphql.Field{
				Type: graphql.String,
			},
			UserType: &graphql.Field{
				Type: types.User(),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					wrapper, _ := p.Source.(tokenWrapper)

					ctx := auth.WithAPIKey(p.Context, []byte(wrapper.Token))

					auth, err := service.Authorize(ctx)
					if err != nil {
						return nil, err
					}

					// pass context to root value so child resolvers could get auth auth
					rootValue := p.Info.RootValue.(map[string]interface{})
					rootValue["context"] = console.WithAuth(ctx, auth)

					return &auth.User, nil
				},
			},
		},
	})
}

// tokenWrapper holds token string value so it can be parsed by graphql pkg
type tokenWrapper struct {
	Token string
}
