// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"crypto/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vivint/infectious"

	"storj.io/storj/pkg/pb"
)

type mockDownloader struct {
	shares map[int]share
}

func TestPassingAudit(t *testing.T) {
	ctx := context.Background()
	mockShares := make(map[int]share)

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
			mockShares[i] = share{
				Error:       tt.err,
				PieceNumber: i,
				Data:        someData,
			}
		}
		md := mockDownloader{shares: mockShares}
		verifier := &Verifier{downloader: &md}
		pointer := makePointer(tt.nodeAmt)
		failedNodes, err := verifier.verify(ctx, 6, pointer)
		if err != nil {
			t.Fatal(err)
		}
		if len(failedNodes) != 0 {
			t.Fatal("expected there to be no recorded bad nodes")
		}
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
	auditPkgShares := make([]share, len(modifiedShares))
	for i := range modifiedShares {
		auditPkgShares[i].PieceNumber = modifiedShares[i].Number
		auditPkgShares[i].Data = append([]byte(nil), modifiedShares[i].Data...)
		auditPkgShares[i].Error = nil
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
	auditPkgShares := make([]share, len(shares))
	for i := range shares {
		auditPkgShares[i].PieceNumber = shares[i].Number
		auditPkgShares[i].Data = append([]byte(nil), shares[i].Data...)
		auditPkgShares[i].Error = nil
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

func (m *mockDownloader) DownloadShares(ctx context.Context, pointer *pb.Pointer,
	stripeIndex int) (shares []share, nodes []*pb.Node, err error) {
	for _, share := range m.shares {
		shares = append(shares, share)
	}
	for i := 0; i < 30; i++ {
		node := &pb.Node{
			Id:      strconv.Itoa(i),
			Address: &pb.NodeAddress{},
		}
		nodes = append(nodes, node)
	}
	return shares, nodes, nil
}

func makePointer(nodeAmt int) *pb.Pointer {
	var rps []*pb.RemotePiece
	for i := 0; i < nodeAmt; i++ {
		rps = append(rps, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   "test" + strconv.Itoa(i),
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
		Size: int64(1),
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
