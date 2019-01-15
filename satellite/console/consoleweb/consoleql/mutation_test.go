// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/satellitedb"
)

func TestGrapqhlMutation(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zap.NewExample()

	db, err := satellitedb.NewConsoleDB("sqlite3", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}

	if err = db.CreateTables(); err != nil {
		t.Fatal(err)
	}

	service, err := console.NewService(
		log,
		&consoleauth.Hmac{Secret: []byte("my-suppa-secret-key")},
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

	createUser := console.CreateUser{
		UserInfo: console.UserInfo{
			FirstName: "John",
			LastName:  "Roll",
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

	authCtx := console.WithAuth(ctx, sauth)

	t.Run("Create user mutation", func(t *testing.T) {
		newUser := console.CreateUser{
			UserInfo: console.UserInfo{
				FirstName: "Mickey",
				LastName:  "Green",
				Email:     "u1@email.com",
			},
			Password: "123a123",
		}

		query := fmt.Sprintf(
			"mutation {createUser(input:{email:\"%s\",password:\"%s\",firstName:\"%s\",lastName:\"%s\"})}",
			newUser.Email,
			newUser.Password,
			newUser.FirstName,
			newUser.LastName,
		)

		result := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       ctx,
			RequestString: query,
			RootObject:    make(map[string]interface{}),
		})

		for _, err := range result.Errors {
			assert.NoError(t, err)
		}

		if result.HasErrors() {
			t.Fatal()
		}

		data := result.Data.(map[string]interface{})
		id := data[createUserMutation].(string)

		uID, err := uuid.Parse(id)
		assert.NoError(t, err)

		user, err := service.GetUser(authCtx, *uID)
		assert.NoError(t, err)

		assert.Equal(t, newUser.Email, user.Email)
		assert.Equal(t, newUser.FirstName, user.FirstName)
		assert.Equal(t, newUser.LastName, user.LastName)
	})

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

	t.Run("Update account mutation email only", func(t *testing.T) {
		email := "new@email.com"
		query := fmt.Sprintf(
			"mutation {updateAccount(input:{email:\"%s\"}){id,email,firstName,lastName,createdAt}}",
			email,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		user := data[updateAccountMutation].(map[string]interface{})

		assert.Equal(t, rootUser.ID.String(), user[fieldID])
		assert.Equal(t, email, user[fieldEmail])
		assert.Equal(t, rootUser.FirstName, user[fieldFirstName])
		assert.Equal(t, rootUser.LastName, user[fieldLastName])
	})

	t.Run("Update account mutation firstName only", func(t *testing.T) {
		firstName := "George"
		query := fmt.Sprintf(
			"mutation {updateAccount(input:{firstName:\"%s\"}){id,email,firstName,lastName,createdAt}}",
			firstName,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		user := data[updateAccountMutation].(map[string]interface{})

		assert.Equal(t, rootUser.ID.String(), user[fieldID])
		assert.Equal(t, rootUser.Email, user[fieldEmail])
		assert.Equal(t, firstName, user[fieldFirstName])
		assert.Equal(t, rootUser.LastName, user[fieldLastName])
	})

	t.Run("Update account mutation lastName only", func(t *testing.T) {
		lastName := "Yellow"
		query := fmt.Sprintf(
			"mutation {updateAccount(input:{lastName:\"%s\"}){id,email,firstName,lastName,createdAt}}",
			lastName,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		user := data[updateAccountMutation].(map[string]interface{})

		assert.Equal(t, rootUser.ID.String(), user[fieldID])
		assert.Equal(t, rootUser.Email, user[fieldEmail])
		assert.Equal(t, rootUser.FirstName, user[fieldFirstName])
		assert.Equal(t, lastName, user[fieldLastName])
	})

	t.Run("Update account mutation all info", func(t *testing.T) {
		email := "test@newmail.com"
		firstName := "Fill"
		lastName := "Goal"

		query := fmt.Sprintf(
			"mutation {updateAccount(input:{email:\"%s\",firstName:\"%s\",lastName:\"%s\"}){id,email,firstName,lastName,createdAt}}",
			email,
			firstName,
			lastName,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		user := data[updateAccountMutation].(map[string]interface{})

		assert.Equal(t, rootUser.ID.String(), user[fieldID])
		assert.Equal(t, email, user[fieldEmail])
		assert.Equal(t, firstName, user[fieldFirstName])
		assert.Equal(t, lastName, user[fieldLastName])

		createdAt := time.Time{}
		err := createdAt.UnmarshalText([]byte(user[fieldCreatedAt].(string)))

		assert.NoError(t, err)
		assert.Equal(t, rootUser.CreatedAt, createdAt)
	})

	t.Run("Change password mutation", func(t *testing.T) {
		newPassword := "145a145a"

		query := fmt.Sprintf(
			"mutation {changePassword(password:\"%s\",newPassword:\"%s\"){id,email,firstName,lastName,createdAt}}",
			createUser.Password,
			newPassword,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		user := data[changePasswordMutation].(map[string]interface{})

		assert.Equal(t, rootUser.ID.String(), user[fieldID])
		assert.Equal(t, rootUser.Email, user[fieldEmail])
		assert.Equal(t, rootUser.FirstName, user[fieldFirstName])
		assert.Equal(t, rootUser.LastName, user[fieldLastName])

		createdAt := time.Time{}
		err := createdAt.UnmarshalText([]byte(user[fieldCreatedAt].(string)))

		assert.NoError(t, err)
		assert.Equal(t, rootUser.CreatedAt, createdAt)

		oldHash := rootUser.PasswordHash

		rootUser, err = service.GetUser(authCtx, rootUser.ID)
		if err != nil {
			t.Fatal(err)
		}

		assert.False(t, bytes.Equal(oldHash, rootUser.PasswordHash))

		createUser.Password = newPassword
	})

	token, err = service.Token(ctx, rootUser.Email, createUser.Password)
	if err != nil {
		t.Fatal(err)
	}

	sauth, err = service.Authorize(auth.WithAPIKey(ctx, []byte(token)))
	if err != nil {
		t.Fatal(err)
	}

	authCtx = console.WithAuth(ctx, sauth)

	var projectID string
	t.Run("Create project mutation", func(t *testing.T) {
		projectInfo := console.ProjectInfo{
			Name:        "Project name",
			Description: "desc",
		}

		query := fmt.Sprintf(
			"mutation {createProject(input:{name:\"%s\",description:\"%s\",isTermsAccepted:true}){name,description,id,createdAt}}",
			projectInfo.Name,
			projectInfo.Description,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		project := data[createProjectMutation].(map[string]interface{})

		assert.Equal(t, projectInfo.Name, project[fieldName])
		assert.Equal(t, projectInfo.Description, project[fieldDescription])

		projectID = project[fieldID].(string)
	})

	pID, err := uuid.Parse(projectID)
	if err != nil {
		t.Fatal(err)
	}

	project, err := service.GetProject(authCtx, *pID)
	if err != nil {
		t.Fatal(err, project)
	}

	t.Run("Update project description mutation", func(t *testing.T) {
		query := fmt.Sprintf(
			"mutation {updateProjectDescription(id:\"%s\",description:\"%s\"){id,name,description}}",
			project.ID.String(),
			"",
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		proj := data[updateProjectDescriptionMutation].(map[string]interface{})

		assert.Equal(t, project.ID.String(), proj[fieldID])
		assert.Equal(t, project.Name, proj[fieldName])
		assert.Equal(t, "", proj[fieldDescription])
	})

	user1, err := service.CreateUser(authCtx, console.CreateUser{
		UserInfo: console.UserInfo{
			FirstName: "User1",
			Email:     "u1@email.net",
		},
		Password: "123a123",
	})

	if err != nil {
		t.Fatal(err, project)
	}

	user2, err := service.CreateUser(authCtx, console.CreateUser{
		UserInfo: console.UserInfo{
			FirstName: "User1",
			Email:     "u2@email.net",
		},
		Password: "123a123",
	})

	if err != nil {
		t.Fatal(err, project)
	}

	t.Run("Add project members mutation", func(t *testing.T) {
		query := fmt.Sprintf(
			"mutation {addProjectMembers(projectID:\"%s\",email:[\"%s\",\"%s\"]){id,name,members(limit:50,offset:0){joinedAt}}}",
			project.ID.String(),
			user1.Email,
			user2.Email,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		proj := data[addProjectMembersMutation].(map[string]interface{})

		assert.Equal(t, project.ID.String(), proj[fieldID])
		assert.Equal(t, project.Name, proj[fieldName])
		assert.Equal(t, 3, len(proj[fieldMembers].([]interface{})))
	})

	t.Run("Delete project members mutation", func(t *testing.T) {
		query := fmt.Sprintf(
			"mutation {deleteProjectMembers(projectID:\"%s\",email:[\"%s\",\"%s\"]){id,name,members(limit:50,offset:0){user{id}}}}",
			project.ID.String(),
			user1.Email,
			user2.Email,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		proj := data[deleteProjectMembersMutation].(map[string]interface{})

		members := proj[fieldMembers].([]interface{})
		rootMember := members[0].(map[string]interface{})[userType].(map[string]interface{})

		assert.Equal(t, project.ID.String(), proj[fieldID])
		assert.Equal(t, project.Name, proj[fieldName])
		assert.Equal(t, 1, len(members))

		assert.Equal(t, rootUser.ID.String(), rootMember[fieldID])
	})

	var keyID string
	t.Run("Create api key mutation", func(t *testing.T) {
		keyName := "key1"
		query := fmt.Sprintf(
			"mutation {createAPIKey(projectID:\"%s\",name:\"%s\"){key,keyInfo{id,name,projectID}}}",
			project.ID.String(),
			keyName,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		createAPIKey := data[createAPIKeyMutation].(map[string]interface{})

		key := createAPIKey[fieldKey].(string)
		keyInfo := createAPIKey[apiKeyInfoType].(map[string]interface{})

		assert.NotEqual(t, "", key)

		assert.Equal(t, keyName, keyInfo[fieldName])
		assert.Equal(t, project.ID.String(), keyInfo[fieldProjectID])

		keyID = keyInfo[fieldID].(string)
	})

	t.Run("Delete api key mutation", func(t *testing.T) {
		id, err := uuid.Parse(keyID)
		if err != nil {
			t.Fatal(err)
		}

		info, err := service.GetAPIKeyInfo(authCtx, *id)
		if err != nil {
			t.Fatal(err)
		}

		query := fmt.Sprintf(
			"mutation {deleteAPIKey(id:\"%s\"){name,projectID}}",
			id.String(),
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		keyInfo := data[deleteAPIKeyMutation].(map[string]interface{})

		assert.Equal(t, info.Name, keyInfo[fieldName])
		assert.Equal(t, project.ID.String(), keyInfo[fieldProjectID])
	})

	t.Run("Delete project mutation", func(t *testing.T) {
		query := fmt.Sprintf(
			"mutation {deleteProject(id:\"%s\"){id,name}}",
			projectID,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		proj := data[deleteProjectMutation].(map[string]interface{})

		assert.Equal(t, project.Name, proj[fieldName])
		assert.Equal(t, project.ID.String(), proj[fieldID])

		_, err := service.GetProject(authCtx, project.ID)
		assert.Error(t, err)
	})

	t.Run("Delete account mutation", func(t *testing.T) {
		query := fmt.Sprintf(
			"mutation {deleteAccount(password:\"%s\"){id}}",
			createUser.Password,
		)

		result := testQuery(t, query)

		data := result.(map[string]interface{})
		user := data[deleteAccountMutation].(map[string]interface{})

		assert.Equal(t, rootUser.ID.String(), user[fieldID])

		_, err := service.GetUser(authCtx, rootUser.ID)
		assert.Error(t, err)
	})
}
