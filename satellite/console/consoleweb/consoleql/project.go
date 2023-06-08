// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"strconv"
	"time"

	"github.com/graphql-go/graphql"

	"storj.io/common/memory"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

const (
	// ProjectType is a graphql type name for project.
	ProjectType = "project"
	// ProjectInputType is a graphql type name for project input.
	ProjectInputType = "projectInput"
	// ProjectLimitType is a graphql type name for project limit.
	ProjectLimitType = "projectLimit"
	// ProjectUsageType is a graphql type name for project usage.
	ProjectUsageType = "projectUsage"
	// ProjectsCursorInputType is a graphql input type name for projects cursor.
	ProjectsCursorInputType = "projectsCursor"
	// ProjectsPageType is a graphql type name for projects page.
	ProjectsPageType = "projectsPage"
	// BucketUsageCursorInputType is a graphql input
	// type name for bucket usage cursor.
	BucketUsageCursorInputType = "bucketUsageCursor"
	// BucketUsageType is a graphql type name for bucket usage.
	BucketUsageType = "bucketUsage"
	// BucketUsagePageType is a graphql type name for bucket usage page.
	BucketUsagePageType = "bucketUsagePage"
	// ProjectMembersAndInvitationsPageType is a graphql type name for a page of project members and invitations.
	ProjectMembersAndInvitationsPageType = "projectMembersAndInvitationsPage"
	// ProjectMembersCursorInputType is a graphql type name for project members.
	ProjectMembersCursorInputType = "projectMembersCursor"
	// APIKeysPageType is a graphql type name for api keys page.
	APIKeysPageType = "apiKeysPage"
	// APIKeysCursorInputType is a graphql type name for api keys.
	APIKeysCursorInputType = "apiKeysCursor"
	// FieldPublicID is a field name for "publicId".
	FieldPublicID = "publicId"
	// FieldOwnerID is a field name for "ownerId".
	FieldOwnerID = "ownerId"
	// FieldName is a field name for "name".
	FieldName = "name"
	// FieldBucketName is a field name for "bucket name".
	FieldBucketName = "bucketName"
	// FieldDescription is a field name for description.
	FieldDescription = "description"
	// FieldMembersAndInvitations is field name for members and invitations.
	FieldMembersAndInvitations = "membersAndInvitations"
	// FieldAPIKeys is a field name for api keys.
	FieldAPIKeys = "apiKeys"
	// FieldUsage is a field name for usage rollup.
	FieldUsage = "usage"
	// FieldBucketUsages is a field name for bucket usages.
	FieldBucketUsages = "bucketUsages"
	// FieldStorageLimit is a field name for the storage limit.
	FieldStorageLimit = "storageLimit"
	// FieldBandwidthLimit is a field name for bandwidth limit.
	FieldBandwidthLimit = "bandwidthLimit"
	// FieldStorage is a field name for storage total.
	FieldStorage = "storage"
	// FieldEgress is a field name for egress total.
	FieldEgress = "egress"
	// FieldSegmentCount is a field name for segments count.
	FieldSegmentCount = "segmentCount"
	// FieldObjectCount is a field name for objects count.
	FieldObjectCount = "objectCount"
	// FieldPageCount is a field name for total page count.
	FieldPageCount = "pageCount"
	// FieldCurrentPage is a field name for current page number.
	FieldCurrentPage = "currentPage"
	// FieldTotalCount is a field name for bucket usage count total.
	FieldTotalCount = "totalCount"
	// FieldMemberCount is a field name for number of project members.
	FieldMemberCount = "memberCount"
	// FieldProjects is a field name for projects.
	FieldProjects = "projects"
	// FieldProjectMembers is a field name for project members.
	FieldProjectMembers = "projectMembers"
	// FieldProjectInvitations is a field name for project member invitations.
	FieldProjectInvitations = "projectInvitations"
	// CursorArg is an argument name for cursor.
	CursorArg = "cursor"
	// PageArg ia an argument name for page number.
	PageArg = "page"
	// LimitArg is argument name for limit.
	LimitArg = "limit"
	// OffsetArg is argument name for offset.
	OffsetArg = "offset"
	// SearchArg is argument name for search.
	SearchArg = "search"
	// OrderArg is argument name for order.
	OrderArg = "order"
	// OrderDirectionArg is argument name for order direction.
	OrderDirectionArg = "orderDirection"
	// SinceArg marks start of the period.
	SinceArg = "since"
	// BeforeArg marks end of the period.
	BeforeArg = "before"
)

