// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"time"

	"github.com/graphql-go/graphql"

	"storj.io/storj/satellite/console"
)

const (
	// ProjectMemberType is a graphql type name for project member
	ProjectMemberType = "projectMember"
	// FieldJoinedAt is a field name for joined at timestamp
	FieldJoinedAt = "joinedAt"
)

// graphqlProjectMember creates projectMember type
func graphqlProjectMember(service *console.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: ProjectMemberType,
		Fields: graphql.Fields{
			UserType: &graphql.Field{
				Type: types.User(),
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

// projectMember encapsulates User and joinedAt
type projectMember struct {
	User     *console.User
	JoinedAt time.Time
}
