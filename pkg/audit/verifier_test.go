// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package audit_test

import (
	"context"
	"crypto/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vivint/infectious"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type mockDownloader struct {
	shares map[int]audit.Share
}

func TestPassingAudit(t *testing.T) {
	ctx := context.Background()
	mockShares := make(map[int]audit.Share)

	for _, tt := range []struct {
		nodeAmt  int
		shareAmt int
		required int
		total    int
		err      error
	}{
		{nodeAmt: 30, shareAmt: 30, required: 20, total: 40, err: nil},
	} {
		someData := randData(32 * 1024)
		for i := 0; i < tt.shareAmt; i++ {
			mockShares[i] = audit.Share{
				Error:       tt.err,
				PieceNumber: i,
				Data:        someData,
			}
		}
		md := mockDownloader{shares: mockShares}
		verifier := &audit.Verifier{downloader: &md}
		pointer := makePointer(tt.nodeAmt)
		verifiedNodes, err := verifier.Verify(ctx, &audit.Stripe{Index: 6, Segment: pointer, PBA: nil, Authorization: nil})
		if err != nil {
			t.Fatal(err)
		}

		if len(verifiedNodes.SuccessNodeIDs) == 0 {
			t.Fatal("expected there to be passing nodes")
		}
	}
}

func TestSomeNodesPassAudit(t *testing.T) {
	ctx := context.Background()
	mockShares := make(map[int]share)

	for _, tt := range []struct {
		nodeAmt  int
		shareAmt int
		required int
		total    int
		err0     error
		err1     error
	}{
		{nodeAmt: 30, shareAmt: 30, required: 20, total: 40, err0: Error.New("unable to get node"), err1: nil},
	} {
		someData := randData(32 * 1024)
		for i := 0; i < 10; i++ {
			mockShares[i] = share{
				Error:       tt.err0,
				PieceNumber: i,
				Data:        someData,
			}
		}
		for i := 10; i < tt.shareAmt; i++ {
			mockShares[i] = share{
				Error:       tt.err1,
				PieceNumber: i,
				Data:        someData,
			}
		}

		md := mockDownloader{shares: mockShares}
		verifier := &audit.Verifier{downloader: &md}
		pointer := makePointer(tt.nodeAmt)
		verifiedNodes, err := verifier.verify(ctx, &audit.Stripe{Index: 6, Segment: pointer, PBA: nil, Authorization: nil})
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, len(verifiedNodes.OfflineNodeIDs), 10)
	}
}

func TestFailingAudit(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	f, err := infectious.NewFEC(required, total)
	if err != nil {
		panic(err)
	}

	shares := make([]infectious.Share, total)
	output := func(s infectious.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// the data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	if err != nil {
		panic(err)
	}

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
	auditPkgShares := make(map[int]audit.Share, len(modifiedShares))
	for i := range modifiedShares {
		auditPkgShares[modifiedShares[i].Number] = audit.Share{
			PieceNumber: modifiedShares[i].Number,
			Data:        append([]byte(nil), modifiedShares[i].Data...),
		}
	}
	pieceNums, err := auditShares(ctx, 8, 14, auditPkgShares)
	if err != nil {
		panic(err)
	}
	for i, num := range pieceNums {
		if num != badPieceNums[i] {
			t.Fatal("expected nums in pieceNums to be same as in badPieceNums")
		}
	}
}

func TestNotEnoughShares(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	f, err := infectious.NewFEC(required, total)
	if err != nil {
		panic(err)
	}

	shares := make([]infectious.Share, total)
	output := func(s infectious.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// the data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	auditPkgShares := make(map[int]share, len(shares))
	for i := range shares {
		auditPkgShares[shares[i].Number] = share{
			PieceNumber: shares[i].Number,
			Data:        append([]byte(nil), shares[i].Data...),
		}
	}
	_, err = auditShares(ctx, 20, 40, auditPkgShares)
	assert.Contains(t, err.Error(), "infectious: must specify at least the number of required shares")
}

func TestCalcPadded(t *testing.T) {
	for _, tt := range []struct {
		segSize    int64
		blockSize  int
		paddedSize int64
	}{
		{segSize: int64(5 * 1024), blockSize: 1024, paddedSize: int64(5120)},
		{segSize: int64(5 * 1023), blockSize: 1024, paddedSize: int64(5120)},
	} {
		result := calcPadded(tt.segSize, tt.blockSize)
		assert.Equal(t, result, tt.paddedSize)
	}
}

func (m *mockDownloader) DownloadShares(ctx context.Context, pointer *pb.Pointer, stripeIndex int,
	pba *pb.PayerBandwidthAllocation, authorization *pb.SignedMessage) (shares map[int]share, nodes map[int]storj.NodeID, err error) {

	nodes = make(map[int]*pb.Node, 30)

	for i := 0; i < 30; i++ {
		nodes[i] = teststorj.NodeIDFromString(strconv.Itoa(i))
	}
	return m.shares, nodes, nil
}

func makePointer(nodeAmt int) *pb.Pointer {
	var rps []*pb.RemotePiece
	for i := 0; i < nodeAmt; i++ {
		rps = append(rps, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   teststorj.NodeIDFromString("test" + strconv.Itoa(i)),
		})
	}
	pr := &pb.Pointer{
		Type: pb.Pointer_REMOTE,
		Remote: &pb.RemoteSegment{
			Redundancy: &pb.RedundancyScheme{
				Type:             pb.RedundancyScheme_RS,
				MinReq:           20,
				Total:            40,
				RepairThreshold:  2,
				SuccessThreshold: 3,
				ErasureShareSize: 4,
			},
			PieceId:      "testId",
			RemotePieces: rps,
		},
		SegmentSize: int64(1),
	}
	return pr
}

func randData(amount int) []byte {
	buf := make([]byte, amount)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return buf
}
