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
	// BucketUsageCursorInputType is a graphql input
	// type name for bucket usage cursor
	BucketUsageCursorInputType = "bucketUsageCursor"
	// BucketUsageType is a graphql type name for bucket usage
	BucketUsageType = "bucketUsage"
	// BucketUsagePageType is a field name for bucket usage page
	BucketUsagePageType = "bucketUsagePage"
	// ProjectMembersPageType is a field name for project members page
	ProjectMembersPageType = "projectMembersPage"
	// ProjectMembersCursorInputType is a graphql type name for project members
	ProjectMembersCursorInputType = "projectMembersCursor"
	// APIKeysPageType is a field name for api keys page
	APIKeysPageType = "apiKeysPage"
	// APIKeysCursorInputType is a graphql type name for api keys
	APIKeysCursorInputType = "apiKeysCursor"
	// FieldName is a field name for "name"
	FieldName = "name"
	// FieldBucketName is a field name for "bucket name"
	FieldBucketName = "bucketName"
	// FieldDescription is a field name for description
	FieldDescription = "description"
	// FieldMembers is field name for members
	FieldMembers = "members"
	// FieldAPIKeys is a field name for api keys
	FieldAPIKeys = "apiKeys"
	// FieldUsage is a field name for usage rollup
	FieldUsage = "usage"
	// FieldBucketUsages is a field name for bucket usages
	FieldBucketUsages = "bucketUsages"
	// FieldStorage is a field name for storage total
	FieldStorage = "storage"
	// FieldEgress is a field name for egress total
	FieldEgress = "egress"
	// FieldObjectCount is a field name for objects count
	FieldObjectCount = "objectCount"
	// FieldPageCount is a field name for total page count
	FieldPageCount = "pageCount"
	// FieldCurrentPage is a field name for current page number
	FieldCurrentPage = "currentPage"
	// FieldTotalCount is a field name for bucket usage count total
	FieldTotalCount = "totalCount"
	// FieldProjectMembers is a field name for project members
	FieldProjectMembers = "projectMembers"
	// CursorArg is an argument name for cursor
	CursorArg = "cursor"
	// PageArg ia an argument name for page number
	PageArg = "page"
	// LimitArg is argument name for limit
	LimitArg = "limit"
	// OffsetArg is argument name for offset
	OffsetArg = "offset"
	// SearchArg is argument name for search
	SearchArg = "search"
	// OrderArg is argument name for order
	OrderArg = "order"
	// OrderDirectionArg is argument name for order direction
	OrderDirectionArg = "orderDirection"
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
				Type: types.projectMemberPage,
				Args: graphql.FieldConfigArgument{
					CursorArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.projectMembersCursor),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*console.Project)

					_, err := console.GetAuth(p.Context)
					if err != nil {
						return nil, HandleError(err)
					}

					cursor := cursorArgsToProjectMembersCursor(p.Args[CursorArg].(map[string]interface{}))
					page, err := service.GetProjectMembers(p.Context, project.ID, cursor)
					if err != nil {
						return nil, HandleError(err)
					}

					var users []projectMember
					for _, member := range page.ProjectMembers {
						user, err := service.GetUser(p.Context, member.MemberID)
						if err != nil {
							return nil, HandleError(err)
						}

						users = append(users, projectMember{
							User:     user,
							JoinedAt: member.CreatedAt,
						})
					}

					projectMembersPage := projectMembersPage{
						ProjectMembers: users,
						TotalCount:     page.TotalCount,
						Offset:         page.Offset,
						Limit:          page.Limit,
						Order:          int(page.Order),
						OrderDirection: int(page.OrderDirection),
						Search:         page.Search,
						CurrentPage:    page.CurrentPage,
						PageCount:      page.PageCount,
					}
					return projectMembersPage, nil
				},
			},
			FieldAPIKeys: &graphql.Field{
				Type: types.apiKeyPage,
				Args: graphql.FieldConfigArgument{
					CursorArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.apiKeysCursor),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*console.Project)

					_, err := console.GetAuth(p.Context)
					if err != nil {
						return nil, err
					}

					cursor := cursorArgsToAPIKeysCursor(p.Args[CursorArg].(map[string]interface{}))
					page, err := service.GetAPIKeys(p.Context, project.ID, cursor)
					if err != nil {
						return nil, err
					}

					apiKeysPage := apiKeysPage{
						APIKeys:        page.APIKeys,
						TotalCount:     page.TotalCount,
						Offset:         page.Offset,
						Limit:          page.Limit,
						Order:          int(page.Order),
						OrderDirection: int(page.OrderDirection),
						Search:         page.Search,
						CurrentPage:    page.CurrentPage,
						PageCount:      page.PageCount,
					}

					return apiKeysPage, err
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

					usage, err := service.GetProjectUsage(p.Context, project.ID, since, before)
					if err != nil {
						return nil, HandleError(err)
					}

					return usage, nil
				},
			},
			FieldBucketUsages: &graphql.Field{
				Type: types.bucketUsagePage,
				Args: graphql.FieldConfigArgument{
					BeforeArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.DateTime),
					},
					CursorArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.bucketUsageCursor),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*console.Project)

					before := p.Args[BeforeArg].(time.Time)
					cursor := fromMapBucketUsageCursor(p.Args[CursorArg].(map[string]interface{}))

					page, err := service.GetBucketTotals(p.Context, project.ID, cursor, before)
					if err != nil {
						return nil, HandleError(err)
					}

					return page, nil
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

