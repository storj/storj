// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"time"

	"github.com/graphql-go/graphql"

	"storj.io/storj/pkg/satellite"
)

const (
	projectMemberType = "projectMember"

	fieldJoinedAt = "joinedAt"
)

// graphqlProjectMember creates projectMember type
func graphqlProjectMember(service *satellite.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: projectMemberType,
		Fields: graphql.Fields{
			userType: &graphql.Field{
				Type: types.User(),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					member, _ := p.Source.(projectMember)
					// company sub query expects pointer
					return member.User, nil
				},
			},
			fieldJoinedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
		},
	})
}

// projectMember encapsulates User and joinedAt
type projectMember struct {
	User     *satellite.User
	JoinedAt time.Time
}
