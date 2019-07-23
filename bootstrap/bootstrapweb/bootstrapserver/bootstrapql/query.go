// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bootstrapql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/bootstrap/bootstrapweb"
	"storj.io/storj/pkg/storj"
)

const (
	// Query is immutable graphql request
	Query = "query"
	// IsNodeUpQuery is a query name for checking if node is up
	IsNodeUpQuery = "isNodeUp"

	// NodeID is a field name for nodeID
	NodeID = "nodeID"
)

// rootQuery creates query for graphql
func rootQuery(service *bootstrapweb.Service) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: Query,
		Fields: graphql.Fields{
			IsNodeUpQuery: &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					NodeID: &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					inputNodeID, _ := p.Args[NodeID].(string)

					nodeID, err := storj.NodeIDFromString(inputNodeID)
					if err != nil {
						return false, err
					}

					return service.IsNodeAvailable(p.Context, nodeID)
				},
			},
		},
	})
}
