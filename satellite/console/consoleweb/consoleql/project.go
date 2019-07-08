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
	// PaymentMethodType is a field name for payment method
	PaymentMethodType = "paymentMethod"
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
	// FieldPaymentMethods is a field name for payments methods
	FieldPaymentMethods = "paymentMethods"
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
	// FieldCardBrand is a field name for credit card brand
	FieldCardBrand = "brand"
	// FieldCardLastFour is a field name for credit card last four digits
	FieldCardLastFour = "lastFour"
	// FieldCardToken is a field name for credit card token
	FieldCardToken = "cardToken"
	// FieldIsDefault is a field name for default payment method
	FieldIsDefault = "isDefault"
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

					_, err = console.GetAuth(p.Context)
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

					return service.GetBucketTotals(p.Context, project.ID, cursor, before)
				},
			},
			FieldPaymentMethods: &graphql.Field{
				Type: graphql.NewList(types.paymentMethod),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*console.Project)

					paymentMethods, err := service.GetProjectPaymentMethods(p.Context, project.ID)
					if err != nil {
						return nil, err
					}

					var projectPaymentMethods []projectPayment
					for _, paymentMethod := range paymentMethods {
						projectPaymentMethod := projectPayment{
							ID:         paymentMethod.ID.String(),
							LastFour:   paymentMethod.Card.LastFour,
							AddedAt:    paymentMethod.CreatedAt,
							CardBrand:  paymentMethod.Card.Brand,
							ExpMonth:   paymentMethod.Card.ExpirationMonth,
							ExpYear:    paymentMethod.Card.ExpirationYear,
							HolderName: paymentMethod.Card.Name,
							IsDefault:  paymentMethod.IsDefault,
						}

						projectPaymentMethods = append(projectPaymentMethods, projectPaymentMethod)
					}

					return projectPaymentMethods, nil
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

const (
	// FieldExpirationYear is field name for expiration year
	FieldExpirationYear = "expYear"
	// FieldExpirationMonth is field name for expiration month
	FieldExpirationMonth = "expMonth"
	// FieldHolderName is field name for holder name
	FieldHolderName = "holderName"
	// FieldAddedAt is field name for added at date
	FieldAddedAt = "addedAt"
)

// graphqlPaymentMethod creates invoice payment method graphql type
func graphqlPaymentMethod() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: PaymentMethodType,
		Fields: graphql.Fields{
			FieldID: &graphql.Field{
				Type: graphql.String,
			},
			FieldExpirationYear: &graphql.Field{
				Type: graphql.Int,
			},
			FieldExpirationMonth: &graphql.Field{
				Type: graphql.Int,
			},
			FieldCardBrand: &graphql.Field{
				Type: graphql.String,
			},
			FieldCardLastFour: &graphql.Field{
				Type: graphql.String,
			},
			FieldHolderName: &graphql.Field{
				Type: graphql.String,
			},
			FieldAddedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
			FieldIsDefault: &graphql.Field{
				Type: graphql.Boolean,
			},
		},
	})
}

type projectPayment struct {
	ID         string
	ExpYear    int64
	ExpMonth   int64
	CardBrand  string
	LastFour   string
	HolderName string
	AddedAt    time.Time
	IsDefault  bool
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
