// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"crypto/rand"
	"strconv"
	"testing"

	"storj.io/storj/pkg/pb"
)

type mockDownloader struct {
	shares map[int]share
}

func TestDownloadShares(t *testing.T) {
	ctx := context.Background()
	mockShares := make(map[int]share)

	for i, tt := range []struct {
		stripeIndex int
		nodeAmount  int
		shareAmount int
		required    int
		total       int
		err         error
	}{
		{2, 30, 30, 20, 40, nil},
	} {
		someData := randData(32 * 1024)
		pointer := makePointer()
		var nodes []*pb.Node
		for i = 0; i < tt.nodeAmount; i++ {
			node := &pb.Node{
				Id:      strconv.Itoa(i),
				Address: &pb.NodeAddress{},
			}
			nodes = append(nodes, node)
		}

		for i = 0; i < tt.shareAmount; i++ {
			mockShares[i] = share{
				Error:       tt.err,
				PieceNumber: i,
				Data:        someData,
			}
		}
		md := mockDownloader{shares: mockShares}
		a := &Auditor{downloader: &md}

		_, err := a.auditStripe(ctx, pointer, tt.stripeIndex, tt.required, tt.total)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestAuditShares(t *testing.T) {
	ctx := context.Background()

	for i, tt := range []struct {
		shareAmount int
		required    int
		total       int
		err         error
	}{
		{30, 20, 40, nil},
	} {
		var shares []share
		someData := randData(32 * 1024)
		for i = 0; i < tt.shareAmount; i++ {
			s := share{
				Error:       nil,
				PieceNumber: i,
				Data:        someData,
			}
			shares = append(shares, s)
		}

		_, err := auditShares(ctx, tt.required, tt.total, shares)
		if err != nil {
			panic(err)
		}
	}
}

func (m *mockDownloader) DownloadShares(ctx context.Context, pointer *pb.Pointer,
	stripeIndex int) (shares []share, nodes []*pb.Node, err error) {
	for _, share := range m.shares {
		shares = append(shares, share)
	}
	return shares, nodes, nil
}

func (m *mockDownloader) lookupNodes(ctx context.Context, pieces []*pb.RemotePiece) (nodes []*pb.Node, err error) {
	return
}

func makePointer() *pb.Pointer {
	var rps []*pb.RemotePiece
	for i := 0; i < 15; i++ {
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
				MinReq:           1,
				Total:            3,
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
