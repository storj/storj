// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"strconv"
	"testing"

	"github.com/vivint/infectious"

	"storj.io/storj/pkg/pb"
)

type mockDownloader struct {
	shares map[int]share
}

func TestPassingAudit(t *testing.T) {
	ctx := context.Background()
	mockShares := make(map[int]share)

	for i, tt := range []struct {
		nodeAmt  int
		shareAmt int
		required int
		total    int
		err      error
	}{
		{nodeAmt: 30, shareAmt: 30, required: 20, total: 40, err: nil},
	} {
		someData := randData(32 * 1024)
		var nodes []*pb.Node
		for i = 0; i < tt.nodeAmt; i++ {
			node := &pb.Node{
				Id:      strconv.Itoa(i),
				Address: &pb.NodeAddress{},
			}
			nodes = append(nodes, node)
		}
		for i = 0; i < tt.shareAmt; i++ {
			mockShares[i] = share{
				Error:       tt.err,
				PieceNumber: i,
				Data:        someData,
			}
		}
		md := mockDownloader{shares: mockShares}
		auditor := &Auditor{downloader: &md}
		pointer := makePointer(tt.nodeAmt)
		badNodes, err := auditor.auditStripe(ctx, pointer, 6, 20, 40)
		if err != nil {
			t.Fatal(err)
		}
		if len(badNodes) != 0 {
			t.Fatal(err)
		}
	}
}

func TestFailingAudit(t *testing.T) {
	ctx := context.Background()
	mockShares := make([]share, 30)

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
		for i := range mockShares {
			mockShares[i] = share{
				Error:       tt.err,
				PieceNumber: i,
				Data:        someData,
			}
		}
		copies, err := makeCopies(ctx, mockShares)
		copies = copies[2:]
		copies[2].Data[1] = '!'

		f, err := infectious.NewFEC(tt.required, tt.total)
		if err != nil {
			panic(err)
		}

		err = f.Correct(copies)
		if err != nil {
			t.Fatal(err)
		}

		var pieceNums []int
		for i, share := range copies {
			if !bytes.Equal(mockShares[i].Data, share.Data) {
				fmt.Println(mockShares[i].Data[:10])
				fmt.Println(share.Data[:10])
				pieceNums = append(pieceNums, share.Number)
			}
		}

		if len(pieceNums) != 2 {
			t.Fatal("expected pieceNums to have len 2")
		}
	}
}

func TestExample(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	// Create a *FEC, which will require required pieces for reconstruction at
	// minimum, and generate total total pieces.
	f, err := infectious.NewFEC(required, total)
	if err != nil {
		panic(err)
	}

	// Prepare to receive the shares of encoded data.
	shares := make([]infectious.Share, total)
	output := func(s infectious.Share) {
		// the memory in s gets reused, so we need to make a deep copy
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

	// Let's reconstitute with two pieces missing and one piece corrupted.
	modifiedShares = modifiedShares[2:] // drop the first two pieces
	modifiedShares = append(modifiedShares, shares[0])
	modifiedShares = append(modifiedShares, shares[1])

	fmt.Println("modified shares", modifiedShares[0].Data[:3])
	fmt.Println("hello world good data", shares[0].Data[:3])

	preCorrectedShares := make([]infectious.Share, len(shares))
	for i := range shares {
		preCorrectedShares[i] = modifiedShares[i].DeepCopy()
	}
	fmt.Println("pre-corrected shares", preCorrectedShares[0].Data[:3])
	fmt.Println("modified shares", modifiedShares[0].Data[:3])
	fmt.Println("hello world good data", shares[0].Data[:3])

	modifiedShares[0].Data[1] = '!'
	modifiedShares[2].Data[0] = '#' // mutate some data
	modifiedShares[3].Data[1] = '!'
	modifiedShares[4].Data[0] = 'b' // mutate some data
	// modifiedShares[5].Data[0] = 'a' // mutate some data
	// modifiedShares[6].Data[1] = '!'
	// modifiedShares[7].Data[0] = '#' // mutate some data

	err = f.Correct(modifiedShares)
	if err != nil {
		panic(err)
	}

	fmt.Println("pre-corrected shares", preCorrectedShares[0].Data[:3])
	fmt.Println("modified shares", modifiedShares[0].Data[:3])
	fmt.Println("hello world good data", shares[0].Data[:3])

	//need to copy modifiedShares then compare copiedModifiedShares to corrected modified shares
	var badNums []int
	for i, pre := range preCorrectedShares {
		if !bytes.Equal(modifiedShares[i].Data, pre.Data) {
			badNums = append(badNums, pre.Number)
		}
	}
	fmt.Println(badNums)

	ctx := context.Background()
	ourPkgShares := make([]share, len(modifiedShares))
	for i := range modifiedShares {
		ourPkgShares[i].PieceNumber = modifiedShares[i].Number
		ourPkgShares[i].Data = append([]byte(nil), modifiedShares[i].Data...)
		ourPkgShares[i].Error = nil
		fmt.Println("ourpkgshare", ourPkgShares[i].Data[:3])
		fmt.Println("mod shares", modifiedShares[i].Data[:3])
	}
	pieceNums, err := auditShares(ctx, 8, 14, ourPkgShares)
	if err != nil {
		panic(err)
	}
	fmt.Println(pieceNums)

}

// if i == 1 {
// 	mockShares[i] = share{
// 		Error: Error.New("couldn't download"),
// 	}
// }

// md := mockDownloader{shares: mockShares}
// auditor := &Auditor{downloader: &md}
// pointer := makePointer(tt.nodeAmt)

func TestNotEnoughNodes(t *testing.T) {
	ctx := context.Background()
	mockShares := make(map[int]share)

	for i, tt := range []struct {
		nodeAmt  int
		shareAmt int
		required int
		total    int
		err      error
	}{
		{nodeAmt: 30, shareAmt: 30, required: 20, total: 40, err: nil},
	} {
		someData := randData(32 * 1024)
		var nodes []*pb.Node
		for i = 0; i < tt.nodeAmt; i++ {
			node := &pb.Node{
				Id:      strconv.Itoa(i),
				Address: &pb.NodeAddress{},
			}
			nodes = append(nodes, node)
		}
		for i = 0; i < tt.shareAmt; i++ {
			mockShares[i] = share{
				Error:       tt.err,
				PieceNumber: i,
				Data:        someData,
			}
		}
		md := mockDownloader{shares: mockShares}
		auditor := &Auditor{downloader: &md}
		pointer := makePointer(tt.nodeAmt)
		_, err := auditor.auditStripe(ctx, pointer, 6, 20, 40)
		if err != nil {
			t.Fatal(err)
		}
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
