// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/uplink/private/eestream"
)

func TestFailingAudit(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	f, err := eestream.NewFEC(required, total)
	require.NoError(t, err)

	shares := make([]eestream.Share, total)
	output := func(s eestream.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// the data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	require.NoError(t, err)

	modifiedShares := make([]eestream.Share, len(shares))
	for i := range shares {
		modifiedShares[i] = shares[i].DeepCopy()
	}

	modifiedShares[0].Data[1] = '!'
	modifiedShares[2].Data[0] = '#'
	modifiedShares[3].Data[1] = '!'
	modifiedShares[4].Data[0] = 'b'

	badPieceNums := []int{0, 2, 3, 4}

	ctx := t.Context()
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

	f, err := eestream.NewFEC(required, total)
	require.NoError(t, err)

	shares := make([]eestream.Share, total)
	output := func(s eestream.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// the data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	require.NoError(t, err)

	ctx := t.Context()
	auditPkgShares := make(map[int]Share, len(shares))
	for i := range shares {
		auditPkgShares[shares[i].Number] = Share{
			PieceNum: shares[i].Number,
			Data:     append([]byte(nil), shares[i].Data...),
		}
	}
	_, _, err = auditShares(ctx, 20, 40, auditPkgShares)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "must specify at least the number of required shares")
}

func TestCreatePendingAudits(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	f, err := eestream.NewFEC(required, total)
	require.NoError(t, err)

	shares := make([]eestream.Share, total)
	output := func(s eestream.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// The data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	require.NoError(t, err)

	testNodeID := testrand.NodeID()

	ctx := t.Context()
	const pieceNum = 1
	contained := make(map[int]storj.NodeID)
	contained[pieceNum] = testNodeID

	segment := testSegment()
	segmentInfo := metabase.Segment{
		StreamID:    segment.StreamID,
		Position:    segment.Position,
		RootPieceID: testrand.PieceID(),
		Redundancy: storj.RedundancyScheme{
			Algorithm:      storj.ReedSolomon,
			RequiredShares: required,
			TotalShares:    total,
			ShareSize:      int32(len(shares[0].Data)),
		},
	}

	pending, err := createPendingAudits(ctx, contained, segment)
	require.NoError(t, err)
	require.Equal(t, 1, len(pending))
	assert.Equal(t, testNodeID, pending[0].Locator.NodeID)
	assert.Equal(t, segmentInfo.StreamID, pending[0].Locator.StreamID)
	assert.Equal(t, segmentInfo.Position, pending[0].Locator.Position)
	assert.Equal(t, pieceNum, pending[0].Locator.PieceNum)
	assert.EqualValues(t, 0, pending[0].ReverifyCount)
}

func testSegment() Segment {
	return Segment{
		StreamID: testrand.UUID(),
		Position: metabase.SegmentPosition{
			Index: uint32(testrand.Intn(100)),
		},
	}
}
