// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestApiKeysRepository(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projects := db.Console().Projects()
		apikeys := db.Console().APIKeys()
		users := db.Console().Users()
		pm := db.Console().ProjectMembers()

		project, err := projects.Insert(ctx, &console.Project{
			Name:        "ProjectName",
			Description: "projects description",
		})
		assert.NotNil(t, project)
		assert.NoError(t, err)

		userAgent := []byte("testUserAgent")

		t.Run("Creation success", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				key, err := macaroon.NewAPIKey([]byte("testSecret"))
				assert.NoError(t, err)

				keyInfo := console.APIKeyInfo{
					Name:      fmt.Sprintf("key %d", i),
					ProjectID: project.ID,
					Secret:    []byte("testSecret"),
					UserAgent: userAgent,
				}

				createdKey, err := apikeys.Create(ctx, key.Head(), keyInfo)
				assert.NotNil(t, createdKey)
				assert.NoError(t, err)
			}
		})

		t.Run("Get by head", func(t *testing.T) {
			limit := 2 * memory.B
			project.StorageLimit = &limit
			project.BandwidthLimit = &limit
			err = projects.Update(ctx, project)
			assert.NoError(t, err)

			key, err := macaroon.NewAPIKey([]byte("testSecret"))
			assert.NoError(t, err)

			createdKey, err := apikeys.Create(ctx, key.Head(), console.APIKeyInfo{
				Name:      "testKeyName",
				ProjectID: project.ID,
				Secret:    []byte("testSecret"),
				UserAgent: userAgent,
			})
			assert.NotNil(t, createdKey)
			assert.NoError(t, err)

			keyInfo, err := apikeys.GetByHead(ctx, key.Head())
			assert.NoError(t, err)
			assert.NotNil(t, keyInfo)
			assert.Equal(t, project.StorageLimit.Int64(), *keyInfo.ProjectStorageLimit)
			assert.Equal(t, project.BandwidthLimit.Int64(), *keyInfo.ProjectBandwidthLimit)

			limit /= 2
			project.UserSpecifiedStorageLimit = &limit
			project.UserSpecifiedBandwidthLimit = &limit
			err = projects.Update(ctx, project)
			assert.NoError(t, err)

			keyInfo, err = apikeys.GetByHead(ctx, key.Head())
			assert.NoError(t, err)
			assert.NotNil(t, keyInfo)
			assert.Equal(t, project.UserSpecifiedStorageLimit.Int64(), *keyInfo.ProjectStorageLimit)
			assert.Equal(t, project.UserSpecifiedBandwidthLimit.Int64(), *keyInfo.ProjectBandwidthLimit)

			err = apikeys.Delete(ctx, createdKey.ID)
			assert.NoError(t, err)
		})

		t.Run("GetPagedByProjectID success", func(t *testing.T) {
			cursor := console.APIKeyCursor{
				Page:   1,
				Limit:  10,
				Search: "",
			}
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor, "")

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
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor, "")

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
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor, "")

			assert.NotNil(t, page)
			assert.Equal(t, len(page.APIKeys), 10)
			assert.NoError(t, err)

			key, err := apikeys.Get(ctx, page.APIKeys[0].ID)
			assert.NotNil(t, key)
			assert.Equal(t, page.APIKeys[0].ID, key.ID)
			assert.Equal(t, page.APIKeys[0].UserAgent, userAgent)
			assert.NoError(t, err)
		})

		t.Run("Update success", func(t *testing.T) {
			cursor := console.APIKeyCursor{
				Page:   1,
				Limit:  10,
				Search: "",
			}
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor, "")
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
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor, "")
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

			page, err = apikeys.GetPagedByProjectID(ctx, project.ID, cursor, "")
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
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor, "")

			assert.Nil(t, page)
			assert.Error(t, err)
		})

		t.Run("GetAllNamesByProjectID success", func(t *testing.T) {
			project, err = projects.Insert(ctx, &console.Project{
				Name:        "ProjectName1",
				Description: "projects description",
			})
			assert.NotNil(t, project)
			assert.NoError(t, err)

			names, err := apikeys.GetAllNamesByProjectID(ctx, project.ID)
			assert.NoError(t, err)
			assert.Equal(t, 0, len(names))

			secret, err := macaroon.NewSecret()
			assert.NoError(t, err)

			key, err := macaroon.NewAPIKey(secret)
			assert.NoError(t, err)

			key1, err := macaroon.NewAPIKey(secret)
			assert.NoError(t, err)

			keyInfo := console.APIKeyInfo{
				Name:      "awesomeKey",
				ProjectID: project.ID,
				Secret:    secret,
				UserAgent: userAgent,
			}

			keyInfo1 := console.APIKeyInfo{
				Name:      "awesomeKey1",
				ProjectID: project.ID,
				Secret:    secret,
				UserAgent: userAgent,
			}

			createdKey, err := apikeys.Create(ctx, key.Head(), keyInfo)
			assert.NoError(t, err)
			assert.NotNil(t, createdKey)

			createdKey1, err := apikeys.Create(ctx, key1.Head(), keyInfo1)
			assert.NoError(t, err)
			assert.NotNil(t, createdKey1)

			names, err = apikeys.GetAllNamesByProjectID(ctx, project.ID)
			assert.NoError(t, err)
			assert.NotNil(t, names)
			assert.Equal(t, 2, len(names))
			assert.Equal(t, keyInfo.Name, names[0])
			assert.Equal(t, keyInfo1.Name, names[1])
		})

		t.Run("GetPagedByProjectID with excluding name prefix", func(t *testing.T) {
			pr, err := projects.Insert(ctx, &console.Project{
				Name: "ProjectName2",
			})
			assert.NotNil(t, pr)
			assert.NoError(t, err)

			secret, err := macaroon.NewSecret()
			assert.NoError(t, err)

			key, err := macaroon.NewAPIKey(secret)
			assert.NoError(t, err)
			key1, err := macaroon.NewAPIKey(secret)
			assert.NoError(t, err)
			key2, err := macaroon.NewAPIKey(secret)
			assert.NoError(t, err)

			keyInfo := console.APIKeyInfo{
				Name:      "visibleKey1",
				ProjectID: pr.ID,
				Secret:    secret,
			}
			keyInfo1 := console.APIKeyInfo{
				Name:      "visibleKey2",
				ProjectID: pr.ID,
				Secret:    secret,
			}
			ignoredPrefix := "notVisibleKey"
			keyInfo2 := console.APIKeyInfo{
				Name:      ignoredPrefix + "123",
				ProjectID: pr.ID,
				Secret:    secret,
			}

			createdKey, err := apikeys.Create(ctx, key.Head(), keyInfo)
			assert.NoError(t, err)
			assert.NotNil(t, createdKey)
			createdKey1, err := apikeys.Create(ctx, key1.Head(), keyInfo1)
			assert.NoError(t, err)
			assert.NotNil(t, createdKey1)
			createdKey2, err := apikeys.Create(ctx, key2.Head(), keyInfo2)
			assert.NoError(t, err)
			assert.NotNil(t, createdKey2)

			cursor := console.APIKeyCursor{Page: 1, Limit: 10}
			keys, err := apikeys.GetPagedByProjectID(ctx, pr.ID, cursor, ignoredPrefix)
			assert.NoError(t, err)
			assert.NotNil(t, keys)
			assert.Equal(t, uint64(2), keys.TotalCount)
			assert.Equal(t, 2, len(keys.APIKeys))
			assert.Equal(t, keyInfo.Name, keys.APIKeys[0].Name)
			assert.Equal(t, keyInfo1.Name, keys.APIKeys[1].Name)

			cursor.Search = ignoredPrefix
			keys, err = apikeys.GetPagedByProjectID(ctx, pr.ID, cursor, ignoredPrefix)
			assert.NoError(t, err)
			assert.NotNil(t, keys)
			assert.Equal(t, uint64(0), keys.TotalCount)
			assert.Equal(t, 0, len(keys.APIKeys))
		})

		t.Run("DeleteExpiredByNamePrefix", func(t *testing.T) {
			pr, err := projects.Insert(ctx, &console.Project{
				Name: "ProjectName3",
			})
			assert.NotNil(t, pr)
			assert.NoError(t, err)

			secret, err := macaroon.NewSecret()
			assert.NoError(t, err)

			key, err := macaroon.NewAPIKey(secret)
			assert.NoError(t, err)
			key1, err := macaroon.NewAPIKey(secret)
			assert.NoError(t, err)
			key2, err := macaroon.NewAPIKey(secret)
			assert.NoError(t, err)
			key3, err := macaroon.NewAPIKey(secret)
			assert.NoError(t, err)

			prefix := "prefix"
			now := time.Now()

			keyInfo := console.APIKeyInfo{
				Name:      "randomName",
				ProjectID: pr.ID,
				Secret:    secret,
			}
			keyInfo1 := console.APIKeyInfo{
				Name:      prefix,
				ProjectID: pr.ID,
				Secret:    secret,
			}
			keyInfo2 := console.APIKeyInfo{
				Name:      prefix + "test",
				ProjectID: pr.ID,
				Secret:    secret,
			}
			keyInfo3 := console.APIKeyInfo{
				Name:      prefix + "test1",
				ProjectID: pr.ID,
				Secret:    secret,
			}

			createdKey, err := apikeys.Create(ctx, key.Head(), keyInfo)
			assert.NoError(t, err)
			assert.NotNil(t, createdKey)
			createdKey1, err := apikeys.Create(ctx, key1.Head(), keyInfo1)
			assert.NoError(t, err)
			assert.NotNil(t, createdKey1)
			createdKey2, err := apikeys.Create(ctx, key2.Head(), keyInfo2)
			assert.NoError(t, err)
			assert.NotNil(t, createdKey2)
			createdKey3, err := apikeys.Create(ctx, key3.Head(), keyInfo3)
			assert.NoError(t, err)
			assert.NotNil(t, createdKey3)

			query := db.Testing().Rebind("UPDATE api_keys SET created_at = ? WHERE id = ?")

			_, err = db.Testing().RawDB().ExecContext(ctx, query, now.Add(-24*time.Hour), createdKey1.ID)
			assert.NoError(t, err)
			_, err = db.Testing().RawDB().ExecContext(ctx, query, now.Add(-24*2*time.Hour), createdKey2.ID)
			assert.NoError(t, err)
			_, err = db.Testing().RawDB().ExecContext(ctx, query, now.Add(-24*3*time.Hour), createdKey3.ID)
			assert.NoError(t, err)

			cursor := console.APIKeyCursor{Page: 1, Limit: 10}
			keys, err := apikeys.GetPagedByProjectID(ctx, pr.ID, cursor, "")
			assert.NoError(t, err)
			assert.NotNil(t, keys)
			assert.Len(t, keys.APIKeys, 4)

			// Even with a page size set to 1, 2 of the 4 keys must be deleted.
			err = apikeys.DeleteExpiredByNamePrefix(ctx, time.Hour*47, prefix, 0, 1)
			assert.NoError(t, err)

			keys, err = apikeys.GetPagedByProjectID(ctx, pr.ID, cursor, "")
			assert.NoError(t, err)
			assert.NotNil(t, keys)
			assert.Len(t, keys.APIKeys, 2)

			// 1 of the 2 remaining keys has to be deleted because the only one doesn't have a prefix.
			err = apikeys.DeleteExpiredByNamePrefix(ctx, time.Hour*23, prefix, 0, 1)
			assert.NoError(t, err)

			keys, err = apikeys.GetPagedByProjectID(ctx, pr.ID, cursor, "")
			assert.NoError(t, err)
			assert.NotNil(t, keys)
			assert.Len(t, keys.APIKeys, 1)
			assert.Equal(t, keyInfo.Name, keys.APIKeys[0].Name)
		})

		t.Run("DeleteMultiple", func(t *testing.T) {
			pr, err := projects.Insert(ctx, &console.Project{
				Name: "ProjectName3",
			})
			require.NoError(t, err)
			require.NotNil(t, pr)

			secret, err := macaroon.NewSecret()
			require.NoError(t, err)

			key0, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)
			key1, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)
			key2, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)

			keyInfo0 := console.APIKeyInfo{Name: "key0", ProjectID: pr.ID, Secret: secret}
			keyInfo1 := console.APIKeyInfo{Name: "key1", ProjectID: pr.ID, Secret: secret}
			keyInfo2 := console.APIKeyInfo{Name: "key2", ProjectID: pr.ID, Secret: secret}

			createdKey0, err := apikeys.Create(ctx, key0.Head(), keyInfo0)
			require.NoError(t, err)
			require.NotNil(t, createdKey0)
			createdKey1, err := apikeys.Create(ctx, key1.Head(), keyInfo1)
			require.NoError(t, err)
			require.NotNil(t, createdKey1)
			createdKey2, err := apikeys.Create(ctx, key2.Head(), keyInfo2)
			require.NoError(t, err)
			require.NotNil(t, createdKey2)

			err = apikeys.DeleteMultiple(ctx, []uuid.UUID{createdKey0.ID, createdKey2.ID})
			require.NoError(t, err)

			keys, err := apikeys.GetAllNamesByProjectID(ctx, pr.ID)
			require.NoError(t, err)
			require.Equal(t, []string{"key1"}, keys)
		})

		t.Run("CreatorEmail visibility and search", func(t *testing.T) {
			memberEmail := "member@example.com"
			memberUser, err := users.Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				Email:        memberEmail,
				PasswordHash: []byte("password"),
			})
			require.NoError(t, err)

			member, err := pm.Insert(ctx, memberUser.ID, project.ID, console.RoleMember)
			require.NoError(t, err)

			secret, err := macaroon.NewSecret()
			require.NoError(t, err)
			key, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)

			createdKey, err := apikeys.Create(ctx, key.Head(), console.APIKeyInfo{
				Name:      "keyForMember",
				ProjectID: project.ID,
				Secret:    secret,
				CreatedBy: memberUser.ID,
			})
			require.NoError(t, err)

			// member‐search — the key be returned with CreatorEmail set to member's email.
			cursor := console.APIKeyCursor{Page: 1, Limit: 10, Search: memberEmail}
			page, err := apikeys.GetPagedByProjectID(ctx, project.ID, cursor, "")
			require.NoError(t, err)
			require.NotNil(t, page)
			require.NotNil(t, page.APIKeys)
			require.Len(t, page.APIKeys, 1)
			require.Equal(t, memberEmail, page.APIKeys[0].CreatorEmail)

			err = pm.Delete(ctx, member.MemberID, project.ID)
			require.NoError(t, err)

			// blank‐search — the key should still be returned, but CreatorEmail must now be empty
			cursor.Search = ""
			page, err = apikeys.GetPagedByProjectID(ctx, project.ID, cursor, "")
			require.NoError(t, err)
			require.NotNil(t, page)
			require.NotNil(t, page.APIKeys)

			var seen bool
			for _, ak := range page.APIKeys {
				if ak.ID == createdKey.ID {
					seen = true
					require.Equal(t, "", ak.CreatorEmail, "email of ex-member must be hidden")
				}
			}
			require.True(t, seen, "expected to find keyForMember in the results")
		})
	})
}
