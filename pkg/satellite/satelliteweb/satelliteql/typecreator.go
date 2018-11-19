// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"
	"storj.io/storj/pkg/satellite"
)

// Types return graphql type objects
type Types interface {
	RootQuery() *graphql.Object
	RootMutation() *graphql.Object

	UserType() *graphql.Object
}

// TypeCreator handles graphql type creation and error checking
type TypeCreator struct {
	query    *graphql.Object
	mutation *graphql.Object

	user *graphql.Object
}

// RootQuery returns instance of query *graphql.Object
func (c *TypeCreator) RootQuery() *graphql.Object {
	return c.query
}

// RootMutation returns instance of mutation *graphql.Object
func (c *TypeCreator) RootMutation() *graphql.Object {
	return c.mutation
}

// Create create types and check for error
func (c *TypeCreator) Create(service *satellite.Service) error {
	c.user = graphqlUser()
	if err := c.user.Error(); err != nil {
		return err
	}

	c.query = rootQuery(service, c)
	if err := c.query.Error(); err != nil {
		return err
	}

	c.mutation = rootMutation(service, c)
	if err := c.mutation.Error(); err != nil {
		return err
	}

	return nil
}

// UserType returns instance of user *graphql.Object
func (c *TypeCreator) UserType() *graphql.Object {
	return c.user
}