// graphqlProject creates *graphql.Object type representation of satellite.ProjectInfo.
func graphqlProject(service *console.Service, types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: ProjectType,
		Fields: graphql.Fields{
			FieldID: &graphql.Field{
				Type: graphql.String,
			},
			FieldPublicID: &graphql.Field{
				Type: graphql.String,
			},
			FieldName: &graphql.Field{
				Type: graphql.String,
			},
			FieldOwnerID: &graphql.Field{
				Type: graphql.String,
			},
			FieldDescription: &graphql.Field{
				Type: graphql.String,
			},
			FieldCreatedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
			FieldMemberCount: &graphql.Field{
				Type: graphql.Int,
			},
			FieldMembersAndInvitations: &graphql.Field{
				Type: types.projectMembersAndInvitationsPage,
				Args: graphql.FieldConfigArgument{
					CursorArg: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(types.projectMembersCursor),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*console.Project)

					_, err := console.GetUser(p.Context)
					if err != nil {
						return nil, err
					}

					cursor := cursorArgsToProjectMembersCursor(p.Args[CursorArg].(map[string]interface{}))
					page, err := service.GetProjectMembersAndInvitations(p.Context, project.ID, cursor)
					if err != nil {
						return nil, err
					}

					var users []projectMember
					for _, member := range page.ProjectMembers {
						user, err := service.GetUser(p.Context, member.MemberID)
						if err != nil {
							return nil, err
						}

						users = append(users, projectMember{
							User:     user,
							JoinedAt: member.CreatedAt,
						})
					}

					projectMembersPage := projectMembersPage{
						ProjectMembers:     users,
						ProjectInvitations: page.ProjectInvitations,
						TotalCount:         page.TotalCount,
						Offset:             page.Offset,
						Limit:              page.Limit,
						Order:              int(page.Order),
						OrderDirection:     int(page.OrderDirection),
						Search:             page.Search,
						CurrentPage:        page.CurrentPage,
						PageCount:          page.PageCount,
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
						return nil, err
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
						return nil, err
					}

					return page, nil
				},
			},
		},
	})
}

// graphqlProjectInput creates graphql.InputObject type needed to create/update satellite.Project.
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

// graphqlProjectLimit creates graphql.InputObject type needed to create/update satellite.Project.
func graphqlProjectLimit() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: ProjectLimitType,
		Fields: graphql.InputObjectConfigFieldMap{
			FieldStorageLimit: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			FieldBandwidthLimit: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
		},
	})
}

// graphqlBucketUsageCursor creates bucket usage cursor graphql input type.
func graphqlProjectsCursor() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: ProjectsCursorInputType,
		Fields: graphql.InputObjectConfigFieldMap{
			LimitArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
			PageArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
		},
	})
}

// graphqlBucketUsageCursor creates bucket usage cursor graphql input type.
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

// graphqlBucketUsage creates bucket usage grapqhl type.
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
			FieldSegmentCount: &graphql.Field{
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

// graphqlProjectsPage creates a projects page graphql object.
func graphqlProjectsPage(types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: ProjectsPageType,
		Fields: graphql.Fields{
			FieldProjects: &graphql.Field{
				Type: graphql.NewList(types.project),
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

// graphqlBucketUsagePage creates bucket usage page graphql object.
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

// graphqlProjectUsage creates project usage graphql type.
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
			FieldSegmentCount: &graphql.Field{
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

// fromMapProjectInfo creates console.ProjectInfo from input args.
func fromMapProjectInfo(args map[string]interface{}) (project console.ProjectInfo) {
	project.Name, _ = args[FieldName].(string)
	project.Description, _ = args[FieldDescription].(string)

	return
}

// fromMapProjectInfoProjectLimits creates console.ProjectInfo from input args.
func fromMapProjectInfoProjectLimits(projectInfo, projectLimits map[string]interface{}) (project console.ProjectInfo, err error) {
	project.Name, _ = projectInfo[FieldName].(string)
	project.Description, _ = projectInfo[FieldDescription].(string)
	storageLimit, err := strconv.Atoi(projectLimits[FieldStorageLimit].(string))
	if err != nil {
		return project, err
	}
	project.StorageLimit = memory.Size(storageLimit)
	bandwidthLimit, err := strconv.Atoi(projectLimits[FieldBandwidthLimit].(string))
	if err != nil {
		return project, err
	}
	project.BandwidthLimit = memory.Size(bandwidthLimit)

	return
}

// fromMapProjectsCursor creates console.ProjectsCursor from input args.
func fromMapProjectsCursor(args map[string]interface{}) (cursor console.ProjectsCursor) {
	cursor.Limit = args[LimitArg].(int)
	cursor.Page = args[PageArg].(int)
	return
}

// fromMapBucketUsageCursor creates accounting.BucketUsageCursor from input args.
func fromMapBucketUsageCursor(args map[string]interface{}) (cursor accounting.BucketUsageCursor) {
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
