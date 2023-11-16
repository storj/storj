// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/post"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/testredis"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/console/restkeys"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripe"
)

// discardSender discard sending of an actual email.
type discardSender struct{}

// SendEmail immediately returns with nil error.
func (*discardSender) SendEmail(ctx context.Context, msg *post.Message) error {
	return nil
}

// FromAddress returns empty post.Address.
func (*discardSender) FromAddress() post.Address {
	return post.Address{}
}

func TestGraphqlMutation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		log := zaptest.NewLogger(t)

		analyticsService := analytics.NewService(log, analytics.Config{}, "test-satellite")

		redis, err := testredis.Mini(ctx)
		require.NoError(t, err)
		defer ctx.Check(redis.Close)

		cache, err := live.OpenCache(ctx, log.Named("cache"), live.Config{StorageBackend: "redis://" + redis.Addr() + "?db=0"})
		require.NoError(t, err)

		projectLimitCache := accounting.NewProjectLimitCache(db.ProjectAccounting(), 0, 0, 0, accounting.ProjectLimitConfig{CacheCapacity: 100})

		projectUsage := accounting.NewService(db.ProjectAccounting(), cache, projectLimitCache, *sat.Metabase.DB, 5*time.Minute, -10*time.Second)

		// TODO maybe switch this test to testplanet to avoid defining config and Stripe service
		pc := paymentsconfig.Config{
			UsagePrice: paymentsconfig.ProjectUsagePrice{
				StorageTB: "10",
				EgressTB:  "45",
				Segment:   "0.0000022",
			},
		}

		prices, err := pc.UsagePrice.ToModel()
		require.NoError(t, err)

		priceOverrides, err := pc.UsagePriceOverrides.ToModels()
		require.NoError(t, err)

		paymentsService, err := stripe.NewService(
			log.Named("payments.stripe:service"),
			stripe.NewStripeMock(
				db.StripeCoinPayments().Customers(),
				db.Console().Users(),
			),
			pc.StripeCoinPayments,
			db.StripeCoinPayments(),
			db.Wallets(),
			db.Billing(),
			db.Console().Projects(),
			db.Console().Users(),
			db.ProjectAccounting(),
			prices,
			priceOverrides,
			pc.PackagePlans.Packages,
			pc.BonusRate,
			nil,
		)
		require.NoError(t, err)

		service, err := console.NewService(
			log.Named("console"),
			db.Console(),
			restkeys.NewService(db.OIDC().OAuthTokens(), planet.Satellites[0].Config.RESTKeys),
			db.ProjectAccounting(),
			projectUsage,
			sat.API.Buckets.Service,
			paymentsService.Accounts(),
			// TODO: do we need a payment deposit wallet here?
			nil,
			db.Billing(),
			analyticsService,
			consoleauth.NewService(consoleauth.Config{
				TokenExpirationTime: 24 * time.Hour,
			}, &consoleauth.Hmac{Secret: []byte("my-suppa-secret-key")}),
			nil,
			"",
			"",
			sat.Config.Metainfo.ProjectLimits.MaxBuckets,
			console.Config{
				PasswordCost:        console.TestPasswordCost,
				DefaultProjectLimit: 5,
				Session: console.SessionConfig{
					Duration: time.Hour,
				},
			},
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
		rootObject[consoleql.SatelliteRegion] = "EU1"

		schema, err := consoleql.CreateSchema(log, service, mailService)
		require.NoError(t, err)

		createUser := console.CreateUser{
			FullName:        "John Roll",
			ShortName:       "Roll",
			Email:           "test@mail.test",
			UserAgent:       []byte("120bf202-8252-437e-ac12-0e364bee852e"),
			Password:        "123a123",
			SignupPromoCode: "promo1",
		}

		regToken, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		rootUser, err := service.CreateUser(ctx, createUser, regToken.Secret)
		require.NoError(t, err)
		require.Equal(t, createUser.UserAgent, rootUser.UserAgent)

		couponType, err := paymentsService.Accounts().Setup(ctx, rootUser.ID, rootUser.Email, rootUser.SignupPromoCode)

		var signupCouponType payments.CouponType = payments.SignupCoupon

		require.NoError(t, err)
		assert.Equal(t, signupCouponType, couponType)

		activationToken, err := service.GenerateActivationToken(ctx, rootUser.ID, rootUser.Email)
		require.NoError(t, err)

		_, err = service.ActivateAccount(ctx, activationToken)
		require.NoError(t, err)

		tokenInfo, err := service.Token(ctx, console.AuthUser{Email: createUser.Email, Password: createUser.Password})
		require.NoError(t, err)

		userCtx, err := service.TokenAuth(ctx, tokenInfo.Token, time.Now())
		require.NoError(t, err)

		testQuery := func(t *testing.T, query string) (interface{}, error) {
			result := graphql.Do(graphql.Params{
				Schema:        schema,
				Context:       userCtx,
				RequestString: query,
				RootObject:    rootObject,
			})

			for _, err := range result.Errors {
				if err.OriginalError() != nil {
					return nil, err
				}
			}
			require.False(t, result.HasErrors())

			return result.Data, nil
		}

		tokenInfo, err = service.Token(ctx, console.AuthUser{Email: rootUser.Email, Password: createUser.Password})
		require.NoError(t, err)

		userCtx, err = service.TokenAuth(ctx, tokenInfo.Token, time.Now())
		require.NoError(t, err)

		var projectIDField string
		var projectPublicIDField string
		t.Run("Create project mutation", func(t *testing.T) {
			projectInfo := console.UpsertProjectInfo{
				Name:        "Project name",
				Description: "desc",
			}

			query := fmt.Sprintf(
				"mutation {createProject(input:{name:\"%s\",description:\"%s\"}){name,description,id,publicId,createdAt}}",
				projectInfo.Name,
				projectInfo.Description,
			)

			result, err := testQuery(t, query)
			require.NoError(t, err)

			data := result.(map[string]interface{})
			project := data[consoleql.CreateProjectMutation].(map[string]interface{})

			assert.Equal(t, projectInfo.Name, project[consoleql.FieldName])
			assert.Equal(t, projectInfo.Description, project[consoleql.FieldDescription])

			projectIDField = project[consoleql.FieldID].(string)
			projectPublicIDField = project[consoleql.FieldPublicID].(string)

			_, err = uuid.FromString(projectPublicIDField)
			require.NoError(t, err)
		})

		projectID, err := uuid.FromString(projectIDField)
		require.NoError(t, err)

		project, err := service.GetProject(userCtx, projectID)
		require.NoError(t, err)
		require.Equal(t, rootUser.UserAgent, project.UserAgent)

		regTokenUser1, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		user1, err := service.CreateUser(userCtx, console.CreateUser{
			FullName: "User1",
			Email:    "u1@mail.test",
			Password: "123a123",
		}, regTokenUser1.Secret)
		require.NoError(t, err)

		t.Run("Activation", func(t *testing.T) {
			activationToken1, err := service.GenerateActivationToken(
				ctx,
				user1.ID,
				"u1@mail.test",
			)
			require.NoError(t, err)

			_, err = service.ActivateAccount(ctx, activationToken1)
			require.NoError(t, err)

			user1.Email = "u1@mail.test"
		})

		regTokenUser2, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		user2, err := service.CreateUser(userCtx, console.CreateUser{
			FullName: "User1",
			Email:    "u2@mail.test",
			Password: "123a123",
		}, regTokenUser2.Secret)
		require.NoError(t, err)

		t.Run("Activation", func(t *testing.T) {
			activationToken2, err := service.GenerateActivationToken(
				ctx,
				user2.ID,
				"u2@mail.test",
			)
			require.NoError(t, err)

			_, err = service.ActivateAccount(ctx, activationToken2)
			require.NoError(t, err)

			user2.Email = "u2@mail.test"
		})

		regTokenUser3, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		user3, err := service.CreateUser(userCtx, console.CreateUser{
			FullName: "User3",
			Email:    "u3@mail.test",
			Password: "123a123",
		}, regTokenUser3.Secret)
		require.NoError(t, err)

		t.Run("Activation", func(t *testing.T) {
			activationToken3, err := service.GenerateActivationToken(
				ctx,
				user3.ID,
				"u3@mail.test",
			)
			require.NoError(t, err)

			_, err = service.ActivateAccount(ctx, activationToken3)
			require.NoError(t, err)

			user3.Email = "u3@mail.test"
		})

		testAdd := func(query string, expectedMembers int) {
			result, err := testQuery(t, query)
			require.NoError(t, err)

			data := result.(map[string]interface{})
			proj := data[consoleql.AddProjectMembersMutation].(map[string]interface{})

			members := proj[consoleql.FieldMembersAndInvitations].(map[string]interface{})
			projectMembers := members[consoleql.FieldProjectMembers].([]interface{})

			assert.Equal(t, project.ID.String(), proj[consoleql.FieldID])
			assert.Equal(t, project.PublicID.String(), proj[consoleql.FieldPublicID])
			assert.Equal(t, project.Name, proj[consoleql.FieldName])
			assert.Equal(t, expectedMembers, len(projectMembers))
		}

		t.Run("Add project members mutation", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {addProjectMembers(projectID:\"%s\",email:[\"%s\",\"%s\"]){id,publicId,name,membersAndInvitations(cursor: { limit: 50, search: \"\", page: 1, order: 1, orderDirection: 2 }){projectMembers{joinedAt}}}}",
				project.ID.String(),
				user1.Email,
				user2.Email,
			)

			testAdd(query, 3)
		})

		t.Run("Add project members mutation with publicId", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {addProjectMembers(publicId:\"%s\",email:[\"%s\"]){id,publicId,name,membersAndInvitations(cursor: { limit: 50, search: \"\", page: 1, order: 1, orderDirection: 2 }){projectMembers{joinedAt}}}}",
				project.PublicID.String(),
				user3.Email,
			)

			testAdd(query, 4)
		})

		t.Run("Fail add project members mutation without ID", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {addProjectMembers(email:[\"%s\",\"%s\"]){id,publicId,name,membersAndInvitations(cursor: { limit: 50, search: \"\", page: 1, order: 1, orderDirection: 2 }){projectMembers{joinedAt}}}}",
				user1.Email,
				user2.Email,
			)

			_, err = testQuery(t, query)
			require.Error(t, err)
		})

		t.Run("Delete project members mutation", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {deleteProjectMembers(projectID:\"%s\",email:[\"%s\",\"%s\",\"%s\"]){id,publicId,name,membersAndInvitations(cursor: { limit: 50, search: \"\", page: 1, order: 1, orderDirection: 2 }){projectMembers{user{id}}}}}",
				project.ID.String(),
				user1.Email,
				user2.Email,
				user3.Email,
			)

			result, err := testQuery(t, query)
			require.NoError(t, err)

			data := result.(map[string]interface{})
			proj := data[consoleql.DeleteProjectMembersMutation].(map[string]interface{})

			members := proj[consoleql.FieldMembersAndInvitations].(map[string]interface{})
			projectMembers := members[consoleql.FieldProjectMembers].([]interface{})
			rootMember := projectMembers[0].(map[string]interface{})[consoleql.UserType].(map[string]interface{})

			assert.Equal(t, project.ID.String(), proj[consoleql.FieldID])
			assert.Equal(t, project.PublicID.String(), proj[consoleql.FieldPublicID])
			assert.Equal(t, project.Name, proj[consoleql.FieldName])
			assert.Equal(t, 1, len(members))

			assert.Equal(t, rootUser.ID.String(), rootMember[consoleql.FieldID])
		})

		var keyID string
		t.Run("Create api key mutation", func(t *testing.T) {
			keyName := "key1"
			query := fmt.Sprintf(
				"mutation {createAPIKey(projectID:\"%s\",name:\"%s\"){key,keyInfo{id,name,projectID}}}",
				project.ID.String(),
				keyName,
			)

			result, err := testQuery(t, query)
			require.NoError(t, err)

			data := result.(map[string]interface{})
			createAPIKey := data[consoleql.CreateAPIKeyMutation].(map[string]interface{})

			key := createAPIKey[consoleql.FieldKey].(string)
			keyInfo := createAPIKey[consoleql.APIKeyInfoType].(map[string]interface{})

			assert.NotEqual(t, "", key)

			assert.Equal(t, keyName, keyInfo[consoleql.FieldName])
			assert.Equal(t, project.ID.String(), keyInfo[consoleql.FieldProjectID])

			keyID = keyInfo[consoleql.FieldID].(string)
		})

		t.Run("Delete api key mutation", func(t *testing.T) {
			id, err := uuid.FromString(keyID)
			require.NoError(t, err)

			info, err := service.GetAPIKeyInfo(userCtx, id)
			require.NoError(t, err)

			query := fmt.Sprintf(
				"mutation {deleteAPIKeys(id:[\"%s\"]){name,projectID}}",
				keyID,
			)

			result, err := testQuery(t, query)
			require.NoError(t, err)

			data := result.(map[string]interface{})
			keyInfoList := data[consoleql.DeleteAPIKeysMutation].([]interface{})

			for _, k := range keyInfoList {
				keyInfo := k.(map[string]interface{})

				assert.Equal(t, info.Name, keyInfo[consoleql.FieldName])
				assert.Equal(t, project.ID.String(), keyInfo[consoleql.FieldProjectID])
			}
		})

		const testName = "testName"
		const testDescription = "test description"
		const StorageLimit = "100"
		const BandwidthLimit = "100"

		testUpdate := func(query string) {
			result, err := testQuery(t, query)
			require.NoError(t, err)

			data := result.(map[string]interface{})
			proj := data[consoleql.UpdateProjectMutation].(map[string]interface{})

			assert.Equal(t, project.ID.String(), proj[consoleql.FieldID])
			assert.Equal(t, project.PublicID.String(), proj[consoleql.FieldPublicID])
			assert.Equal(t, testName, proj[consoleql.FieldName])
			assert.Equal(t, testDescription, proj[consoleql.FieldDescription])
		}

		t.Run("Update project mutation", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {updateProject(id:\"%s\",projectFields:{name:\"%s\",description:\"%s\"},projectLimits:{storageLimit:\"%s\",bandwidthLimit:\"%s\"}){id,publicId,name,description}}",
				project.ID.String(),
				testName,
				testDescription,
				StorageLimit,
				BandwidthLimit,
			)

			testUpdate(query)
		})

		t.Run("Update project mutation with publicId", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {updateProject(publicId:\"%s\",projectFields:{name:\"%s\",description:\"%s\"},projectLimits:{storageLimit:\"%s\",bandwidthLimit:\"%s\"}){id,publicId,name,description}}",
				project.PublicID.String(),
				testName,
				testDescription,
				StorageLimit,
				BandwidthLimit,
			)

			testUpdate(query)
		})

		t.Run("Fail update project mutation without ID", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {updateProject(projectFields:{name:\"%s\",description:\"%s\"},projectLimits:{storageLimit:\"%s\",bandwidthLimit:\"%s\"}){id,publicId,name,description}}",
				testName,
				testDescription,
				StorageLimit,
				BandwidthLimit,
			)

			_, err := testQuery(t, query)
			require.Error(t, err)
		})

		t.Run("Delete project mutation", func(t *testing.T) {
			query := fmt.Sprintf(
				"mutation {deleteProject(id:\"%s\"){id,name}}",
				projectID,
			)

			result, err := testQuery(t, query)
			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, console.ErrUnauthorized.New("not implemented").Error(), err.Error())
		})
	})
}
