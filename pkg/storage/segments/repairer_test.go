// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/teststorj"
	mock_overlay "storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	mock_pointerdb "storj.io/storj/pkg/pointerdb/pdbclient/mocks"
	"storj.io/storj/pkg/ranger"
	mock_ecclient "storj.io/storj/pkg/storage/ec/mocks"
)

func TestNewSegmentRepairer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOC := mock_overlay.NewMockClient(ctrl)
	mockEC := mock_ecclient.NewMockClient(ctrl)
	mockPDB := mock_pointerdb.NewMockClient(ctrl)

	ss := NewSegmentRepairer(mockOC, mockEC, mockPDB)
	assert.NotNil(t, ss)
}

func TestSegmentStoreRepairRemote(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ti := time.Unix(0, 0).UTC()
	someTime, err := ptypes.TimestampProto(ti)
	assert.NoError(t, err)

	for _, tt := range []struct {
		pathInput               string
		thresholdSize           int
		pointerType             pb.Pointer_DataType
		size                    int64
		metadata                []byte
		lostPieces              []int32
		newNodes                []*pb.Node
		data                    string
		strsize, offset, length int64
		substr                  string
		meta                    Meta
	}{
		{
			"path/1/2/3",
			10,
			pb.Pointer_REMOTE,
			int64(3),
			[]byte("metadata"),
			[]int32{},
			[]*pb.Node{
				teststorj.MockNode("1"),
				teststorj.MockNode("2"),
			},
			"abcdefghijkl",
			12,
			1,
			4,
			"bcde",
			Meta{},
		},
	} {
		mockOC := mock_overlay.NewMockClient(ctrl)
		mockEC := mock_ecclient.NewMockClient(ctrl)
		mockPDB := mock_pointerdb.NewMockClient(ctrl)

		sr := Repairer{mockOC, mockEC, mockPDB, &pb.NodeStats{}}
		assert.NotNil(t, sr)

		calls := []*gomock.Call{
			mockPDB.EXPECT().Get(
				gomock.Any(), gomock.Any(),
			).Return(&pb.Pointer{
				Type: tt.pointerType,
				Remote: &pb.RemoteSegment{
					Redundancy: &pb.RedundancyScheme{
						Type:             pb.RedundancyScheme_RS,
						MinReq:           1,
						Total:            2,
						RepairThreshold:  1,
						SuccessThreshold: 2,
					},
					PieceId:      "here's my piece id",
					RemotePieces: []*pb.RemotePiece{},
				},
				CreationDate:   someTime,
				ExpirationDate: someTime,
				SegmentSize:    tt.size,
				Metadata:       tt.metadata,
			}, nil, nil, nil),
			mockOC.EXPECT().BulkLookup(gomock.Any(), gomock.Any()),
			mockOC.EXPECT().Choose(gomock.Any(), gomock.Any()).Return(tt.newNodes, nil),
			mockPDB.EXPECT().PayerBandwidthAllocation(gomock.Any(), gomock.Any()),
			mockEC.EXPECT().Get(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(ranger.ByteRanger([]byte(tt.data)), nil),
			mockPDB.EXPECT().PayerBandwidthAllocation(gomock.Any(), gomock.Any()),
			mockEC.EXPECT().Put(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(tt.newNodes, nil),
			mockPDB.EXPECT().Put(
				gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(nil),
		}
		gomock.InOrder(calls...)

		err := sr.Repair(ctx, tt.pathInput, tt.lostPieces)
		assert.NoError(t, err)
	}
}
