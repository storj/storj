// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
	"go.uber.org/zap"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
)

// TypeCreator handles graphql type creation and error checking.
type TypeCreator struct {
	query    *graphql.Object
	mutation *graphql.Object

	user                             *graphql.Object
	reward                           *graphql.Object
	project                          *graphql.Object
	projectUsage                     *graphql.Object
	projectsPage                     *graphql.Object
	bucketUsage                      *graphql.Object
	bucketUsagePage                  *graphql.Object
	projectMember                    *graphql.Object
	projectInvitation                *graphql.Object
	projectMemberPage                *graphql.Object
	projectMembersAndInvitationsPage *graphql.Object
	apiKeyPage                       *graphql.Object
	apiKeyInfo                       *graphql.Object
	createAPIKey                     *graphql.Object

	userInput            *graphql.InputObject
	projectInput         *graphql.InputObject
	projectLimit         *graphql.InputObject
	projectsCursor       *graphql.InputObject
	bucketUsageCursor    *graphql.InputObject
	projectMembersCursor *graphql.InputObject
	apiKeysCursor        *graphql.InputObject
}

// Create create types and check for error.
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

	c.projectLimit = graphqlProjectLimit()
	if err := c.projectLimit.Error(); err != nil {
		return err
	}

	c.bucketUsageCursor = graphqlBucketUsageCursor()
	if err := c.bucketUsageCursor.Error(); err != nil {
		return err
	}

	c.projectMembersCursor = graphqlProjectMembersCursor()
	if err := c.projectMembersCursor.Error(); err != nil {
		return err
	}

	c.apiKeysCursor = graphqlAPIKeysCursor()
	if err := c.apiKeysCursor.Error(); err != nil {
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

	c.projectInvitation = graphqlProjectInvitation()
	if err := c.projectInvitation.Error(); err != nil {
		return err
	}

	c.projectMemberPage = graphqlProjectMembersPage(c)
	if err := c.projectMemberPage.Error(); err != nil {
		return err
	}

	c.projectMembersAndInvitationsPage = graphqlProjectMembersAndInvitationsPage(c)
	if err := c.projectMembersAndInvitationsPage.Error(); err != nil {
		return err
	}

	c.apiKeyPage = graphqlAPIKeysPage(c)
	if err := c.apiKeyPage.Error(); err != nil {
		return err
	}

	c.project = graphqlProject(service, c)
	if err := c.project.Error(); err != nil {
		return err
	}

	c.projectsCursor = graphqlProjectsCursor()
	if err := c.projectsCursor.Error(); err != nil {
		return err
	}

	c.projectsPage = graphqlProjectsPage(c)
	if err := c.projectsPage.Error(); err != nil {
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

// RootQuery returns instance of query *graphql.Object.
func (c *TypeCreator) RootQuery() *graphql.Object {
	return c.query
}

// RootMutation returns instance of mutation *graphql.Object.
func (c *TypeCreator) RootMutation() *graphql.Object {
	return c.mutation
}
