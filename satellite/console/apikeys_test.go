// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestApiKeysRepository(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		projects := db.Console().Projects()
		apikeys := db.Console().APIKeys()

		project, err := projects.Insert(ctx, &console.Project{
			Name:        "ProjectName",
			Description: "projects description",
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
	})
}
