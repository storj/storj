// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package valueattribution_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/satellite/valueattribution"
)

func TestValueAttribution(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		valAttrdb := db.ValueAttribution()

		// unique partner and bucket
		partnerInfo := &valueattribution.PartnerInfo{
			PartnerID:  []byte("partnerID"),
			BucketName: []byte("bucketname"),
		}

		// same partner and dfferent bucket
		partnerInfo1 := &valueattribution.PartnerInfo{
			PartnerID:  []byte("partnerID"),
			BucketName: []byte("different bucketname"),
		}

		// different partner and existing bucket
		partnerInfo2 := &valueattribution.PartnerInfo{
			PartnerID:  []byte("different partnerID"),
			BucketName: []byte("different bucketname"),
		}

		{ // Insert
			_, err := valAttrdb.Insert(ctx, partnerInfo)
			assert.NoError(t, err)

			_, err = valAttrdb.Insert(ctx, partnerInfo1)
			assert.NoError(t, err)

			_, err = valAttrdb.Insert(ctx, partnerInfo2)
			assert.Error(t, err)
		}

		{ // Get
			info, err := valAttrdb.Get(ctx, partnerInfo.BucketName)
			assert.NoError(t, err)
			assert.Equal(t, partnerInfo.PartnerID, info.PartnerID)

			info, err = valAttrdb.Get(ctx, partnerInfo1.BucketName)
			assert.NoError(t, err)
			assert.Equal(t, partnerInfo1.PartnerID, info.PartnerID)
		}
	})
}
