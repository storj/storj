// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/satellite/console"
)

// Types return graphql type objects
type Types interface {
	RootQuery() *graphql.Object
	RootMutation() *graphql.Object

	Token() *graphql.Object

	User() *graphql.Object
	Project() *graphql.Object
	ProjectMember() *graphql.Object
	APIKeyInfo() *graphql.Object
	CreateAPIKey() *graphql.Object

	UserInput() *graphql.InputObject
	ProjectInput() *graphql.InputObject
}

// TypeCreator handles graphql type creation and error checking
type TypeCreator struct {
	query    *graphql.Object
	mutation *graphql.Object

	token *graphql.Object

	user          *graphql.Object
	project       *graphql.Object
	projectMember *graphql.Object
	apiKeyInfo    *graphql.Object
	createAPIKey  *graphql.Object

	userInput    *graphql.InputObject
	projectInput *graphql.InputObject
}

// Create create types and check for error
func (c *TypeCreator) Create(service *console.Service) error {
	// inputs
	c.userInput = graphqlUserInput(c)
	if err := c.userInput.Error(); err != nil {
		return err
	}

	c.projectInput = graphqlProjectInput()
	if err := c.projectInput.Error(); err != nil {
		return err
	}

	// entities
	c.user = graphqlUser()
	if err := c.user.Error(); err != nil {
		return err
	}

	c.apiKeyInfo = graphqlAPIKeyInfo()
	if err := c.apiKeyInfo.Error(); err != nil {
		return err
	}

	c.createAPIKey = graphqlCreateAPIKey(c)
	if err := c.createAPIKey.Error(); err != nil {
		return err
	}

	c.projectMember = graphqlProjectMember(service, c)
	if err := c.projectMember.Error(); err != nil {
		return err
	}

	c.project = graphqlProject(service, c)
	if err := c.project.Error(); err != nil {
		return err
	}

	c.token = graphqlToken(service, c)
	if err := c.user.Error(); err != nil {
		return err
	}

	// root objects
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

// RootQuery returns instance of query *graphql.Object
func (c *TypeCreator) RootQuery() *graphql.Object {
	return c.query
}

// RootMutation returns instance of mutation *graphql.Object
func (c *TypeCreator) RootMutation() *graphql.Object {
	return c.mutation
}

// Token returns *graphql.Object which encapsulates User and token string
func (c *TypeCreator) Token() *graphql.Object {
	return c.token
}

// User returns instance of satellite.User *graphql.Object
func (c *TypeCreator) User() *graphql.Object {
	return c.user
}

// APIKeyInfo returns instance of satellite.APIKeyInfo *graphql.Object
func (c *TypeCreator) APIKeyInfo() *graphql.Object {
	return c.apiKeyInfo
}

// CreateAPIKey encapsulates api key and key info
// returns *graphql.Object
func (c *TypeCreator) CreateAPIKey() *graphql.Object {
	return c.createAPIKey
}

// Project returns instance of satellite.Project *graphql.Object
func (c *TypeCreator) Project() *graphql.Object {
	return c.project
}

// ProjectMember returns instance of projectMember *graphql.Object
func (c *TypeCreator) ProjectMember() *graphql.Object {
	return c.projectMember
}

// UserInput returns instance of UserInput *graphql.Object
func (c *TypeCreator) UserInput() *graphql.InputObject {
	return c.userInput
}

// ProjectInput returns instance of ProjectInfo *graphql.Object
func (c *TypeCreator) ProjectInput() *graphql.InputObject {
	return c.projectInput
}
