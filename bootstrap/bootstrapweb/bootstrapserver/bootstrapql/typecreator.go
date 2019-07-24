// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bootstrapql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/bootstrap/bootstrapweb"
)

// Types return graphql type objects
type Types interface {
	RootQuery() *graphql.Object
}

// TypeCreator handles graphql type creation and error checking
type TypeCreator struct {
	query *graphql.Object
}

// Create create types and check for error
func (c *TypeCreator) Create(service *bootstrapweb.Service) error {
	// root objects
	c.query = rootQuery(service)

	err := c.query.Error()
	if err != nil {
		return err
	}

	return nil
}

// RootQuery returns instance of query *graphql.Object
func (c *TypeCreator) RootQuery() *graphql.Object {
	return c.query
}
