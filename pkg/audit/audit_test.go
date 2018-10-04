// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"io/ioutil"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/vivint/infectious"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/eestream"
	mock_oclient "storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	mock_psclient "storj.io/storj/pkg/piecestore/rpc/client/mocks"
	"storj.io/storj/pkg/provider"
	mock_ranger "storj.io/storj/pkg/ranger/mocks"
	mock_transport "storj.io/storj/pkg/transport/mocks"
)

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

func makeReaders() (readerMap map[int]io.ReadCloser, err error) {
	ctx := context.Background()
	data := randData(32 * 1024)
	fc, err := infectious.NewFEC(2, 4)
	if err != nil {
		return nil, err
	}
	es := eestream.NewRSScheme(fc, 8*1024)
	rs, err := eestream.NewRedundancyStrategy(es, 0, 0)
	if err != nil {
		return nil, err
	}
	readers, err := eestream.EncodeReader(ctx, bytes.NewReader(data), rs, 0)
	if err != nil {
		return nil, err
	}
	readerMap = make(map[int]io.ReadCloser, len(readers))
	for i, reader := range readers {
		readerMap[i] = ioutil.NopCloser(reader)
	}
	return readerMap, nil
}

func TestRunAudit(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOC := mock_oclient.NewMockClient(ctrl)
	mockPSC := mock_psclient.NewMockPSClient(ctrl)
	mockTC := mock_transport.NewMockClient(ctrl)
	mockRanger := mock_ranger.NewMockRanger(ctrl)

	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	id := provider.FullIdentity{Key: privKey}

	a := &Auditor{t: mockTC, o: mockOC, identity: id}
	p := makePointer()

	var nodes []*pb.Node
	for i := 0; i < 15; i++ {
		nodes = append(nodes, &pb.Node{
			Id:      "fakeId",
			Address: &pb.NodeAddress{},
		})
	}

	var conn *grpc.ClientConn
	readers, err := makeReaders()
	if err != nil {
		panic(err)
	}

	mockOC.EXPECT().BulkLookup(
		gomock.Any(), gomock.Any()).Return(nodes, nil)
	mockTC.EXPECT().DialNode(
		gomock.Any(), gomock.Any()).Return(conn, nil)
	mockPSC.EXPECT().Get(
		gomock.Any(), gomock.Any(), gomock.Any(), &pb.PayerBandwidthAllocation{},
	).Return(mockRanger, nil).AnyTimes()
	mockRanger.EXPECT().Range(
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(readers[0], nil).AnyTimes()

	_, err = a.runAudit(ctx, p, 15, 20, 40)
	if err != nil {
		panic(err)
	}
}
