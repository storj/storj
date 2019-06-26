// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

const (
	numBuckets     = 5
	tallyIntervals = 10

	tallyInterval = time.Hour
)

func TestUsageRollups(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		now := time.Now()
		start := now.Add(tallyInterval * time.Duration(-tallyIntervals))

		project1 := testrand.UUID()
		project2 := testrand.UUID()

		p1base := binary.BigEndian.Uint64(project1[:8]) >> 48
		p2base := binary.BigEndian.Uint64(project2[:8]) >> 48

		getValue := func(i, j int, base uint64) int64 {
			a := uint64((i+1)*(j+1)) ^ base
			a &^= (1 << 63)
			return int64(a)
		}

		actions := []pb.PieceAction{
			pb.PieceAction_GET,
			pb.PieceAction_GET_AUDIT,
			pb.PieceAction_GET_REPAIR,
		}

		var buckets []string
		for i := 0; i < numBuckets; i++ {
			bucketName := fmt.Sprintf("bucket_%d", i)

			// project 1
			for _, action := range actions {
				value := getValue(0, i, p1base)

				err := db.Orders().UpdateBucketBandwidthAllocation(ctx,
					project1,
					[]byte(bucketName),
					action,
					value*6,
					now,
				)
				if err != nil {
					t.Fatal(err)
				}

				err = db.Orders().UpdateBucketBandwidthSettle(ctx,
					project1,
					[]byte(bucketName),
					action,
					value*3,
					now,
				)
				if err != nil {
					t.Fatal(err)
				}

				err = db.Orders().UpdateBucketBandwidthInline(ctx,
					project1,
					[]byte(bucketName),
					action,
					value,
					now,
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			// project 2
			for _, action := range actions {
				value := getValue(1, i, p2base)

				err := db.Orders().UpdateBucketBandwidthAllocation(ctx,
					project2,
					[]byte(bucketName),
					action,
					value*6,
					now,
				)
				if err != nil {
					t.Fatal(err)
				}

				err = db.Orders().UpdateBucketBandwidthSettle(ctx,
					project2,
					[]byte(bucketName),
					action,
					value*3,
					now,
				)
				if err != nil {
					t.Fatal(err)
				}

				err = db.Orders().UpdateBucketBandwidthInline(ctx,
					project2,
					[]byte(bucketName),
					action,
					value,
					now,
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			buckets = append(buckets, bucketName)
		}

		for i := 0; i < tallyIntervals; i++ {
			interval := start.Add(tallyInterval * time.Duration(i))

			bucketTallies := make(map[string]*accounting.BucketTally)
			for j, bucket := range buckets {
				bucketID1 := project1.String() + "/" + bucket
				bucketID2 := project2.String() + "/" + bucket
				value1 := getValue(i, j, p1base) * 10
				value2 := getValue(i, j, p2base) * 10

				tally1 := &accounting.BucketTally{
					BucketName:     []byte(bucket),
					ProjectID:      project1[:],
					Segments:       value1,
					InlineSegments: value1,
					RemoteSegments: value1,
					Files:          value1,
					InlineFiles:    value1,
					RemoteFiles:    value1,
					Bytes:          value1,
					InlineBytes:    value1,
					RemoteBytes:    value1,
					MetadataSize:   value1,
				}

				tally2 := &accounting.BucketTally{
					BucketName:     []byte(bucket),
					ProjectID:      project2[:],
					Segments:       value2,
					InlineSegments: value2,
					RemoteSegments: value2,
					Files:          value2,
					InlineFiles:    value2,
					RemoteFiles:    value2,
					Bytes:          value2,
					InlineBytes:    value2,
					RemoteBytes:    value2,
					MetadataSize:   value2,
				}

				bucketTallies[bucketID1] = tally1
				bucketTallies[bucketID2] = tally2
			}

			tallies, err := db.ProjectAccounting().SaveTallies(ctx, interval, bucketTallies)
			if err != nil {
				t.Fatal(err)
			}
			if len(tallies) != len(buckets)*2 {
				t.Fatal()
			}
		}

		usageRollups := db.Console().UsageRollups()

		t.Run("test project total", func(t *testing.T) {
			projTotal1, err := usageRollups.GetProjectTotal(ctx, project1, start, now)
			assert.NoError(t, err)
			assert.NotNil(t, projTotal1)

			projTotal2, err := usageRollups.GetProjectTotal(ctx, project2, start, now)
			assert.NoError(t, err)
			assert.NotNil(t, projTotal2)
		})

		t.Run("test bucket usage rollups", func(t *testing.T) {
			rollups1, err := usageRollups.GetBucketUsageRollups(ctx, project1, start, now)
			assert.NoError(t, err)
			assert.NotNil(t, rollups1)

			rollups2, err := usageRollups.GetBucketUsageRollups(ctx, project2, start, now)
			assert.NoError(t, err)
			assert.NotNil(t, rollups2)
		})

		t.Run("test bucket totals", func(t *testing.T) {
			cursor := console.BucketUsageCursor{
				Limit: 20,
				Page:  1,
			}

			totals1, err := usageRollups.GetBucketTotals(ctx, project1, cursor, start, now)
			assert.NoError(t, err)
			assert.NotNil(t, totals1)

			totals2, err := usageRollups.GetBucketTotals(ctx, project2, cursor, start, now)
			assert.NoError(t, err)
			assert.NotNil(t, totals2)
		})
	})
}
