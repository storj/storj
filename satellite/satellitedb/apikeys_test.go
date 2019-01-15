// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"fmt"
	"testing"

	"storj.io/storj/satellite/console"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/testcontext"
)

func TestApiKeysRepository(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	db, err := NewConsoleDB("sqlite3", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(db.Close)

	// creating tables
	err = db.CreateTables()
	if err != nil {
		t.Fatal(err)
	}

	projects := db.Projects()
	apikeys := db.APIKeys()

	project, err := projects.Insert(ctx, &console.Project{
		Name:          "ProjectName",
		TermsAccepted: 1,
		Description:   "projects description",
	})
	assert.NotNil(t, project)
	assert.NoError(t, err)

	t.Run("Creation success", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			key, err := console.CreateAPIKey()
			assert.NoError(t, err)

			keyInfo := console.APIKeyInfo{
				Name:      fmt.Sprintf("key %d", i),
				ProjectID: project.ID,
			}

			createdKey, err := apikeys.Create(ctx, *key, keyInfo)
			assert.NotNil(t, createdKey)
			assert.NoError(t, err)
		}
	})

	t.Run("GetByProjectID success", func(t *testing.T) {
		keys, err := apikeys.GetByProjectID(ctx, project.ID)
		assert.NotNil(t, keys)
		assert.Equal(t, len(keys), 10)
		assert.NoError(t, err)
	})

	t.Run("Get By ID success", func(t *testing.T) {
		keys, err := apikeys.GetByProjectID(ctx, project.ID)
		assert.NotNil(t, keys)
		assert.Equal(t, len(keys), 10)
		assert.NoError(t, err)

		key, err := apikeys.Get(ctx, keys[0].ID)
		assert.NotNil(t, key)
		assert.Equal(t, keys[0].ID, key.ID)
		assert.NoError(t, err)
	})

	t.Run("Update success", func(t *testing.T) {
		keys, err := apikeys.GetByProjectID(ctx, project.ID)
		assert.NotNil(t, keys)
		assert.Equal(t, len(keys), 10)
		assert.NoError(t, err)

		key, err := apikeys.Get(ctx, keys[0].ID)
		assert.NotNil(t, key)
		assert.Equal(t, keys[0].ID, key.ID)
		assert.NoError(t, err)

		key.Name = "some new name"

		err = apikeys.Update(ctx, *key)
		assert.NoError(t, err)

		updatedKey, err := apikeys.Get(ctx, keys[0].ID)
		assert.NotNil(t, key)
		assert.Equal(t, key.Name, updatedKey.Name)
		assert.NoError(t, err)
	})

	t.Run("Delete success", func(t *testing.T) {
		keys, err := apikeys.GetByProjectID(ctx, project.ID)
		assert.NotNil(t, keys)
		assert.Equal(t, len(keys), 10)
		assert.NoError(t, err)

		key, err := apikeys.Get(ctx, keys[0].ID)
		assert.NotNil(t, key)
		assert.Equal(t, keys[0].ID, key.ID)
		assert.NoError(t, err)

		key.Name = "some new name"

		err = apikeys.Delete(ctx, key.ID)
		assert.NoError(t, err)

		keys, err = apikeys.GetByProjectID(ctx, project.ID)
		assert.NotNil(t, keys)
		assert.Equal(t, len(keys), 9)
		assert.NoError(t, err)
	})
}
