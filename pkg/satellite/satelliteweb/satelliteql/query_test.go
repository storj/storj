package satelliteql

import (
	"fmt"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/stretchr/testify/assert"

	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/satellite"
	"storj.io/storj/pkg/satellite/satelliteauth"
	"storj.io/storj/pkg/satellite/satellitedb"
)

func TestGraphqlQuery(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zap.NewExample()

	db, err := satellitedb.New("sqlite3", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}

	if err = db.CreateTables(); err != nil {
		t.Fatal(err)
	}

	service, err := satellite.NewService(
		log,
		&satelliteauth.Hmac{Secret: []byte("my-suppa-secret-key")},
		db,
	)

	if err != nil {
		t.Fatal(err)
	}

	creator := TypeCreator{}
	if err = creator.Create(service); err != nil {
		t.Fatal(err)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    creator.RootQuery(),
		Mutation: creator.RootMutation(),
	})

	if err != nil {
		t.Fatal(err)
	}

	createUser := satellite.CreateUser{
		UserInfo: satellite.UserInfo{
			FirstName: "John",
			LastName:  "",
			Email:     "test@email.com",
		},
		Password: "123a123",
	}

	rootUser, err := service.CreateUser(ctx, createUser)
	if err != nil {
		t.Fatal(err)
	}

	token, err := service.Token(ctx, createUser.Email, createUser.Password)
	if err != nil {
		t.Fatal(err)
	}

	sauth, err := service.Authorize(auth.WithAPIKey(ctx, []byte(token)))
	if err != nil {
		t.Fatal(err)
	}

	authCtx := satellite.WithAuth(ctx, sauth)

	testQuery := func(t *testing.T, query string) interface{} {
		result := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       authCtx,
			RequestString: query,
			RootObject:    make(map[string]interface{}),
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
		testUser := func(t *testing.T, actual map[string]interface{}, expected *satellite.User) {
			assert.Equal(t, expected.ID.String(), actual[fieldID])
			assert.Equal(t, expected.Email, actual[fieldEmail])
			assert.Equal(t, expected.FirstName, actual[fieldFirstName])
			assert.Equal(t, expected.LastName, actual[fieldLastName])

			createdAt := time.Time{}
			err := createdAt.UnmarshalText([]byte(actual[fieldCreatedAt].(string)))

			assert.NoError(t, err)
			assert.Equal(t, expected.CreatedAt, createdAt)
		}

		t.Run("With ID", func(t *testing.T) {
			query := fmt.Sprintf(
				"query {user(id:\"%s\"){id,email,firstName,lastName,createdAt}}",
				rootUser.ID.String(),
			)

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			user := data[userQuery].(map[string]interface{})

			testUser(t, user, rootUser)
		})

		t.Run("With AuthFallback", func(t *testing.T) {
			query := "query {user{id,email,firstName,lastName,createdAt}}"

			result := testQuery(t, query)

			data := result.(map[string]interface{})
			user := data[userQuery].(map[string]interface{})

			testUser(t, user, rootUser)
		})
	})

	createdProject, err := service.CreateProject(authCtx, satellite.ProjectInfo{
		Name:            "TestProject",
		IsTermsAccepted: true,
	})

	if err != nil {
		t.Fatal(err)
	}

	// "query {project(id:\"%s\"){id,name,members(offset:0, limit:50){user{firstName,lastName,email}},apiKeys{name,id,createdAt,projectID}}}"
	t.Run("Project query base info", func(t *testing.T) {
		query := fmt.Sprintf(
			"query {project(id:\"%s\"){id,name,description,createdAt}}",
			createdProject.ID.String(),
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		project := data[projectQuery].(map[string]interface{})

		assert.Equal(t, createdProject.ID.String(), project[fieldID])
		assert.Equal(t, createdProject.Name, project[fieldName])
		assert.Equal(t, createdProject.Description, project[fieldDescription])

		createdAt := time.Time{}
		err := createdAt.UnmarshalText([]byte(project[fieldCreatedAt].(string)))

		assert.NoError(t, err)
		assert.Equal(t, createdProject.CreatedAt, createdAt)
	})

	user1, err := service.CreateUser(authCtx, satellite.CreateUser{
		UserInfo: satellite.UserInfo{
			FirstName: "Mickey",
			LastName:  "Last",
			Email:     "uu1@email.com",
		},
		Password: "123a123",
	})

	if err != nil {
		t.Fatal(err)
	}

	user2, err := service.CreateUser(authCtx, satellite.CreateUser{
		UserInfo: satellite.UserInfo{
			FirstName: "Dubas",
			LastName:  "Name",
			Email:     "uu2@email.com",
		},
		Password: "123a123",
	})

	if err != nil {
		t.Fatal(err)
	}

	err = service.AddProjectMembers(authCtx, createdProject.ID, []string{
		user1.Email,
		user2.Email,
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Run("Project query team members", func(t *testing.T) {
		query := fmt.Sprintf(
			"query {project(id:\"%s\"){members(offset:0, limit:50){user{id,firstName,lastName,email,createdAt}}}}",
			createdProject.ID.String(),
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		project := data[projectQuery].(map[string]interface{})
		members := project[fieldMembers].([]interface{})

		assert.Equal(t, 3, len(members))

		testUser := func(t *testing.T, actual map[string]interface{}, expected *satellite.User) {
			assert.Equal(t, expected.Email, actual[fieldEmail])
			assert.Equal(t, expected.FirstName, actual[fieldFirstName])
			assert.Equal(t, expected.LastName, actual[fieldLastName])

			createdAt := time.Time{}
			err := createdAt.UnmarshalText([]byte(actual[fieldCreatedAt].(string)))

			assert.NoError(t, err)
			assert.Equal(t, expected.CreatedAt, createdAt)
		}

		var foundRoot, foundU1, foundU2 bool

		for _, entry := range members {
			member := entry.(map[string]interface{})
			user := member[userType].(map[string]interface{})

			id := user[fieldID].(string)
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
		project := data[projectQuery].(map[string]interface{})
		keys := project[fieldAPIKeys].([]interface{})

		assert.Equal(t, 2, len(keys))

		testAPIKey := func(t *testing.T, actual map[string]interface{}, expected *satellite.APIKeyInfo) {
			assert.Equal(t, expected.Name, actual[fieldName])
			assert.Equal(t, expected.ProjectID.String(), actual[fieldProjectID])

			createdAt := time.Time{}
			err := createdAt.UnmarshalText([]byte(actual[fieldCreatedAt].(string)))

			assert.NoError(t, err)
			assert.Equal(t, expected.CreatedAt, createdAt)
		}

		var foundKey1, foundKey2 bool

		for _, entry := range keys {
			key := entry.(map[string]interface{})

			id := key[fieldID].(string)
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

	project2, err := service.CreateProject(authCtx, satellite.ProjectInfo{
		Name:            "Project2",
		Description:     "Test desc",
		IsTermsAccepted: true,
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Run("MyProjects query", func(t *testing.T) {
		query := "query {myProjects{id,name,description,createdAt}}"

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		projectsList := data[myProjectsQuery].([]interface{})

		assert.Equal(t, 2, len(projectsList))

		testProject := func(t *testing.T, actual map[string]interface{}, expected *satellite.Project) {
			assert.Equal(t, expected.Name, actual[fieldName])
			assert.Equal(t, expected.Description, actual[fieldDescription])

			createdAt := time.Time{}
			err := createdAt.UnmarshalText([]byte(actual[fieldCreatedAt].(string)))

			assert.NoError(t, err)
			assert.Equal(t, expected.CreatedAt, createdAt)
		}

		var foundProj1, foundProj2 bool

		for _, entry := range projectsList {
			project := entry.(map[string]interface{})

			id := project[fieldID].(string)
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
			"query {token(email: \"%s\", password: \"%s\"){token,user{id,email,firstName,lastName,createdAt}}}",
			createUser.Email,
			createUser.Password,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		queryToken := data[tokenQuery].(map[string]interface{})

		token := queryToken[tokenType].(string)
		user := queryToken[userType].(map[string]interface{})

		tauth, err := service.Authorize(auth.WithAPIKey(ctx, []byte(token)))
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, rootUser.ID, tauth.User.ID)
		assert.Equal(t, rootUser.ID.String(), user[fieldID])
		assert.Equal(t, rootUser.Email, user[fieldEmail])
		assert.Equal(t, rootUser.FirstName, user[fieldFirstName])
		assert.Equal(t, rootUser.LastName, user[fieldLastName])

		createdAt := time.Time{}
		err = createdAt.UnmarshalText([]byte(user[fieldCreatedAt].(string)))

		assert.NoError(t, err)
		assert.Equal(t, rootUser.CreatedAt, createdAt)
	})
}
