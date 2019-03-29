// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGraphqlQuery(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		log := zaptest.NewLogger(t)

		service, err := console.NewService(
			log,
			&consoleauth.Hmac{Secret: []byte("my-suppa-secret-key")},
			db.Console(),
			console.TestPasswordCost,
		)

		if err != nil {
			t.Fatal(err)
		}

		mailService, err := mailservice.New(log, &discardSender{}, "testdata")
		if err != nil {
			t.Fatal(err)
		}

		rootObject := make(map[string]interface{})
		rootObject["origin"] = "http://doesntmatter.com/"
		rootObject[consoleql.ActivationPath] = "?activationToken="

		creator := consoleql.TypeCreator{}
		if err = creator.Create(log, service, mailService); err != nil {
			t.Fatal(err)
		}

		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    creator.RootQuery(),
			Mutation: creator.RootMutation(),
		})

		if err != nil {
			t.Fatal(err)
		}

		createUser := console.CreateUser{
			UserInfo: console.UserInfo{
				FullName:  "John",
				ShortName: "",
				Email:     "mtest@email.com",
			},
			Password: "123a123",
		}

		regToken, err := service.CreateRegToken(ctx, 2)
		if err != nil {
			t.Fatal(err)
		}

		rootUser, err := service.CreateUser(ctx, createUser, regToken.Secret)
		if err != nil {
			t.Fatal(err)
		}

		t.Run("Activation", func(t *testing.T) {
			activationToken, err := service.GenerateActivationToken(
				ctx,
				rootUser.ID,
				"mtest@email.com",
			)
			if err != nil {
				t.Fatal(err)
			}
			err = service.ActivateAccount(ctx, activationToken)
			if err != nil {
				t.Fatal(err)
			}
			rootUser.Email = "mtest@email.com"
		})

		token, err := service.Token(ctx, createUser.Email, createUser.Password)
		if err != nil {
			t.Fatal(err)
		}

		sauth, err := service.Authorize(auth.WithAPIKey(ctx, []byte(token)))
		if err != nil {
			t.Fatal(err)
		}

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

			if result.HasErrors() {
				t.Fatal()
			}

			return result.Data
		}

		t.Run("User query", func(t *testing.T) {
			testUser := func(t *testing.T, actual map[string]interface{}, expected *console.User) {
				assert.Equal(t, expected.ID.String(), actual[consoleql.FieldID])
				assert.Equal(t, expected.Email, actual[consoleql.FieldEmail])
				assert.Equal(t, expected.FullName, actual[consoleql.FieldFullName])
				assert.Equal(t, expected.ShortName, actual[consoleql.FieldShortName])

				createdAt := time.Time{}
				err := createdAt.UnmarshalText([]byte(actual[consoleql.FieldCreatedAt].(string)))

				assert.NoError(t, err)
				assert.True(t, expected.CreatedAt.Equal(createdAt))
			}

			t.Run("With ID", func(t *testing.T) {
				query := fmt.Sprintf(
					"query {user(id:\"%s\"){id,email,fullName,shortName,createdAt}}",
					rootUser.ID.String(),
				)

				result := testQuery(t, query)

				data := result.(map[string]interface{})
				user := data[consoleql.UserQuery].(map[string]interface{})

				testUser(t, user, rootUser)
			})

			t.Run("With AuthFallback", func(t *testing.T) {
				query := "query {user{id,email,fullName,shortName,createdAt}}"

				result := testQuery(t, query)

				data := result.(map[string]interface{})
				user := data[consoleql.UserQuery].(map[string]interface{})

				testUser(t, user, rootUser)
			})
		})

		createdProject, err := service.CreateProject(authCtx, console.ProjectInfo{
			Name: "TestProject",
		})

		if err != nil {
			t.Fatal(err)
		}

		// "query {project(id:\"%s\"){id,name,members(offset:0, limit:50){user{fullName,shortName,email}},apiKeys{name,id,createdAt,projectID}}}"
		t.Run("Project query base info", func(t *testing.T) {
			query := fmt.Sprintf(
				"query {project(id:\"%s\"){id,name,description,createdAt}}",
				createdProject.ID.String(),
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			project := data[consoleql.ProjectQuery].(map[string]interface{})

			assert.Equal(t, createdProject.ID.String(), project[consoleql.FieldID])
			assert.Equal(t, createdProject.Name, project[consoleql.FieldName])
			assert.Equal(t, createdProject.Description, project[consoleql.FieldDescription])

			createdAt := time.Time{}
			err := createdAt.UnmarshalText([]byte(project[consoleql.FieldCreatedAt].(string)))

			assert.NoError(t, err)
			assert.True(t, createdProject.CreatedAt.Equal(createdAt))
		})

		regTokenUser1, err := service.CreateRegToken(ctx, 2)
		if err != nil {
			t.Fatal(err)
		}

		user1, err := service.CreateUser(authCtx, console.CreateUser{
			UserInfo: console.UserInfo{
				FullName:  "Mickey Last",
				ShortName: "Last",
				Email:     "muu1@email.com",
			},
			Password: "123a123",
		}, regTokenUser1.Secret)

		if err != nil {
			t.Fatal(err)
		}

		t.Run("Activation", func(t *testing.T) {
			activationToken1, err := service.GenerateActivationToken(
				ctx,
				user1.ID,
				"muu1@email.com",
			)
			if err != nil {
				t.Fatal(err)
			}
			err = service.ActivateAccount(ctx, activationToken1)
			if err != nil {
				t.Fatal(err)
			}
			user1.Email = "muu1@email.com"

		})

		regTokenUser2, err := service.CreateRegToken(ctx, 2)
		if err != nil {
			t.Fatal(err)
		}

		user2, err := service.CreateUser(authCtx, console.CreateUser{
			UserInfo: console.UserInfo{
				FullName:  "Dubas Name",
				ShortName: "Name",
				Email:     "muu2@email.com",
			},
			Password: "123a123",
		}, regTokenUser2.Secret)

		if err != nil {
			t.Fatal(err)
		}

		t.Run("Activation", func(t *testing.T) {
			activationToken2, err := service.GenerateActivationToken(
				ctx,
				user2.ID,
				"muu2@email.com",
			)
			if err != nil {
				t.Fatal(err)
			}
			err = service.ActivateAccount(ctx, activationToken2)
			if err != nil {
				t.Fatal(err)
			}
			user2.Email = "muu2@email.com"
		})

		users, err := service.AddProjectMembers(authCtx, createdProject.ID, []string{
			user1.Email,
			user2.Email,
		})
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 2, len(users))

		t.Run("Project query team members", func(t *testing.T) {
			query := fmt.Sprintf(
				"query {project(id:\"%s\"){members(offset:0, limit:50){user{id,fullName,shortName,email,createdAt}}}}",
				createdProject.ID.String(),
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			project := data[consoleql.ProjectQuery].(map[string]interface{})
			members := project[consoleql.FieldMembers].([]interface{})

			assert.Equal(t, 3, len(members))

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

			for _, entry := range members {
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

		keyInfo1, _, err := service.CreateAPIKey(authCtx, createdProject.ID, "key1")
		if err != nil {
			t.Fatal(err)
		}

		keyInfo2, _, err := service.CreateAPIKey(authCtx, createdProject.ID, "key2")
		if err != nil {
			t.Fatal(err)
		}

		t.Run("Project query api keys", func(t *testing.T) {
			query := fmt.Sprintf(
				"query {project(id:\"%s\"){apiKeys{name,id,createdAt,projectID}}}",
				createdProject.ID.String(),
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			project := data[consoleql.ProjectQuery].(map[string]interface{})
			keys := project[consoleql.FieldAPIKeys].([]interface{})

			assert.Equal(t, 2, len(keys))

			testAPIKey := func(t *testing.T, actual map[string]interface{}, expected *console.APIKeyInfo) {
				assert.Equal(t, expected.Name, actual[consoleql.FieldName])
				assert.Equal(t, expected.ProjectID.String(), actual[consoleql.FieldProjectID])

				createdAt := time.Time{}
				err := createdAt.UnmarshalText([]byte(actual[consoleql.FieldCreatedAt].(string)))

				assert.NoError(t, err)
				assert.True(t, expected.CreatedAt.Equal(createdAt))
			}

			var foundKey1, foundKey2 bool

			for _, entry := range keys {
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

		project2, err := service.CreateProject(authCtx, console.ProjectInfo{
			Name:        "Project2",
			Description: "Test desc",
		})

		if err != nil {
			t.Fatal(err)
		}

		t.Run("MyProjects query", func(t *testing.T) {
			query := "query {myProjects{id,name,description,createdAt}}"

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			projectsList := data[consoleql.MyProjectsQuery].([]interface{})

			assert.Equal(t, 2, len(projectsList))

			testProject := func(t *testing.T, actual map[string]interface{}, expected *console.Project) {
				assert.Equal(t, expected.Name, actual[consoleql.FieldName])
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

		t.Run("Token query", func(t *testing.T) {
			query := fmt.Sprintf(
				"query {token(email: \"%s\", password: \"%s\"){token,user{id,email,fullName,shortName,createdAt}}}",
				createUser.Email,
				createUser.Password,
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			queryToken := data[consoleql.TokenQuery].(map[string]interface{})

			token := queryToken[consoleql.TokenType].(string)
			user := queryToken[consoleql.UserType].(map[string]interface{})

			tauth, err := service.Authorize(auth.WithAPIKey(ctx, []byte(token)))
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, rootUser.ID, tauth.User.ID)
			assert.Equal(t, rootUser.ID.String(), user[consoleql.FieldID])
			assert.Equal(t, rootUser.Email, user[consoleql.FieldEmail])
			assert.Equal(t, rootUser.FullName, user[consoleql.FieldFullName])
			assert.Equal(t, rootUser.ShortName, user[consoleql.FieldShortName])

			createdAt := time.Time{}
			err = createdAt.UnmarshalText([]byte(user[consoleql.FieldCreatedAt].(string)))

			assert.NoError(t, err)
			assert.True(t, rootUser.CreatedAt.Equal(createdAt))
		})
	})
}
