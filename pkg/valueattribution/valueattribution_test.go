// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package valueattribution_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/valueattribution"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestValueAttribution(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		valAttrdb := db.ValueAttribution()

		// Create and insert test segment infos into DB
		partnerInfo := &valueattribution.PartnerInfo{
			PartnerID:  []byte("valueattribution testcase partnerID"),
			BucketName: []byte("valueattribution testcase bucketname"),
		}

		{ // Insert
			_, err := valAttrdb.Insert(ctx, partnerInfo)
			assert.NoError(t, err)
		}

		{ // GetByBucketName
			info, err := valAttrdb.GetByBucketName(ctx, partnerInfo.BucketName)
			assert.NoError(t, err)
			assert.Equal(t, partnerInfo.PartnerID, info.PartnerID)
		}
	})
}
