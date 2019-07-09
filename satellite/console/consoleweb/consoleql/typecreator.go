// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
	"go.uber.org/zap"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
)

// TypeCreator handles graphql type creation and error checking
type TypeCreator struct {
	query    *graphql.Object
	mutation *graphql.Object

	token *graphql.Object

	user            *graphql.Object
	reward          *graphql.Object
	creditUsage     *graphql.Object
	project         *graphql.Object
	projectUsage    *graphql.Object
	bucketUsage     *graphql.Object
	bucketUsagePage *graphql.Object
	projectMember   *graphql.Object
	apiKeyInfo      *graphql.Object
	createAPIKey    *graphql.Object

	userInput         *graphql.InputObject
	projectInput      *graphql.InputObject
	bucketUsageCursor *graphql.InputObject
}

// Create create types and check for error
func (c *TypeCreator) Create(log *zap.Logger, service *console.Service, mailService *mailservice.Service) error {
	// inputs
	c.userInput = graphqlUserInput()
	if err := c.userInput.Error(); err != nil {
		return err
	}

	c.projectInput = graphqlProjectInput()
	if err := c.projectInput.Error(); err != nil {
		return err
	}

	c.bucketUsageCursor = graphqlBucketUsageCursor()
	if err := c.bucketUsageCursor.Error(); err != nil {
		return err
	}

	// entities
	c.user = graphqlUser()
	if err := c.user.Error(); err != nil {
		return err
	}

	c.reward = graphqlReward()
	if err := c.reward.Error(); err != nil {
		return err
	}

	c.creditUsage = graphqlCreditUsage()
	if err := c.creditUsage.Error(); err != nil {
		return err
	}

	c.projectUsage = graphqlProjectUsage()
	if err := c.projectUsage.Error(); err != nil {
		return err
	}

	c.bucketUsage = graphqlBucketUsage()
	if err := c.bucketUsage.Error(); err != nil {
		return err
	}

	c.bucketUsagePage = graphqlBucketUsagePage(c)
	if err := c.bucketUsagePage.Error(); err != nil {
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
	c.query = rootQuery(service, mailService, c)
	if err := c.query.Error(); err != nil {
		return err
	}

	c.mutation = rootMutation(log, service, mailService, c)
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
