// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"testing"
	"time"

	"github.com/zeebo/assert"

	"storj.io/common/pb"
	"storj.io/common/testrand"
	"storj.io/storj/shared/bloomfilter"
)

func TestBloomFilterManager(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	now := time.Now()
	sat := testrand.NodeID()
	includedPiece := testrand.PieceID()
	excludedPiece := testrand.PieceID()

	// make a bloom filter manager and get the trash callback once
	bfm, err := NewBloomFilterManager(dir, 0)
	assert.NoError(t, err)
	fn := bfm.GetBloomFilter(sat)

	// ensure the trash callback doesn't trash anything
	assert.False(t, fn(ctx, includedPiece, now))
	assert.False(t, fn(ctx, excludedPiece, now))
	assert.True(t, bfm.GetCreatedTime(sat).IsZero())

	// create a filter that includes the included piece and excludes the excluded piece
	var filter *bloomfilter.Filter
	for filter == nil || filter.Contains(excludedPiece) {
		filter = bloomfilter.NewOptimal(10, 0.01)
		filter.Add(includedPiece)
	}

	// update the filter for the satellite
	assert.NoError(t, bfm.Queue(ctx, sat, &pb.RetainRequest{
		CreationDate: now,
		Filter:       filter.Bytes(),
	}))

	// ensure the trash callback trashes the excluded piece
	assert.False(t, fn(ctx, includedPiece, now.Add(-time.Second)))
	assert.True(t, fn(ctx, excludedPiece, now.Add(-time.Second)))
	assert.True(t, bfm.GetCreatedTime(sat).Equal(now))

	// reopen the bloom filter manager and ensure the filter is still there
	bfm, err = NewBloomFilterManager(dir, 0)
	assert.NoError(t, err)
	fn = bfm.GetBloomFilter(sat)

	// ensure the trash callback still trashes the excluded piece
	assert.False(t, fn(ctx, includedPiece, now.Add(-time.Second)))
	assert.True(t, fn(ctx, excludedPiece, now.Add(-time.Second)))
	assert.True(t, bfm.GetCreatedTime(sat).Equal(now))

	// setting an invalid filter fails
	assert.Error(t, bfm.Queue(ctx, sat, &pb.RetainRequest{
		CreationDate:  now,
		Filter:        filter.Bytes(),
		HashAlgorithm: pb.PieceHashAlgorithm_BLAKE3,
		Hash:          []byte("lol not the right hash for sure"),
	}))

	// setting an older filter fails
	filter.Add(excludedPiece)
	assert.Error(t, bfm.Queue(ctx, sat, &pb.RetainRequest{
		CreationDate: now.Add(-time.Second),
		Filter:       filter.Bytes(),
	}))

	// it should still trash the excluded piece
	assert.False(t, fn(ctx, includedPiece, now.Add(-time.Second)))
	assert.True(t, fn(ctx, excludedPiece, now.Add(-time.Second)))
	assert.True(t, bfm.GetCreatedTime(sat).Equal(now))
}
