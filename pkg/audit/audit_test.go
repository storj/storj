// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"

	"storj.io/storj/pkg/pb"
	mock_psclient "storj.io/storj/pkg/piecestore/rpc/client/mocks"
	mock_ranger "storj.io/storj/pkg/ranger/mocks"
)

var (
	ctx = context.Background()
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

type buffer struct {
	*bytes.Buffer
}

func (b *buffer) Close() (err error) {
	return
}
func TestRunAudit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPSC := mock_psclient.NewMockPSClient(ctrl)
	mockRanger := mock_ranger.NewMockRanger(ctrl)

	a := &Auditor{ps: mockPSC}
	p := makePointer()

	b := &buffer{bytes.NewBufferString("mock closer")}
	var rc io.ReadCloser
	rc = b

	mockPSC.EXPECT().Meta(gomock.Any(), gomock.Any()).AnyTimes()
	mockPSC.EXPECT().Get(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(mockRanger, nil).AnyTimes()
	mockRanger.EXPECT().Range(
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(rc, nil).AnyTimes()

	_, err := a.runAudit(ctx, p, 15, 20, 40)
	if err != nil {
		panic(err)
	}
}
