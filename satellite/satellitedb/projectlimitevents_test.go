// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectLimitEvents(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		t.Run("Insert", func(t *testing.T) {
			projectID := testrand.UUID()

			event, err := db.ProjectLimitEvents().Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, db.ProjectLimitEvents().UpdateEmailSent(ctx, []uuid.UUID{event.ID}, time.Now()))
			}()

			require.NotEqual(t, uuid.UUID{}, event.ID)
			require.Equal(t, projectID, event.ProjectID)
			require.Equal(t, accounting.StorageUsage80, event.Event)
			require.False(t, event.IsReset)
			require.Nil(t, event.EmailSent)
			require.Nil(t, event.LastAttempted)

			got, err := db.ProjectLimitEvents().GetByID(ctx, event.ID)
			require.NoError(t, err)
			require.Equal(t, event.ID, got.ID)
			require.Equal(t, event.ProjectID, got.ProjectID)
			require.Equal(t, event.Event, got.Event)
			require.Equal(t, event.IsReset, got.IsReset)
			require.Nil(t, got.EmailSent)
			require.Nil(t, got.LastAttempted)

			resetEvent, err := db.ProjectLimitEvents().Insert(ctx, projectID, accounting.StorageUsage80, true)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, db.ProjectLimitEvents().UpdateEmailSent(ctx, []uuid.UUID{resetEvent.ID}, time.Now()))
			}()
			require.True(t, resetEvent.IsReset)

			got, err = db.ProjectLimitEvents().GetByID(ctx, resetEvent.ID)
			require.NoError(t, err)
			require.True(t, got.IsReset)
		})

		t.Run("UpdateLastAttempted", func(t *testing.T) {
			projectID := testrand.UUID()

			e, err := db.ProjectLimitEvents().Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, db.ProjectLimitEvents().UpdateEmailSent(ctx, []uuid.UUID{e.ID}, time.Now()))
			}()

			got, err := db.ProjectLimitEvents().GetByID(ctx, e.ID)
			require.NoError(t, err)
			require.Nil(t, got.LastAttempted)

			require.NoError(t, db.ProjectLimitEvents().UpdateLastAttempted(ctx, []uuid.UUID{e.ID}, time.Now()))

			got, err = db.ProjectLimitEvents().GetByID(ctx, e.ID)
			require.NoError(t, err)
			require.NotNil(t, got.LastAttempted)
		})

		t.Run("GetNextBatch", func(t *testing.T) {
			t.Run("GroupsByProject", func(t *testing.T) {
				projectID := testrand.UUID()

				e1, err := db.ProjectLimitEvents().Insert(ctx, projectID, accounting.StorageUsage80, false)
				require.NoError(t, err)
				e2, err := db.ProjectLimitEvents().Insert(ctx, projectID, accounting.StorageUsage100, false)
				require.NoError(t, err)
				e3, err := db.ProjectLimitEvents().Insert(ctx, projectID, accounting.EgressUsage80, true)
				require.NoError(t, err)
				defer func() {
					require.NoError(t, db.ProjectLimitEvents().UpdateEmailSent(ctx, []uuid.UUID{e1.ID, e2.ID, e3.ID}, time.Now()))
				}()

				batch, err := db.ProjectLimitEvents().GetNextBatch(ctx, time.Now())
				require.NoError(t, err)
				require.Len(t, batch, 3)

				ids := make(map[uuid.UUID]struct{})
				for _, e := range batch {
					ids[e.ID] = struct{}{}
					require.Equal(t, projectID, e.ProjectID)
				}
				require.Contains(t, ids, e1.ID)
				require.Contains(t, ids, e2.ID)
				require.Contains(t, ids, e3.ID)
			})

			t.Run("ExcludesProcessed", func(t *testing.T) {
				projectID := testrand.UUID()

				e1, err := db.ProjectLimitEvents().Insert(ctx, projectID, accounting.StorageUsage80, false)
				require.NoError(t, err)
				e2, err := db.ProjectLimitEvents().Insert(ctx, projectID, accounting.EgressUsage80, false)
				require.NoError(t, err)
				defer func() {
					require.NoError(t, db.ProjectLimitEvents().UpdateEmailSent(ctx, []uuid.UUID{e2.ID}, time.Now()))
				}()

				require.NoError(t, db.ProjectLimitEvents().UpdateEmailSent(ctx, []uuid.UUID{e1.ID}, time.Now()))

				batch, err := db.ProjectLimitEvents().GetNextBatch(ctx, time.Now())
				require.NoError(t, err)
				require.Len(t, batch, 1)
				require.Equal(t, e2.ID, batch[0].ID)
			})

			t.Run("RespectsTimeBuffer", func(t *testing.T) {
				projectID := testrand.UUID()

				e, err := db.ProjectLimitEvents().Insert(ctx, projectID, accounting.StorageUsage80, false)
				require.NoError(t, err)
				defer func() {
					require.NoError(t, db.ProjectLimitEvents().UpdateEmailSent(ctx, []uuid.UUID{e.ID}, time.Now()))
				}()

				batch, err := db.ProjectLimitEvents().GetNextBatch(ctx, time.Now().Add(-time.Hour))
				require.NoError(t, err)
				require.Len(t, batch, 0)

				batch, err = db.ProjectLimitEvents().GetNextBatch(ctx, time.Now())
				require.NoError(t, err)
				require.Len(t, batch, 1)
				require.Equal(t, projectID, batch[0].ProjectID)
			})

			t.Run("SelectsOldestProject", func(t *testing.T) {
				projectID1 := testrand.UUID()
				projectID2 := testrand.UUID()

				e1, err := db.ProjectLimitEvents().Insert(ctx, projectID1, accounting.StorageUsage80, false)
				require.NoError(t, err)
				e2, err := db.ProjectLimitEvents().Insert(ctx, projectID2, accounting.StorageUsage80, false)
				require.NoError(t, err)
				defer func() {
					require.NoError(t, db.ProjectLimitEvents().UpdateEmailSent(ctx, []uuid.UUID{e1.ID, e2.ID}, time.Now()))
				}()

				// Give project1 a later last_attempted so project2 (null) is selected first.
				require.NoError(t, db.ProjectLimitEvents().UpdateLastAttempted(ctx, []uuid.UUID{e1.ID}, time.Now().Add(time.Hour)))

				batch, err := db.ProjectLimitEvents().GetNextBatch(ctx, time.Now())
				require.NoError(t, err)
				require.Len(t, batch, 1)
				require.Equal(t, projectID2, batch[0].ProjectID)
			})
		})
	})
}
