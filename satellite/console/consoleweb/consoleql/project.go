// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"time"

	"github.com/graphql-go/graphql"

	"storj.io/storj/satellite/console"
)

const (
	// ProjectType is a graphql type name for project
	ProjectType = "project"
	// ProjectInputType is a graphql type name for project input
	ProjectInputType = "projectInput"
	// ProjectUsageType is a graphql type name for project usage
	ProjectUsageType = "projectUsage"
	// FieldName is a field name for "name"
	FieldName = "name"
	// FieldDescription is a field name for description
	FieldDescription = "description"
	// FieldMembers is field name for members
	FieldMembers = "members"
	// FieldAPIKeys is a field name for api keys
	FieldAPIKeys = "apiKeys"
	// FieldUsage is a field name for usage rollup
	FieldUsage = "usage"
	// FieldStorage is a field name for storage total
	FieldStorage = "storage"
	// FieldEgress is a field name for egress total
	FieldEgress = "egress"
	// FieldObjectsCount is a field name for objects count
	FieldObjectsCount = "objectsCount"
	// LimitArg is argument name for limit
	LimitArg = "limit"
	// OffsetArg is argument name for offset
	OffsetArg = "offset"
	// SearchArg is argument name for search
	SearchArg = "search"
	// OrderArg is argument name for order
	OrderArg = "order"
	// SinceArg marks start of the period
	SinceArg = "since"
	// BeforeArg marks end of the period
	BeforeArg = "before"
)

// graphqlProject creates *graphql.Object type representation of satellite.ProjectInfo
func graphqlProject(service *console.Service, types *TypeCreator) *graphql.Object {
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
				Type: graphql.NewList(types.projectMember),
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
				Type: graphql.NewList(types.apiKeyInfo),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*console.Project)

					return service.GetAPIKeysInfoByProjectID(p.Context, project.ID)
				},
			},
			FieldUsage: &graphql.Field{
				Type: types.projectUsage,
				Args: graphql.FieldConfigArgument{
					SinceArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.DateTime),
					},
					BeforeArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.DateTime),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*console.Project)

					since := p.Args[SinceArg].(time.Time)
					before := p.Args[BeforeArg].(time.Time)

					return service.GetProjectUsage(p.Context, project.ID, since, before)
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

// graphqlProjectUsage creates project usage graphql type
func graphqlProjectUsage() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: ProjectUsageType,
		Fields: graphql.Fields{
			FieldStorage: &graphql.Field{
				Type: graphql.Float,
			},
			FieldEgress: &graphql.Field{
				Type: graphql.Float,
			},
			FieldObjectsCount: &graphql.Field{
				Type: graphql.Float,
			},
			SinceArg: &graphql.Field{
				Type: graphql.DateTime,
			},
			BeforeArg: &graphql.Field{
				Type: graphql.DateTime,
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