// graphqlBucketUsageCursor creates bucket usage cursor graphql input type
func graphqlBucketUsageCursor() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: BucketUsageCursorInputType,
		Fields: graphql.InputObjectConfigFieldMap{
			SearchArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.String),
			},
			LimitArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
			PageArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
		},
	})
}

// graphqlBucketUsage creates bucket usage grapqhl type
func graphqlBucketUsage() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: BucketUsageType,
		Fields: graphql.Fields{
			FieldBucketName: &graphql.Field{
				Type: graphql.String,
			},
			FieldStorage: &graphql.Field{
				Type: graphql.Float,
			},
			FieldEgress: &graphql.Field{
				Type: graphql.Float,
			},
			FieldObjectCount: &graphql.Field{
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

// graphqlBucketUsagePage creates bucket usage page graphql object
func graphqlBucketUsagePage(types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: BucketUsagePageType,
		Fields: graphql.Fields{
			FieldBucketUsages: &graphql.Field{
				Type: graphql.NewList(types.bucketUsage),
			},
			SearchArg: &graphql.Field{
				Type: graphql.String,
			},
			LimitArg: &graphql.Field{
				Type: graphql.Int,
			},
			OffsetArg: &graphql.Field{
				Type: graphql.Int,
			},
			FieldPageCount: &graphql.Field{
				Type: graphql.Int,
			},
			FieldCurrentPage: &graphql.Field{
				Type: graphql.Int,
			},
			FieldTotalCount: &graphql.Field{
				Type: graphql.Int,
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
			FieldObjectCount: &graphql.Field{
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

// fromMapProjectInfo creates console.ProjectInfo from input args
func fromMapProjectInfo(args map[string]interface{}) (project console.ProjectInfo) {
	project.Name, _ = args[FieldName].(string)
	project.Description, _ = args[FieldDescription].(string)

	return
}

// fromMapBucketUsageCursor creates console.BucketUsageCursor from input args
func fromMapBucketUsageCursor(args map[string]interface{}) (cursor console.BucketUsageCursor) {
	limit, _ := args[LimitArg].(int)
	page, _ := args[PageArg].(int)

	cursor.Limit = uint(limit)
	cursor.Page = uint(page)
	cursor.Search, _ = args[SearchArg].(string)
	return
}

func cursorArgsToProjectMembersCursor(args map[string]interface{}) console.ProjectMembersCursor {
	limit, _ := args[LimitArg].(int)
	page, _ := args[PageArg].(int)
	order, _ := args[OrderArg].(int)
	orderDirection, _ := args[OrderDirectionArg].(int)

	var cursor console.ProjectMembersCursor

	cursor.Limit = uint(limit)
	cursor.Page = uint(page)
	cursor.Order = console.ProjectMemberOrder(order)
	cursor.OrderDirection = console.OrderDirection(orderDirection)
	cursor.Search, _ = args[SearchArg].(string)

	return cursor
}

func cursorArgsToAPIKeysCursor(args map[string]interface{}) console.APIKeyCursor {
	limit, _ := args[LimitArg].(int)
	page, _ := args[PageArg].(int)
	order, _ := args[OrderArg].(int)
	orderDirection, _ := args[OrderDirectionArg].(int)

	var cursor console.APIKeyCursor

	cursor.Limit = uint(limit)
	cursor.Page = uint(page)
	cursor.Order = console.APIKeyOrder(order)
	cursor.OrderDirection = console.OrderDirection(orderDirection)
	cursor.Search, _ = args[SearchArg].(string)

	return cursor
}
