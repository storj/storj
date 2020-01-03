// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/macaroon"
	"storj.io/common/testcontext"
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
				key, err := macaroon.NewAPIKey([]byte("testSecret"))
				assert.NoError(t, err)

				keyInfo := console.APIKeyInfo{
					Name:      fmt.Sprintf("key %d", i),
					ProjectID: project.ID,
					Secret:    []byte("testSecret"),
				}

				createdKey, err := apikeys.Create(ctx, key.Head(), keyInfo)
				assert.NotNil(t, createdKey)
				assert.NoError(t, err)
			}
		})

		t.Run("GetPagedByProjectID success", func(t *testing.T) {
			cursor := console.APIKeyCursor{
				Page:   1,
				Limit:  10,
				Search: "",
			}
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor)

			assert.NotNil(t, page)
			assert.Equal(t, len(page.APIKeys), 10)
			assert.NoError(t, err)
		})

		t.Run("GetPagedByProjectID with limit success", func(t *testing.T) {
			cursor := console.APIKeyCursor{
				Page:   1,
				Limit:  2,
				Search: "",
			}
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor)

			assert.NotNil(t, page)
			assert.Equal(t, len(page.APIKeys), 2)
			assert.Equal(t, page.PageCount, uint(5))
			assert.NoError(t, err)
		})

		t.Run("Get By ID success", func(t *testing.T) {
			cursor := console.APIKeyCursor{
				Page:   1,
				Limit:  10,
				Search: "",
			}
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor)

			assert.NotNil(t, page)
			assert.Equal(t, len(page.APIKeys), 10)
			assert.NoError(t, err)

			key, err := apikeys.Get(ctx, page.APIKeys[0].ID)
			assert.NotNil(t, key)
			assert.Equal(t, page.APIKeys[0].ID, key.ID)
			assert.NoError(t, err)
		})

		t.Run("Update success", func(t *testing.T) {
			cursor := console.APIKeyCursor{
				Page:   1,
				Limit:  10,
				Search: "",
			}
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor)
			assert.NotNil(t, page)
			assert.Equal(t, len(page.APIKeys), 10)
			assert.NoError(t, err)

			key, err := apikeys.Get(ctx, page.APIKeys[0].ID)
			assert.NotNil(t, key)
			assert.Equal(t, page.APIKeys[0].ID, key.ID)
			assert.NoError(t, err)

			key.Name = "some new name"

			err = apikeys.Update(ctx, *key)
			assert.NoError(t, err)

			updatedKey, err := apikeys.Get(ctx, page.APIKeys[0].ID)
			assert.NotNil(t, key)
			assert.Equal(t, key.Name, updatedKey.Name)
			assert.NoError(t, err)
		})

		t.Run("Delete success", func(t *testing.T) {
			cursor := console.APIKeyCursor{
				Page:   1,
				Limit:  10,
				Search: "",
			}
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor)
			assert.NotNil(t, page)
			assert.Equal(t, len(page.APIKeys), 10)
			assert.NoError(t, err)

			key, err := apikeys.Get(ctx, page.APIKeys[0].ID)
			assert.NotNil(t, key)
			assert.Equal(t, page.APIKeys[0].ID, key.ID)
			assert.NoError(t, err)

			key.Name = "some new name"

			err = apikeys.Delete(ctx, key.ID)
			assert.NoError(t, err)

			page, err = apikeys.GetPagedByProjectID(ctx, project.ID, cursor)
			assert.NotNil(t, page)
			assert.Equal(t, len(page.APIKeys), 9)
			assert.NoError(t, err)
		})

		t.Run("GetPageByProjectID with 0 page error", func(t *testing.T) {
			cursor := console.APIKeyCursor{
				Page:   0,
				Limit:  10,
				Search: "",
			}
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor)

			assert.Nil(t, page)
			assert.Error(t, err)
		})

	})
}
