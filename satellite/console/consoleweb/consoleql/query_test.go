// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
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

func TestGraphqlQuery(t *testing.T) {
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
		rootObject[consoleql.LetUsKnowURL] = "letUsKnowURL"
		rootObject[consoleql.ContactInfoURL] = "contactInfoURL"
		rootObject[consoleql.TermsAndConditionsURL] = "termsAndConditionsURL"

		creator := consoleql.TypeCreator{}
		err = creator.Create(log, service, mailService)
		require.NoError(t, err)

		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    creator.RootQuery(),
			Mutation: creator.RootMutation(),
		})
		require.NoError(t, err)

		createUser := console.CreateUser{
			FullName:        "John",
			ShortName:       "",
			Email:           "mtest@mail.test",
			Password:        "123a123",
			SignupPromoCode: "promo1",
		}

		regToken, err := service.CreateRegToken(ctx, 2)
		require.NoError(t, err)

		rootUser, err := service.CreateUser(ctx, createUser, regToken.Secret)
		require.NoError(t, err)

		couponType, err := paymentsService.Accounts().Setup(ctx, rootUser.ID, rootUser.Email, rootUser.SignupPromoCode)

		var signupCouponType payments.CouponType = payments.SignupCoupon

		require.NoError(t, err)
		assert.Equal(t, signupCouponType, couponType)

		t.Run("Activation", func(t *testing.T) {
			activationToken, err := service.GenerateActivationToken(
				ctx,
				rootUser.ID,
				"mtest@mail.test",
			)
			require.NoError(t, err)
			_, err = service.ActivateAccount(ctx, activationToken)
			require.NoError(t, err)
			rootUser.Email = "mtest@mail.test"
		})

		tokenInfo, err := service.Token(ctx, console.AuthUser{Email: createUser.Email, Password: createUser.Password})
		require.NoError(t, err)

		userCtx, err := service.TokenAuth(ctx, tokenInfo.Token, time.Now())
		require.NoError(t, err)

		testQuery := func(t *testing.T, query string) interface{} {
			result := graphql.Do(graphql.Params{
				Schema:        schema,
				Context:       userCtx,
				RequestString: query,
				RootObject:    rootObject,
			})

			for _, err := range result.Errors {
				assert.NoError(t, err)
			}
			require.False(t, result.HasErrors())

			return result.Data
		}

		createdProject, err := service.CreateProject(userCtx, console.ProjectInfo{
			Name: "TestProject",
		})
		require.NoError(t, err)

		// "query {project(id:\"%s\"){id,name,members(offset:0, limit:50){user{fullName,shortName,email}},apiKeys{name,id,createdAt,projectID}}}"
		t.Run("Project query base info", func(t *testing.T) {
			query := fmt.Sprintf(
				"query {project(id:\"%s\"){id,name,publicId,description,createdAt}}",
				createdProject.ID.String(),
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			project := data[consoleql.ProjectQuery].(map[string]interface{})

			assert.Equal(t, createdProject.ID.String(), project[consoleql.FieldID])
			assert.Equal(t, createdProject.PublicID.String(), project[consoleql.FieldPublicID])
			assert.Equal(t, createdProject.Name, project[consoleql.FieldName])
			assert.Equal(t, createdProject.Description, project[consoleql.FieldDescription])

			createdAt := time.Time{}
			err := createdAt.UnmarshalText([]byte(project[consoleql.FieldCreatedAt].(string)))

			assert.NoError(t, err)
			assert.True(t, createdProject.CreatedAt.Equal(createdAt))

			// test getting by publicId
			query = fmt.Sprintf(
				"query {project(publicId:\"%s\"){id,name,publicId,description,createdAt}}",
				createdProject.PublicID.String(),
			)

			result = testQuery(t, query)

			data = result.(map[string]interface{})
			project = data[consoleql.ProjectQuery].(map[string]interface{})

			assert.Equal(t, createdProject.ID.String(), project[consoleql.FieldID])
			assert.Equal(t, createdProject.PublicID.String(), project[consoleql.FieldPublicID])
		})

		regTokenUser1, err := service.CreateRegToken(ctx, 2)
		require.NoError(t, err)

		user1, err := service.CreateUser(userCtx, console.CreateUser{
			FullName:  "Mickey Last",
			ShortName: "Last",
			Password:  "123a123",
			Email:     "muu1@mail.test",
		}, regTokenUser1.Secret)
		require.NoError(t, err)

		t.Run("Activation", func(t *testing.T) {
			activationToken1, err := service.GenerateActivationToken(
				ctx,
				user1.ID,
				"muu1@mail.test",
			)
			require.NoError(t, err)
			_, err = service.ActivateAccount(ctx, activationToken1)
			require.NoError(t, err)
			user1.Email = "muu1@mail.test"
		})

		regTokenUser2, err := service.CreateRegToken(ctx, 2)
		require.NoError(t, err)

		user2, err := service.CreateUser(userCtx, console.CreateUser{
			FullName:  "Dubas Name",
			ShortName: "Name",
			Email:     "muu2@mail.test",
			Password:  "123a123",
		}, regTokenUser2.Secret)
		require.NoError(t, err)

		t.Run("Activation", func(t *testing.T) {
			activationToken2, err := service.GenerateActivationToken(
				ctx,
				user2.ID,
				"muu2@mail.test",
			)
			require.NoError(t, err)
			_, err = service.ActivateAccount(ctx, activationToken2)
			require.NoError(t, err)
			user2.Email = "muu2@mail.test"
		})

		users, err := service.AddProjectMembers(userCtx, createdProject.ID, []string{
			user1.Email,
			user2.Email,
		})
		require.NoError(t, err)
		assert.Equal(t, 2, len(users))

		t.Run("Project query team members", func(t *testing.T) {
			query := fmt.Sprintf(
				"query {project(id: \"%s\") {members( cursor: { limit: %d, search: \"%s\", page: %d, order: %d, orderDirection: %d } ) { projectMembers{ user { id, fullName, shortName, email, createdAt }, joinedAt }, search, limit, order, offset, pageCount, currentPage, totalCount } } }",
				createdProject.ID.String(),
				5,
				"",
				1,
				1,
				2)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			project := data[consoleql.ProjectQuery].(map[string]interface{})
			members := project[consoleql.FieldMembers].(map[string]interface{})
			projectMembers := members[consoleql.FieldProjectMembers].([]interface{})

			assert.Equal(t, 3, len(projectMembers))

			testUser := func(t *testing.T, actual map[string]interface{}, expected *console.User) {
				assert.Equal(t, expected.Email, actual[consoleql.FieldEmail])
				assert.Equal(t, expected.FullName, actual[consoleql.FieldFullName])
				assert.Equal(t, expected.ShortName, actual[consoleql.FieldShortName])

				createdAt := time.Time{}
				err := createdAt.UnmarshalText([]byte(actual[consoleql.FieldCreatedAt].(string)))

				assert.NoError(t, err)
				assert.True(t, expected.CreatedAt.Equal(createdAt))
			}

			var foundRoot, foundU1, foundU2 bool

			for _, entry := range projectMembers {
				member := entry.(map[string]interface{})
				user := member[consoleql.UserType].(map[string]interface{})

				id := user[consoleql.FieldID].(string)
				switch id {
				case rootUser.ID.String():
					foundRoot = true
					testUser(t, user, rootUser)
				case user1.ID.String():
					foundU1 = true
					testUser(t, user, user1)
				case user2.ID.String():
					foundU2 = true
					testUser(t, user, user2)
				}
			}

			assert.True(t, foundRoot)
			assert.True(t, foundU1)
			assert.True(t, foundU2)
		})

		keyInfo1, _, err := service.CreateAPIKey(userCtx, createdProject.ID, "key1")
		require.NoError(t, err)

		keyInfo2, _, err := service.CreateAPIKey(userCtx, createdProject.ID, "key2")
		require.NoError(t, err)

		t.Run("Project query api keys", func(t *testing.T) {
			query := fmt.Sprintf(
				"query {project(id: \"%s\") {apiKeys( cursor: { limit: %d, search: \"%s\", page: %d, order: %d, orderDirection: %d } ) { apiKeys { id, name, createdAt, projectID }, search, limit, order, offset, pageCount, currentPage, totalCount } } }",
				createdProject.ID.String(),
				5,
				"",
				1,
				1,
				2)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			project := data[consoleql.ProjectQuery].(map[string]interface{})
			keys := project[consoleql.FieldAPIKeys].(map[string]interface{})
			apiKeys := keys[consoleql.FieldAPIKeys].([]interface{})

			assert.Equal(t, 2, len(apiKeys))

			testAPIKey := func(t *testing.T, actual map[string]interface{}, expected *console.APIKeyInfo) {
				assert.Equal(t, expected.Name, actual[consoleql.FieldName])
				assert.Equal(t, expected.ProjectID.String(), actual[consoleql.FieldProjectID])

				createdAt := time.Time{}
				err := createdAt.UnmarshalText([]byte(actual[consoleql.FieldCreatedAt].(string)))

				assert.NoError(t, err)
				assert.True(t, expected.CreatedAt.Equal(createdAt))
			}

			var foundKey1, foundKey2 bool

			for _, entry := range apiKeys {
				key := entry.(map[string]interface{})

				id := key[consoleql.FieldID].(string)
				switch id {
				case keyInfo1.ID.String():
					foundKey1 = true
					testAPIKey(t, key, keyInfo1)
				case keyInfo2.ID.String():
					foundKey2 = true
					testAPIKey(t, key, keyInfo2)
				}
			}

			assert.True(t, foundKey1)
			assert.True(t, foundKey2)
		})

		project2, err := service.CreateProject(userCtx, console.ProjectInfo{
			Name:        "Project2",
			Description: "Test desc",
		})
		require.NoError(t, err)

		t.Run("MyProjects query", func(t *testing.T) {
			query := "query {myProjects{id,publicId,name,description,createdAt}}"

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			projectsList := data[consoleql.MyProjectsQuery].([]interface{})

			assert.Equal(t, 2, len(projectsList))

			testProject := func(t *testing.T, actual map[string]interface{}, expected *console.Project) {
				assert.Equal(t, expected.Name, actual[consoleql.FieldName])
				assert.Equal(t, expected.PublicID.String(), actual[consoleql.FieldPublicID])
				assert.Equal(t, expected.Description, actual[consoleql.FieldDescription])

				createdAt := time.Time{}
				err := createdAt.UnmarshalText([]byte(actual[consoleql.FieldCreatedAt].(string)))

				assert.NoError(t, err)
				assert.True(t, expected.CreatedAt.Equal(createdAt))
			}

			var foundProj1, foundProj2 bool

			for _, entry := range projectsList {
				project := entry.(map[string]interface{})

				id := project[consoleql.FieldID].(string)
				switch id {
				case createdProject.ID.String():
					foundProj1 = true
					testProject(t, project, createdProject)
				case project2.ID.String():
					foundProj2 = true
					testProject(t, project, project2)
				}
			}

			assert.True(t, foundProj1)
			assert.True(t, foundProj2)
		})
		t.Run("OwnedProjects query", func(t *testing.T) {
			query := fmt.Sprintf(
				"query {ownedProjects( cursor: { limit: %d, page: %d } ) {projects{id, publicId, name, ownerId, description, createdAt, memberCount}, limit, offset, pageCount, currentPage, totalCount } }",
				5,
				1,
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			projectsPage := data[consoleql.OwnedProjectsQuery].(map[string]interface{})

			projectsList := projectsPage[consoleql.FieldProjects].([]interface{})
			assert.Len(t, projectsList, 2)

			assert.EqualValues(t, 1, projectsPage[consoleql.FieldCurrentPage])
			assert.EqualValues(t, 0, projectsPage[consoleql.OffsetArg])
			assert.EqualValues(t, 5, projectsPage[consoleql.LimitArg])
			assert.EqualValues(t, 1, projectsPage[consoleql.FieldPageCount])
			assert.EqualValues(t, 2, projectsPage[consoleql.FieldTotalCount])

			testProject := func(t *testing.T, actual map[string]interface{}, expected *console.Project, expectedNumMembers int) {
				assert.Equal(t, expected.Name, actual[consoleql.FieldName])
				assert.Equal(t, expected.PublicID.String(), actual[consoleql.FieldPublicID])
				assert.Equal(t, expected.Description, actual[consoleql.FieldDescription])

				createdAt := time.Time{}
				err := createdAt.UnmarshalText([]byte(actual[consoleql.FieldCreatedAt].(string)))

				assert.NoError(t, err)
				assert.True(t, expected.CreatedAt.Equal(createdAt))

				assert.EqualValues(t, expectedNumMembers, actual[consoleql.FieldMemberCount])
			}

			var foundProj1, foundProj2 bool

			for _, entry := range projectsList {
				project := entry.(map[string]interface{})

				id := project[consoleql.FieldID].(string)
				switch id {
				case createdProject.ID.String():
					foundProj1 = true
					testProject(t, project, createdProject, 3)
				case project2.ID.String():
					foundProj2 = true
					testProject(t, project, project2, 1)
				}
			}

			assert.True(t, foundProj1)
			assert.True(t, foundProj2)
		})
	})
}
