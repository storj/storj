// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/satellite/console"
)

const (
	// ProjectType is a graphql type name for project
	ProjectType = "project"
	// ProjectInputType is a graphql type name for project input
	ProjectInputType = "projectInput"
	// FieldName is a field name for "name"
	FieldName = "name"
	// FieldDescription is a field name for description
	FieldDescription = "description"
	// FieldMembers is field name for members
	FieldMembers = "members"
	// FieldAPIKeys is a field name for api keys
	FieldAPIKeys = "apiKeys"

	// LimitArg is argument name for limit
	LimitArg = "limit"
	// OffsetArg is argument name for offset
	OffsetArg = "offset"
	// SearchArg is argument name for search
	SearchArg = "search"
	// OrderArg is argument name for order
	OrderArg = "order"
)

// graphqlProject creates *graphql.Object type representation of satellite.ProjectInfo
func graphqlProject(service *console.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: ProjectType,
		Fields: graphql.Fields{
			FieldID: &graphql.Field{
				Type: graphql.String,
			},
			FieldName: &graphql.Field{
				Type: graphql.String,
			},
			FieldDescription: &graphql.Field{
				Type: graphql.String,
			},
			FieldCreatedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
			FieldMembers: &graphql.Field{
				Type: graphql.NewList(types.ProjectMember()),
				Args: graphql.FieldConfigArgument{
					OffsetArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
					LimitArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
					SearchArg: &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					OrderArg: &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*console.Project)

					offs, _ := p.Args[OffsetArg].(int)
					lim, _ := p.Args[LimitArg].(int)
					search, _ := p.Args[SearchArg].(string)
					order, _ := p.Args[OrderArg].(int)

					pagination := console.Pagination{
						Limit:  lim,
						Offset: int64(offs),
						Search: search,
						Order:  console.ProjectMemberOrder(order),
					}

					members, err := service.GetProjectMembers(p.Context, project.ID, pagination)
					if err != nil {
						return nil, err
					}

					var users []projectMember
					for _, member := range members {
						user, err := service.GetUser(p.Context, member.MemberID)
						if err != nil {
							return nil, err
						}

						users = append(users, projectMember{
							User:     user,
							JoinedAt: member.CreatedAt,
						})
					}

					return users, nil
				},
			},
			FieldAPIKeys: &graphql.Field{
				Type: graphql.NewList(types.APIKeyInfo()),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*console.Project)

					return service.GetAPIKeysInfoByProjectID(p.Context, project.ID)
				},
			},
		},
	})
}

// graphqlProjectInput creates graphql.InputObject type needed to create/update satellite.Project
func graphqlProjectInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: ProjectInputType,
		Fields: graphql.InputObjectConfigFieldMap{
			FieldName: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			FieldDescription: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
		},
	})
}

// fromMapProjectInfo creates satellite.ProjectInfo from input args
func fromMapProjectInfo(args map[string]interface{}) (project console.ProjectInfo) {
	project.Name, _ = args[FieldName].(string)
	project.Description, _ = args[FieldDescription].(string)

	return
}
