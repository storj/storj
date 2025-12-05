// Copyright (C) 2024 Storj Labs, Inc.
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
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGetUnappliedByProjectIDs(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		start := time.Now()
		end := start.AddDate(0, 1, 0)

		projectIDs := []uuid.UUID{}

		for i := 0; i < 5; i++ {
			projectIDs = append(projectIDs, testrand.UUID())
		}

		idsToRequest := []uuid.UUID{projectIDs[0], projectIDs[1], projectIDs[3]}

		// create 3 entries in table. We will request 3:
		// 2 that are there, one that is not. There is an extra record
		// in the table which we are not requesting, to verify it is indeed
		// not returned.
		createRecords := []stripe.CreateProjectRecord{
			{
				ProjectID: idsToRequest[0],
				Storage:   12345,
				Egress:    54321,
				Segments:  1,
			},
			{
				ProjectID: idsToRequest[1],
				Storage:   67890,
				Egress:    98760,
				Segments:  2,
			},
			{
				ProjectID: projectIDs[2],
				Storage:   41276,
				Egress:    91648,
				Segments:  3,
			},
		}
		require.NoError(t, db.StripeCoinPayments().ProjectRecords().Create(ctx, createRecords, start, end))

		records, err := db.StripeCoinPayments().ProjectRecords().GetUnappliedByProjectIDs(ctx, idsToRequest, start, end)
		require.NoError(t, err)
		require.Len(t, records, 2)
		var found1, found2 bool
		// consume one and rerun GetUnappliedByProjectIDs to test that applied records are not returned.
		var toConsume uuid.UUID
		for _, r := range records {
			if r.ProjectID == idsToRequest[0] {
				found1 = true
				toConsume = r.ID
			} else if r.ProjectID == idsToRequest[1] {
				found2 = true
			}
		}
		require.True(t, found1)
		require.True(t, found2)

		require.NoError(t, db.StripeCoinPayments().ProjectRecords().Consume(ctx, toConsume))

		records, err = db.StripeCoinPayments().ProjectRecords().GetUnappliedByProjectIDs(ctx, idsToRequest, start, end)
		require.NoError(t, err)
		require.Len(t, records, 1)
		require.Equal(t, idsToRequest[1], records[0].ProjectID)
	})
}
