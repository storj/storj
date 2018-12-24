// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"
	"storj.io/storj/pkg/satellite"
)

const (
	projectType      = "project"
	projectInputType = "projectInput"
	fieldName        = "name"

	fieldDescription = "description"
	// Indicates if user accepted Terms & Conditions during project creation
	// Used in input model
	fieldIsTermsAccepted = "isTermsAccepted"
	fieldMembers         = "members"
	fieldAPIKeys         = "apiKeys"

	limit  = "limit"
	offset = "offset"
)

// graphqlProject creates *graphql.Object type representation of satellite.ProjectInfo
func graphqlProject(service *satellite.Service, types Types) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: projectType,
		Fields: graphql.Fields{
			fieldID: &graphql.Field{
				Type: graphql.String,
			},
			fieldName: &graphql.Field{
				Type: graphql.String,
			},
			fieldDescription: &graphql.Field{
				Type: graphql.String,
			},
			fieldIsTermsAccepted: &graphql.Field{
				Type: graphql.Int,
			},
			fieldCreatedAt: &graphql.Field{
				Type: graphql.DateTime,
			},
			fieldMembers: &graphql.Field{
				Type: graphql.NewList(types.ProjectMember()),
				Args: graphql.FieldConfigArgument{
					offset: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
					limit: &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*satellite.Project)

					offs, _ := p.Args[offset].(int64)
					lim, _ := p.Args[limit].(int)

					members, err := service.GetProjectMembers(p.Context, project.ID, lim, offs)
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
			fieldAPIKeys: &graphql.Field{
				Type: graphql.NewList(types.APIKey()),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					project, _ := p.Source.(*satellite.Project)

					return service.GetAPIKeysByProjectID(p.Context, project.ID)
				},
			},
		},
	})
}

// graphqlProjectInput creates graphql.InputObject type needed to create/update satellite.Project
func graphqlProjectInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: projectInputType,
		Fields: graphql.InputObjectConfigFieldMap{
			fieldName: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldDescription: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldIsTermsAccepted: &graphql.InputObjectFieldConfig{
				Type: graphql.Boolean,
			},
		},
	})
}

// fromMapProjectInfo creates satellite.ProjectInfo from input args
func fromMapProjectInfo(args map[string]interface{}) (project satellite.ProjectInfo) {
	project.Name, _ = args[fieldName].(string)
	project.Description, _ = args[fieldDescription].(string)
	project.IsTermsAccepted, _ = args[fieldIsTermsAccepted].(bool)

	return
}
