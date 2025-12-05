// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/bucketmigrations"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestBucketMigrations(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		t.Run("Create", func(t *testing.T) {
			projectID := testrand.UUID()
			bucketName := testrand.BucketName()

			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
			require.NoError(t, err)

			migrationDB := db.BucketMigrations()

			migration := bucketmigrations.Migration{
				ID:             testrand.UUID(),
				ProjectID:      projectID,
				BucketName:     bucketName,
				FromPlacement:  1,
				ToPlacement:    2,
				MigrationType:  bucketmigrations.MigrationTypeTrivial,
				State:          bucketmigrations.StatePending,
				BytesProcessed: 0,
			}

			created, err := migrationDB.Create(ctx, migration)
			require.NoError(t, err)
			require.Equal(t, migration.ID, created.ID)
			require.Equal(t, migration.ProjectID, created.ProjectID)
			require.Equal(t, migration.BucketName, created.BucketName)
			require.Equal(t, migration.FromPlacement, created.FromPlacement)
			require.Equal(t, migration.ToPlacement, created.ToPlacement)
			require.Equal(t, migration.MigrationType, created.MigrationType)
			require.Equal(t, migration.State, created.State)
			require.Equal(t, migration.BytesProcessed, created.BytesProcessed)
			require.NotZero(t, created.CreatedAt)
			require.NotZero(t, created.UpdatedAt)
			require.Nil(t, created.ErrorMessage)
			require.Nil(t, created.CompletedAt)
		})

		t.Run("Get", func(t *testing.T) {
			projectID := testrand.UUID()
			bucketName := testrand.BucketName()

			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
			require.NoError(t, err)

			migrationDB := db.BucketMigrations()

			migrationID := testrand.UUID()
			migration := bucketmigrations.Migration{
				ID:            migrationID,
				ProjectID:     projectID,
				BucketName:    bucketName,
				FromPlacement: 1,
				ToPlacement:   2,
				MigrationType: bucketmigrations.MigrationTypeFull,
				State:         bucketmigrations.StatePending,
			}

			_, err = migrationDB.Create(ctx, migration)
			require.NoError(t, err)

			found, err := migrationDB.Get(ctx, migrationID)
			require.NoError(t, err)
			require.Equal(t, migrationID, found.ID)
			require.Equal(t, projectID, found.ProjectID)
			require.Equal(t, bucketName, found.BucketName)
			require.Equal(t, bucketmigrations.StatePending, found.State)

			_, err = migrationDB.Get(ctx, testrand.UUID())
			require.True(t, bucketmigrations.ErrMigrationNotFound.Has(err))
		})

		t.Run("Update", func(t *testing.T) {
			projectID := testrand.UUID()
			bucketName := testrand.BucketName()

			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
			require.NoError(t, err)

			migrationDB := db.BucketMigrations()

			migrationID := testrand.UUID()
			migration := bucketmigrations.Migration{
				ID:            migrationID,
				ProjectID:     projectID,
				BucketName:    bucketName,
				FromPlacement: 1,
				ToPlacement:   2,
				MigrationType: bucketmigrations.MigrationTypeTrivial,
				State:         bucketmigrations.StatePending,
			}

			_, err = migrationDB.Create(ctx, migration)
			require.NoError(t, err)

			newState := bucketmigrations.StateInProgress
			err = migrationDB.Update(ctx, migrationID, bucketmigrations.UpdateFields{
				State: &newState,
			})
			require.NoError(t, err)

			updated, err := migrationDB.Get(ctx, migrationID)
			require.NoError(t, err)
			require.Equal(t, bucketmigrations.StateInProgress, updated.State)
			require.Zero(t, updated.BytesProcessed)

			bytesProcessed := uint64(1024000)
			err = migrationDB.Update(ctx, migrationID, bucketmigrations.UpdateFields{
				BytesProcessed: &bytesProcessed,
			})
			require.NoError(t, err)

			updated, err = migrationDB.Get(ctx, migrationID)
			require.NoError(t, err)
			require.Equal(t, bytesProcessed, updated.BytesProcessed)

			errorMsg := "migration failed due to network error"
			completedState := bucketmigrations.StateFailed
			completedAt := time.Now()
			err = migrationDB.Update(ctx, migrationID, bucketmigrations.UpdateFields{
				State:        &completedState,
				ErrorMessage: &errorMsg,
				CompletedAt:  &completedAt,
			})
			require.NoError(t, err)

			updated, err = migrationDB.Get(ctx, migrationID)
			require.NoError(t, err)
			require.Equal(t, bucketmigrations.StateFailed, updated.State)
			require.NotNil(t, updated.ErrorMessage)
			require.Equal(t, errorMsg, *updated.ErrorMessage)
			require.NotNil(t, updated.CompletedAt)

			err = migrationDB.Delete(ctx, migrationID)
			require.NoError(t, err)
		})

		t.Run("StateTransitions", func(t *testing.T) {
			projectID := testrand.UUID()
			bucketName := testrand.BucketName()

			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
			require.NoError(t, err)

			migrationDB := db.BucketMigrations()

			migrationID := testrand.UUID()
			migration := bucketmigrations.Migration{
				ID:            migrationID,
				ProjectID:     projectID,
				BucketName:    bucketName,
				FromPlacement: 1,
				ToPlacement:   2,
				MigrationType: bucketmigrations.MigrationTypeTrivial,
				State:         bucketmigrations.StatePending,
			}

			_, err = migrationDB.Create(ctx, migration)
			require.NoError(t, err)

			transitions := []struct {
				name     string
				newState bucketmigrations.State
			}{
				{"pending to in_progress", bucketmigrations.StateInProgress},
				{"in_progress to completed", bucketmigrations.StateCompleted},
			}

			for _, tr := range transitions {
				t.Run(tr.name, func(t *testing.T) {
					err = migrationDB.Update(ctx, migrationID, bucketmigrations.UpdateFields{
						State: &tr.newState,
					})
					require.NoError(t, err)

					updated, err := migrationDB.Get(ctx, migrationID)
					require.NoError(t, err)
					require.Equal(t, tr.newState, updated.State)
				})
			}
		})

		t.Run("Delete", func(t *testing.T) {
			projectID := testrand.UUID()
			bucketName := testrand.BucketName()

			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
			require.NoError(t, err)

			migrationDB := db.BucketMigrations()

			migrationID := testrand.UUID()
			migration := bucketmigrations.Migration{
				ID:            migrationID,
				ProjectID:     projectID,
				BucketName:    bucketName,
				FromPlacement: 1,
				ToPlacement:   2,
				MigrationType: bucketmigrations.MigrationTypeTrivial,
				State:         bucketmigrations.StatePending,
			}

			_, err = migrationDB.Create(ctx, migration)
			require.NoError(t, err)

			_, err = migrationDB.Get(ctx, migrationID)
			require.NoError(t, err)

			err = migrationDB.Delete(ctx, migrationID)
			require.NoError(t, err)

			_, err = migrationDB.Get(ctx, migrationID)
			require.True(t, bucketmigrations.ErrMigrationNotFound.Has(err))
		})

		t.Run("ListByBucket", func(t *testing.T) {
			projectID := testrand.UUID()
			bucketName := testrand.BucketName()

			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
			require.NoError(t, err)

			migrationDB := db.BucketMigrations()

			// Create multiple migrations for the same bucket.
			for i := 0; i < 3; i++ {
				migrationID := testrand.UUID()

				migration := bucketmigrations.Migration{
					ID:            migrationID,
					ProjectID:     projectID,
					BucketName:    bucketName,
					FromPlacement: 1 + i,
					ToPlacement:   2 + i,
					MigrationType: bucketmigrations.MigrationTypeTrivial,
					State:         bucketmigrations.StatePending,
				}

				_, err = migrationDB.Create(ctx, migration)
				require.NoError(t, err)
			}

			migrations, err := migrationDB.ListByBucket(ctx, projectID, bucketName)
			require.NoError(t, err)
			require.Len(t, migrations, 3)

			// Verify they are ordered by created_at descending (most recent first).
			for i := 0; i < len(migrations)-1; i++ {
				require.True(t, migrations[i].CreatedAt.After(migrations[i+1].CreatedAt) ||
					migrations[i].CreatedAt.Equal(migrations[i+1].CreatedAt))
			}
		})

		t.Run("ListByState", func(t *testing.T) {
			projectID := testrand.UUID()

			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
			require.NoError(t, err)

			migrationDB := db.BucketMigrations()

			states := []bucketmigrations.State{
				bucketmigrations.StateFailed,
				bucketmigrations.StateFailed,
			}

			for i, state := range states {
				bucketName := testrand.BucketName()

				migration := bucketmigrations.Migration{
					ID:            testrand.UUID(),
					ProjectID:     projectID,
					BucketName:    bucketName,
					FromPlacement: 1 + i,
					ToPlacement:   2 + i,
					MigrationType: bucketmigrations.MigrationTypeTrivial,
					State:         state,
				}

				_, err = migrationDB.Create(ctx, migration)
				require.NoError(t, err)
			}

			pendingMigrations, err := migrationDB.ListByState(ctx, bucketmigrations.ListOptions{
				State:  bucketmigrations.StateFailed,
				Limit:  10,
				Offset: 0,
			})
			require.NoError(t, err)
			require.Len(t, pendingMigrations, 2)

			for i := 0; i < len(pendingMigrations)-1; i++ {
				require.True(t, pendingMigrations[i].CreatedAt.Before(pendingMigrations[i+1].CreatedAt) ||
					pendingMigrations[i].CreatedAt.Equal(pendingMigrations[i+1].CreatedAt))
			}

			// Test pagination.
			pendingPage1, err := migrationDB.ListByState(ctx, bucketmigrations.ListOptions{
				State:  bucketmigrations.StateFailed,
				Limit:  1,
				Offset: 0,
			})
			require.NoError(t, err)
			require.Len(t, pendingPage1, 1)

			pendingPage2, err := migrationDB.ListByState(ctx, bucketmigrations.ListOptions{
				State:  bucketmigrations.StateFailed,
				Limit:  1,
				Offset: 1,
			})
			require.NoError(t, err)
			require.Len(t, pendingPage2, 1)
			require.NotEqual(t, pendingPage1[0].ID, pendingPage2[0].ID)
		})
	})
}
