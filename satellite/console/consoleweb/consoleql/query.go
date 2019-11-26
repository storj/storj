// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/rewards"
)

const (
	// Query is immutable graphql request
	Query = "query"
	// ProjectQuery is a query name for project
	ProjectQuery = "project"
	// MyProjectsQuery is a query name for projects related to account
	MyProjectsQuery = "myProjects"
	// ActiveRewardQuery is a query name for current active reward offer
	ActiveRewardQuery = "activeReward"
	// CreditUsageQuery is a query name for credit usage related to an user
	CreditUsageQuery = "creditUsage"
)

// rootQuery creates query for graphql populated by AccountsClient
func rootQuery(service *console.Service, mailService *mailservice.Service, types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Query,
		Fields: graphql.Fields{
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
		},
	})
}
