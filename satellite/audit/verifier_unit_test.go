// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vivint/infectious"

	"storj.io/common/pb"
	"storj.io/common/pkcrypto"
	"storj.io/common/storj"
	"storj.io/common/testrand"
)

func TestFailingAudit(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	f, err := infectious.NewFEC(required, total)
	require.NoError(t, err)

	shares := make([]infectious.Share, total)
	output := func(s infectious.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// the data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	require.NoError(t, err)

	modifiedShares := make([]infectious.Share, len(shares))
	for i := range shares {
		modifiedShares[i] = shares[i].DeepCopy()
	}

	modifiedShares[0].Data[1] = '!'
	modifiedShares[2].Data[0] = '#'
	modifiedShares[3].Data[1] = '!'
	modifiedShares[4].Data[0] = 'b'

	badPieceNums := []int{0, 2, 3, 4}

	ctx := context.Background()
	auditPkgShares := make(map[int]Share, len(modifiedShares))
	for i := range modifiedShares {
		auditPkgShares[modifiedShares[i].Number] = Share{
			PieceNum: modifiedShares[i].Number,
			Data:     append([]byte(nil), modifiedShares[i].Data...),
		}
	}

	pieceNums, correctedShares, err := auditShares(ctx, 8, 14, auditPkgShares)
	if err != nil {
		panic(err)
	}

	for i, num := range pieceNums {
		if num != badPieceNums[i] {
			t.Fatal("expected nums in pieceNums to be same as in badPieceNums")
		}
	}

	require.Equal(t, shares, correctedShares)
}

func TestNotEnoughShares(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	f, err := infectious.NewFEC(required, total)
	require.NoError(t, err)

	shares := make([]infectious.Share, total)
	output := func(s infectious.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// the data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	require.NoError(t, err)

	ctx := context.Background()
	auditPkgShares := make(map[int]Share, len(shares))
	for i := range shares {
		auditPkgShares[shares[i].Number] = Share{
			PieceNum: shares[i].Number,
			Data:     append([]byte(nil), shares[i].Data...),
		}
	}
	_, _, err = auditShares(ctx, 20, 40, auditPkgShares)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "infectious: must specify at least the number of required shares")
}

func TestCreatePendingAudits(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	f, err := infectious.NewFEC(required, total)
	require.NoError(t, err)

	shares := make([]infectious.Share, total)
	output := func(s infectious.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// The data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	require.NoError(t, err)

	testNodeID := testrand.NodeID()

	ctx := context.Background()
	contained := make(map[int]storj.NodeID)
	contained[1] = testNodeID

	pointer := &pb.Pointer{
		CreationDate: time.Now(),
		Type:         pb.Pointer_REMOTE,
		Remote: &pb.RemoteSegment{
			RootPieceId: storj.NewPieceID(),
			Redundancy: &pb.RedundancyScheme{
				MinReq:           8,
				Total:            14,
				ErasureShareSize: int32(len(shares[0].Data)),
			},
		},
	}

	randomIndex := rand.Int63n(10)

	pending, err := createPendingAudits(ctx, contained, shares, pointer, randomIndex, "")
	require.NoError(t, err)
	require.Equal(t, 1, len(pending))
	assert.Equal(t, testNodeID, pending[0].NodeID)
	assert.Equal(t, pointer.Remote.RootPieceId, pending[0].PieceID)
	assert.Equal(t, randomIndex, pending[0].StripeIndex)
	assert.Equal(t, pointer.Remote.Redundancy.ErasureShareSize, pending[0].ShareSize)
	assert.Equal(t, pkcrypto.SHA256Hash(shares[1].Data), pending[0].ExpectedShareHash)
	assert.EqualValues(t, 0, pending[0].ReverifyCount)
}
