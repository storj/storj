// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"time"

	"github.com/graphql-go/graphql"

	"storj.io/storj/satellite/console"
)

const (
	// ProjectMemberType is a graphql type name for project member.
	ProjectMemberType = "projectMember"
	// ProjectInvitationType is a graphql type name for project member invitation.
	ProjectInvitationType = "projectInvitation"
	// FieldJoinedAt is a field name for joined at timestamp.
	FieldJoinedAt = "joinedAt"
	// FieldExpired is a field name for expiration status.
	FieldExpired = "expired"
)

// graphqlProjectMember creates projectMember type.
func graphqlProjectMember(service *console.Service, types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: ProjectMemberType,
		Fields: graphql.Fields{
			UserType: &graphql.Field{
				Type: types.user,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					member, _ := p.Source.(projectMember)
					// company sub query expects pointer
					return member.User, nil
				},
			},
			FieldJoinedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
		},
	})
}

// graphqlProjectInvitation creates projectInvitation type.
func graphqlProjectInvitation() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: ProjectInvitationType,
		Fields: graphql.Fields{
			FieldEmail: &graphql.Field{
				Type: graphql.String,
			},
			FieldCreatedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
			FieldExpired: &graphql.Field{
				Type: graphql.Boolean,
			},
		},
	})
}

func graphqlProjectMembersCursor() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: ProjectMembersCursorInputType,
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
			OrderArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
			OrderDirectionArg: &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Int),
			},
		},
	})
}

func graphqlProjectMembersAndInvitationsPage(types *TypeCreator) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: ProjectMembersAndInvitationsPageType,
		Fields: graphql.Fields{
			FieldProjectMembers: &graphql.Field{
				Type: graphql.NewList(types.projectMember),
			},
			FieldProjectInvitations: &graphql.Field{
				Type: graphql.NewList(types.projectInvitation),
			},
			SearchArg: &graphql.Field{
				Type: graphql.String,
			},
			LimitArg: &graphql.Field{
				Type: graphql.Int,
			},
			OrderArg: &graphql.Field{
				Type: graphql.Int,
			},
			OrderDirectionArg: &graphql.Field{
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

// projectMember encapsulates User and joinedAt.
type projectMember struct {
	User     *console.User
	JoinedAt time.Time
}

// projectInvitation encapsulates a console.ProjectInvitation and its expiration status.
type projectInvitation struct {
	Email     string
	CreatedAt time.Time
	Expired   bool
}

type projectMembersPage struct {
	ProjectMembers     []projectMember
	ProjectInvitations []projectInvitation

	Search         string
	Limit          uint
	Order          int
	OrderDirection int
	Offset         uint64

	PageCount   uint
	CurrentPage uint
	TotalCount  uint64
}
