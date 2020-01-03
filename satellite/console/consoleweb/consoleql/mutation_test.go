// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/rewards"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// discardSender discard sending of an actual email
type discardSender struct{}

// SendEmail immediately returns with nil error
func (*discardSender) SendEmail(ctx context.Context, msg *post.Message) error {
	return nil
}

// FromAddress returns empty post.Address
func (*discardSender) FromAddress() post.Address {
	return post.Address{}
}

func TestGrapqhlMutation(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		log := zaptest.NewLogger(t)

		partnersService := rewards.NewPartnersService(
			log.Named("partners"),
			rewards.DefaultPartnersDB,
			[]string{
				"https://us-central-1.tardigrade.io/",
				"https://asia-east-1.tardigrade.io/",
				"https://europe-west-1.tardigrade.io/",
			},
		)

		payments := stripecoinpayments.NewService(
			log.Named("payments"),
			stripecoinpayments.Config{},
			db.StripeCoinPayments(),
			db.Console().Projects(),
			db.ProjectAccounting(),
			0, 0, 0,
		)

		cache, err := live.NewCache(log.Named("cache"), live.Config{StorageBackend: "memory"})
		require.NoError(t, err)

		projectUsage := accounting.NewService(db.ProjectAccounting(), cache, 0)

		service, err := console.NewService(
			log.Named("console"),
			&consoleauth.Hmac{Secret: []byte("my-suppa-secret-key")},
			db.Console(),
			db.ProjectAccounting(),
			projectUsage,
			db.Rewards(),
			partnersService,
			payments.Accounts(),
			console.TestPasswordCost,
		)
		require.NoError(t, err)

		mailService, err := mailservice.New(log, &discardSender{}, "testdata")
		require.NoError(t, err)
		defer ctx.Check(mailService.Close)

		rootObject := make(map[string]interface{})
		rootObject["origin"] = "http://doesntmatter.com/"
		rootObject[consoleql.ActivationPath] = "?activationToken="
		rootObject[consoleql.SignInPath] = "login"
		rootObject[consoleql.LetUsKnowURL] = "letUsKnowURL"
		rootObject[consoleql.ContactInfoURL] = "contactInfoURL"
		rootObject[consoleql.TermsAndConditionsURL] = "termsAndConditionsURL"

		schema, err := consoleql.CreateSchema(log, service, mailService)
		require.NoError(t, err)

		createUser := console.CreateUser{
			FullName:  "John Roll",
			ShortName: "Roll",
			Email:     "test@mail.test",
			PartnerID: "120bf202-8252-437e-ac12-0e364bee852e",
			Password:  "123a123",
		}
		refUserID := ""

		regToken, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		rootUser, err := service.CreateUser(ctx, createUser, regToken.Secret, refUserID)
		require.NoError(t, err)
		require.Equal(t, createUser.PartnerID, rootUser.PartnerID.String())

		activationToken, err := service.GenerateActivationToken(ctx, rootUser.ID, rootUser.Email)
		require.NoError(t, err)

		err = service.ActivateAccount(ctx, activationToken)
		require.NoError(t, err)

		token, err := service.Token(ctx, createUser.Email, createUser.Password)
		require.NoError(t, err)

		sauth, err := service.Authorize(auth.WithAPIKey(ctx, []byte(token)))
		require.NoError(t, err)

		authCtx := console.WithAuth(ctx, sauth)

		testQuery := func(t *testing.T, query string) interface{} {
			result := graphql.Do(graphql.Params{
				Schema:        schema,
				Context:       authCtx,
				RequestString: query,
				RootObject:    rootObject,
			})

			for _, err := range result.Errors {
				assert.NoError(t, err)
			}
			require.False(t, result.HasErrors())

			return result.Data
		}

		token, err = service.Token(ctx, rootUser.Email, createUser.Password)
		require.NoError(t, err)

		sauth, err = service.Authorize(auth.WithAPIKey(ctx, []byte(token)))
		require.NoError(t, err)

		authCtx = console.WithAuth(ctx, sauth)

		var projectID string
		t.Run("Create project mutation", func(t *testing.T) {
			projectInfo := console.ProjectInfo{
				Name:        "Project name",
				Description: "desc",
			}

			query := fmt.Sprintf(
				"mutation {createProject(input:{name:\"%s\",description:\"%s\"}){name,description,id,createdAt}}",
				projectInfo.Name,
				projectInfo.Description,
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			project := data[consoleql.CreateProjectMutation].(map[string]interface{})

			assert.Equal(t, projectInfo.Name, project[consoleql.FieldName])
			assert.Equal(t, projectInfo.Description, project[consoleql.FieldDescription])

			projectID = project[consoleql.FieldID].(string)
		})

		pID, err := uuid.Parse(projectID)
		require.NoError(t, err)

		project, err := service.GetProject(authCtx, *pID)
		require.NoError(t, err)
		require.Equal(t, rootUser.PartnerID, project.PartnerID)

		t.Run("Update project description mutation", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {updateProjectDescription(id:\"%s\",description:\"%s\"){id,name,description}}",
				project.ID.String(),
				"",
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			proj := data[consoleql.UpdateProjectDescriptionMutation].(map[string]interface{})

			assert.Equal(t, project.ID.String(), proj[consoleql.FieldID])
			assert.Equal(t, project.Name, proj[consoleql.FieldName])
			assert.Equal(t, "", proj[consoleql.FieldDescription])
		})

		regTokenUser1, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		user1, err := service.CreateUser(authCtx, console.CreateUser{
			FullName: "User1",
			Email:    "u1@mail.test",
			Password: "123a123",
		}, regTokenUser1.Secret, refUserID)
		require.NoError(t, err)

		t.Run("Activation", func(t *testing.T) {
			activationToken1, err := service.GenerateActivationToken(
				ctx,
				user1.ID,
				"u1@mail.test",
			)
			require.NoError(t, err)

			err = service.ActivateAccount(ctx, activationToken1)
			require.NoError(t, err)

			user1.Email = "u1@mail.test"
		})

		regTokenUser2, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		user2, err := service.CreateUser(authCtx, console.CreateUser{
			FullName: "User1",
			Email:    "u2@mail.test",
			Password: "123a123",
		}, regTokenUser2.Secret, refUserID)
		require.NoError(t, err)

		t.Run("Activation", func(t *testing.T) {
			activationToken2, err := service.GenerateActivationToken(
				ctx,
				user2.ID,
				"u2@mail.test",
			)
			require.NoError(t, err)

			err = service.ActivateAccount(ctx, activationToken2)
			require.NoError(t, err)

			user2.Email = "u2@mail.test"
		})

		t.Run("Add project members mutation", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {addProjectMembers(projectID:\"%s\",email:[\"%s\",\"%s\"]){id,name,members(cursor: { limit: 50, search: \"\", page: 1, order: 1, orderDirection: 2 }){projectMembers{joinedAt}}}}",
				project.ID.String(),
				user1.Email,
				user2.Email,
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			proj := data[consoleql.AddProjectMembersMutation].(map[string]interface{})

			members := proj[consoleql.FieldMembers].(map[string]interface{})
			projectMembers := members[consoleql.FieldProjectMembers].([]interface{})

			assert.Equal(t, project.ID.String(), proj[consoleql.FieldID])
			assert.Equal(t, project.Name, proj[consoleql.FieldName])
			assert.Equal(t, 3, len(projectMembers))
		})

		t.Run("Delete project members mutation", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {deleteProjectMembers(projectID:\"%s\",email:[\"%s\",\"%s\"]){id,name,members(cursor: { limit: 50, search: \"\", page: 1, order: 1, orderDirection: 2 }){projectMembers{user{id}}}}}",
				project.ID.String(),
				user1.Email,
				user2.Email,
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			proj := data[consoleql.DeleteProjectMembersMutation].(map[string]interface{})

			members := proj[consoleql.FieldMembers].(map[string]interface{})
			projectMembers := members[consoleql.FieldProjectMembers].([]interface{})
			rootMember := projectMembers[0].(map[string]interface{})[consoleql.UserType].(map[string]interface{})

			assert.Equal(t, project.ID.String(), proj[consoleql.FieldID])
			assert.Equal(t, project.Name, proj[consoleql.FieldName])
			assert.Equal(t, 1, len(members))

			assert.Equal(t, rootUser.ID.String(), rootMember[consoleql.FieldID])
		})

		var keyID string
		t.Run("Create api key mutation", func(t *testing.T) {
			keyName := "key1"
			query := fmt.Sprintf(
				"mutation {createAPIKey(projectID:\"%s\",name:\"%s\"){key,keyInfo{id,name,projectID,partnerId}}}",
				project.ID.String(),
				keyName,
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			createAPIKey := data[consoleql.CreateAPIKeyMutation].(map[string]interface{})

			key := createAPIKey[consoleql.FieldKey].(string)
			keyInfo := createAPIKey[consoleql.APIKeyInfoType].(map[string]interface{})

			assert.NotEqual(t, "", key)

			assert.Equal(t, keyName, keyInfo[consoleql.FieldName])
			assert.Equal(t, project.ID.String(), keyInfo[consoleql.FieldProjectID])
			assert.Equal(t, rootUser.PartnerID.String(), keyInfo[consoleql.FieldPartnerID])

			keyID = keyInfo[consoleql.FieldID].(string)
		})

		t.Run("Delete api key mutation", func(t *testing.T) {
			id, err := uuid.Parse(keyID)
			require.NoError(t, err)

			info, err := service.GetAPIKeyInfo(authCtx, *id)
			require.NoError(t, err)

			query := fmt.Sprintf(
				"mutation {deleteAPIKeys(id:[\"%s\"]){name,projectID}}",
				keyID,
			)

			result := testQuery(t, query)
			data := result.(map[string]interface{})
			keyInfoList := data[consoleql.DeleteAPIKeysMutation].([]interface{})

			for _, k := range keyInfoList {
				keyInfo := k.(map[string]interface{})

				assert.Equal(t, info.Name, keyInfo[consoleql.FieldName])
				assert.Equal(t, project.ID.String(), keyInfo[consoleql.FieldProjectID])
			}
		})

		t.Run("Delete project mutation", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {deleteProject(id:\"%s\"){id,name}}",
				projectID,
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			proj := data[consoleql.DeleteProjectMutation].(map[string]interface{})

			assert.Equal(t, project.Name, proj[consoleql.FieldName])
			assert.Equal(t, project.ID.String(), proj[consoleql.FieldID])

			_, err := service.GetProject(authCtx, project.ID)
			assert.Error(t, err)
		})
	})
}
