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

	Token() *graphql.Object

	User() *graphql.Object
	Company() *graphql.Object
	Project() *graphql.Object

	UserInput() *graphql.InputObject
	CompanyInput() *graphql.InputObject
	ProjectInput() *graphql.InputObject
}

// TypeCreator handles graphql type creation and error checking
type TypeCreator struct {
	query    *graphql.Object
	mutation *graphql.Object

	token *graphql.Object

	user    *graphql.Object
	company *graphql.Object
	project *graphql.Object

	userInput    *graphql.InputObject
	companyInput *graphql.InputObject
	projectInput *graphql.InputObject
}

// Create create types and check for error
func (c *TypeCreator) Create(service *satellite.Service) error {
	c.company = graphqlCompany()
	if err := c.company.Error(); err != nil {
		return err
	}

	c.companyInput = graphqlCompanyInput()
	if err := c.companyInput.Error(); err != nil {
		return err
	}

	c.projectInput = graphqlProjectInput()
	if err := c.projectInput.Error(); err != nil {
		return err
	}

	c.project = graphqlProject(service)
	if err := c.project.Error(); err != nil {
		return err
	}

	c.user = graphqlUser(service, c)
	if err := c.user.Error(); err != nil {
		return err
	}

	c.userInput = graphqlUserInput(c)
	if err := c.userInput.Error(); err != nil {
		return err
	}

	c.token = graphqlToken(service, c)
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

// Company returns instance of satellite.Company *graphql.Object
func (c *TypeCreator) Company() *graphql.Object {
	return c.company
}

// Project returns instance of satellite.Project *graphql.Object
func (c *TypeCreator) Project() *graphql.Object {
	return c.project
}

// UserInput returns instance of UserInput *graphql.Object
func (c *TypeCreator) UserInput() *graphql.InputObject {
	return c.userInput
}

// CompanyInput returns instance of CompanyInfo *graphql.Object
func (c *TypeCreator) CompanyInput() *graphql.InputObject {
	return c.companyInput
}

// ProjectInput returns instance of ProjectInfo *graphql.Object
func (c *TypeCreator) ProjectInput() *graphql.InputObject {
	return c.projectInput
}
