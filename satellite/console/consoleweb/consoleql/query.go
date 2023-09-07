// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
)

const (
	// Query is immutable graphql request.
	Query = "query"
	// ProjectQuery is a query name for project.
	ProjectQuery = "project"
	// OwnedProjectsQuery is a query name for projects owned by an account.
	OwnedProjectsQuery = "ownedProjects"
	// MyProjectsQuery is a query name for projects related to account.
	MyProjectsQuery = "myProjects"
)

// rootQuery creates query for graphql populated by AccountsClient.
func rootQuery(service *console.Service, mailService *mailservice.Service, types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Query,
		Fields: graphql.Fields{
			ProjectQuery: &graphql.Field{
				Type: types.project,
				Args: graphql.FieldConfigArgument{
					FieldID: &graphql.ArgumentConfig{
						Type:         graphql.String,
						DefaultValue: "",
					},
					FieldPublicID: &graphql.ArgumentConfig{
						Type:         graphql.String,
						DefaultValue: "",
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {

					projectID, err := getProjectID(p)
					if err != nil {
						return nil, err
					}

					project, err := service.GetProject(p.Context, projectID)
					if err != nil {
						return nil, err
					}

					return project, nil
				},
			},
			OwnedProjectsQuery: &graphql.Field{
				Type: types.projectsPage,
				Args: graphql.FieldConfigArgument{
					CursorArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.projectsCursor),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					cursor := fromMapProjectsCursor(p.Args[CursorArg].(map[string]interface{}))
					page, err := service.GetUsersOwnedProjectsPage(p.Context, cursor)
					return page, err
				},
			},
			MyProjectsQuery: &graphql.Field{
				Type: graphql.NewList(types.project),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					projects, err := service.GetUsersProjects(p.Context)
					if err != nil {
						return nil, err
					}

					return projects, nil
				},
			},
		},
	})
}
