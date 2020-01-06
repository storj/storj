// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectRecords(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		utc := time.Now().UTC()

		prjID, err := uuid.New()
		require.NoError(t, err)

		start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

		projectRecordsDB := db.StripeCoinPayments().ProjectRecords()

		t.Run("create", func(t *testing.T) {
			err = projectRecordsDB.Create(ctx,
				[]stripecoinpayments.CreateProjectRecord{
					{
						ProjectID: *prjID,
						Storage:   1,
						Egress:    2,
						Objects:   3,
					},
				},
				start, end,
			)
			require.NoError(t, err)
		})

		t.Run("check", func(t *testing.T) {
			err = projectRecordsDB.Check(ctx, *prjID, start, end)
			require.Error(t, err)
			assert.Equal(t, stripecoinpayments.ErrProjectRecordExists, err)
		})

		page, err := projectRecordsDB.ListUnapplied(ctx, 0, 1, time.Now())
		require.NoError(t, err)
		require.Equal(t, 1, len(page.Records))

		t.Run("consume", func(t *testing.T) {
			err = projectRecordsDB.Consume(ctx, page.Records[0].ID)
			require.NoError(t, err)
		})

		page, err = projectRecordsDB.ListUnapplied(ctx, 0, 1, time.Now())
		require.NoError(t, err)
		require.Equal(t, 0, len(page.Records))
	})
}

func TestProjectRecordsList(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		utc := time.Now().UTC()

		start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

		projectRecordsDB := db.StripeCoinPayments().ProjectRecords()

		const recordsLen = 5

		var createProjectRecords []stripecoinpayments.CreateProjectRecord
		for i := 0; i < recordsLen; i++ {
			projID, err := uuid.New()
			require.NoError(t, err)

			createProjectRecords = append(createProjectRecords,
				stripecoinpayments.CreateProjectRecord{
					ProjectID: *projID,
					Storage:   float64(i) + 1,
					Egress:    int64(i) + 2,
					Objects:   int64(i) + 3,
				},
			)
		}

		err := projectRecordsDB.Create(ctx, createProjectRecords, start, end)
		require.NoError(t, err)

		page, err := projectRecordsDB.ListUnapplied(ctx, 0, recordsLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, recordsLen, len(page.Records))

		assert.False(t, page.Next)
		assert.Equal(t, int64(0), page.NextOffset)

		for _, record := range page.Records {
			for _, createRecord := range createProjectRecords {
				if record.ProjectID != createRecord.ProjectID {
					continue
				}

				assert.NotNil(t, record.ID)
				assert.Equal(t, 16, len(record.ID))
				assert.Equal(t, createRecord.ProjectID, record.ProjectID)
				assert.Equal(t, createRecord.Storage, record.Storage)
				assert.Equal(t, createRecord.Egress, record.Egress)
				assert.Equal(t, createRecord.Objects, record.Objects)
				assert.Equal(t, start, record.PeriodStart.UTC())
				assert.Equal(t, end, record.PeriodEnd.UTC())
			}
		}
	})
}
