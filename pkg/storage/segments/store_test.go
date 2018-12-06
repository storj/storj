// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/eestream/mocks"
	mock_overlay "storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/pb"
	pdb "storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/pointerdb/pdbclient/mocks"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/ec/mocks"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storj"
)

var (
	ctx = context.Background()
)

func TestNewSegmentStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOC := mock_overlay.NewMockClient(ctrl)
	mockEC := mock_ecclient.NewMockClient(ctrl)
	mockPDB := mock_pointerdb.NewMockClient(ctrl)
	rs := eestream.RedundancyStrategy{
		ErasureScheme: mock_eestream.NewMockErasureScheme(ctrl),
	}

	ss := NewSegmentStore(mockOC, mockEC, mockPDB, rs, 10, 0, 0, 0, 0)
	assert.NotNil(t, ss)
}

func TestSegmentStoreMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOC := mock_overlay.NewMockClient(ctrl)
	mockEC := mock_ecclient.NewMockClient(ctrl)
	mockPDB := mock_pointerdb.NewMockClient(ctrl)
	rs := eestream.RedundancyStrategy{
		ErasureScheme: mock_eestream.NewMockErasureScheme(ctrl),
	}

	ss := segmentStore{mockOC, mockEC, mockPDB, rs, 10, nodeStats{}}
	assert.NotNil(t, ss)

	var mExp time.Time
	pExp, err := ptypes.TimestampProto(mExp)
	assert.NoError(t, err)

	for _, tt := range []struct {
		pathInput     string
		returnPointer *pb.Pointer
		returnMeta    Meta
	}{
		{"path/1/2/3", &pb.Pointer{CreationDate: pExp, ExpirationDate: pExp}, Meta{Modified: mExp, Expiration: mExp}},
	} {
		calls := []*gomock.Call{
			mockPDB.EXPECT().Get(
				gomock.Any(), gomock.Any(),
			).Return(tt.returnPointer, nil, nil, nil),
		}
		gomock.InOrder(calls...)

		m, err := ss.Meta(ctx, tt.pathInput)
		assert.NoError(t, err)
		assert.Equal(t, m, tt.returnMeta)
	}
}

func TestSegmentStorePutRemote(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range []struct {
		name          string
		pathInput     string
		mdInput       []byte
		thresholdSize int
		expiration    time.Time
		readerContent string
	}{
		{"test remote put", "path/1", []byte("abcdefghijklmnopqrstuvwxyz"), 2, time.Unix(0, 0).UTC(), "readerreaderreader"},
	} {
		mockOC := mock_overlay.NewMockClient(ctrl)
		mockEC := mock_ecclient.NewMockClient(ctrl)
		mockPDB := mock_pointerdb.NewMockClient(ctrl)
		mockES := mock_eestream.NewMockErasureScheme(ctrl)
		rs := eestream.RedundancyStrategy{
			ErasureScheme: mockES,
		}

		ss := segmentStore{mockOC, mockEC, mockPDB, rs, tt.thresholdSize, nodeStats{}}
		assert.NotNil(t, ss)

		calls := []*gomock.Call{
			mockES.EXPECT().TotalCount().Return(1),
			mockOC.EXPECT().Choose(
				gomock.Any(), gomock.Any(),
			).Return([]*pb.Node{
				{Id: teststorj.NodeIDFromString("im-a-node")},
			}, nil),
			mockPDB.EXPECT().SignedMessage(),
			mockPDB.EXPECT().PayerBandwidthAllocation(gomock.Any(), gomock.Any()),
			mockEC.EXPECT().Put(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			),
			mockES.EXPECT().RequiredCount().Return(1),
			mockES.EXPECT().TotalCount().Return(1),
			mockES.EXPECT().ErasureShareSize().Return(1),
			mockPDB.EXPECT().Put(
				gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(nil),
			mockPDB.EXPECT().Get(
				gomock.Any(), gomock.Any(),
			),
		}
		gomock.InOrder(calls...)

		_, err := ss.Put(ctx, strings.NewReader(tt.readerContent), tt.expiration, func() (storj.Path, []byte, error) {
			return tt.pathInput, tt.mdInput, nil
		})
		assert.NoError(t, err, tt.name)
	}
}

func TestSegmentStorePutInline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range []struct {
		name          string
		pathInput     string
		mdInput       []byte
		thresholdSize int
		expiration    time.Time
		readerContent string
	}{
		{"test inline put", "path/1", []byte("111"), 1000, time.Unix(0, 0).UTC(), "readerreaderreader"},
	} {
		mockOC := mock_overlay.NewMockClient(ctrl)
		mockEC := mock_ecclient.NewMockClient(ctrl)
		mockPDB := mock_pointerdb.NewMockClient(ctrl)
		mockES := mock_eestream.NewMockErasureScheme(ctrl)
		rs := eestream.RedundancyStrategy{
			ErasureScheme: mockES,
		}

		ss := segmentStore{mockOC, mockEC, mockPDB, rs, tt.thresholdSize, nodeStats{}}
		assert.NotNil(t, ss)

		calls := []*gomock.Call{
			mockPDB.EXPECT().Put(
				gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(nil),
			mockPDB.EXPECT().Get(
				gomock.Any(), gomock.Any(),
			),
		}
		gomock.InOrder(calls...)

		_, err := ss.Put(ctx, strings.NewReader(tt.readerContent), tt.expiration, func() (storj.Path, []byte, error) {
			return tt.pathInput, tt.mdInput, nil
		})
		assert.NoError(t, err, tt.name)
	}
}

func TestSegmentStoreGetInline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ti := time.Unix(0, 0).UTC()
	someTime, err := ptypes.TimestampProto(ti)
	assert.NoError(t, err)

	for _, tt := range []struct {
		pathInput     string
		thresholdSize int
		pointerType   pb.Pointer_DataType
		inlineContent []byte
		size          int64
		metadata      []byte
	}{
		{"path/1/2/3", 10, pb.Pointer_INLINE, []byte("000"), int64(3), []byte("metadata")},
	} {
		mockOC := mock_overlay.NewMockClient(ctrl)
		mockEC := mock_ecclient.NewMockClient(ctrl)
		mockPDB := mock_pointerdb.NewMockClient(ctrl)
		mockES := mock_eestream.NewMockErasureScheme(ctrl)
		rs := eestream.RedundancyStrategy{
			ErasureScheme: mockES,
		}

		ss := segmentStore{mockOC, mockEC, mockPDB, rs, tt.thresholdSize, nodeStats{}}
		assert.NotNil(t, ss)

		calls := []*gomock.Call{
			mockPDB.EXPECT().Get(
				gomock.Any(), gomock.Any(),
			).Return(&pb.Pointer{
				Type:           tt.pointerType,
				InlineSegment:  tt.inlineContent,
				CreationDate:   someTime,
				ExpirationDate: someTime,
				SegmentSize:    tt.size,
				Metadata:       tt.metadata,
			}, nil, nil, nil),
		}
		gomock.InOrder(calls...)

		_, _, err := ss.Get(ctx, tt.pathInput)
		assert.NoError(t, err)
	}
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
		mockES := mock_eestream.NewMockErasureScheme(ctrl)
		rs := eestream.RedundancyStrategy{
			ErasureScheme: mockES,
		}

		ss := segmentStore{mockOC, mockEC, mockPDB, rs, tt.thresholdSize, nodeStats{}}
		assert.NotNil(t, ss)

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
			mockPDB.EXPECT().SignedMessage(),
			mockEC.EXPECT().Get(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(ranger.ByteRanger([]byte(tt.data)), nil),
			mockEC.EXPECT().Put(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(tt.newNodes, nil),
			mockES.EXPECT().RequiredCount().Return(1),
			mockES.EXPECT().TotalCount().Return(1),
			mockES.EXPECT().ErasureShareSize().Return(1),
			mockPDB.EXPECT().Put(
				gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(nil),
		}
		gomock.InOrder(calls...)

		err := ss.Repair(ctx, tt.pathInput, tt.lostPieces)
		assert.NoError(t, err)
	}
}

func TestSegmentStoreGetRemote(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ti := time.Unix(0, 0).UTC()
	someTime, err := ptypes.TimestampProto(ti)
	assert.NoError(t, err)

	for _, tt := range []struct {
		pathInput     string
		thresholdSize int
		pointerType   pb.Pointer_DataType
		size          int64
		metadata      []byte
	}{
		{"path/1/2/3", 10, pb.Pointer_REMOTE, int64(3), []byte("metadata")},
	} {
		mockOC := mock_overlay.NewMockClient(ctrl)
		mockEC := mock_ecclient.NewMockClient(ctrl)
		mockPDB := mock_pointerdb.NewMockClient(ctrl)
		mockES := mock_eestream.NewMockErasureScheme(ctrl)
		rs := eestream.RedundancyStrategy{
			ErasureScheme: mockES,
		}

		ss := segmentStore{mockOC, mockEC, mockPDB, rs, tt.thresholdSize, nodeStats{}}
		assert.NotNil(t, ss)

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
			mockPDB.EXPECT().SignedMessage(),
			mockEC.EXPECT().Get(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			),
		}
		gomock.InOrder(calls...)

		_, _, err := ss.Get(ctx, tt.pathInput)
		assert.NoError(t, err)
	}
}

func TestSegmentStoreDeleteInline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ti := time.Unix(0, 0).UTC()
	someTime, err := ptypes.TimestampProto(ti)
	assert.NoError(t, err)

	for _, tt := range []struct {
		pathInput     string
		thresholdSize int
		pointerType   pb.Pointer_DataType
		inlineContent []byte
		size          int64
		metadata      []byte
	}{
		{"path/1/2/3", 10, pb.Pointer_INLINE, []byte("000"), int64(3), []byte("metadata")},
	} {
		mockOC := mock_overlay.NewMockClient(ctrl)
		mockEC := mock_ecclient.NewMockClient(ctrl)
		mockPDB := mock_pointerdb.NewMockClient(ctrl)
		mockES := mock_eestream.NewMockErasureScheme(ctrl)
		rs := eestream.RedundancyStrategy{
			ErasureScheme: mockES,
		}

		ss := segmentStore{mockOC, mockEC, mockPDB, rs, tt.thresholdSize, nodeStats{}}
		assert.NotNil(t, ss)

		calls := []*gomock.Call{
			mockPDB.EXPECT().Get(
				gomock.Any(), gomock.Any(),
			).Return(&pb.Pointer{
				Type:           tt.pointerType,
				InlineSegment:  tt.inlineContent,
				CreationDate:   someTime,
				ExpirationDate: someTime,
				SegmentSize:    tt.size,
				Metadata:       tt.metadata,
			}, nil, nil, nil),
			mockPDB.EXPECT().Delete(
				gomock.Any(), gomock.Any(),
			),
		}
		gomock.InOrder(calls...)

		err := ss.Delete(ctx, tt.pathInput)
		assert.NoError(t, err)
	}
}

func TestSegmentStoreDeleteRemote(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ti := time.Unix(0, 0).UTC()
	someTime, err := ptypes.TimestampProto(ti)
	assert.NoError(t, err)

	for _, tt := range []struct {
		pathInput     string
		thresholdSize int
		pointerType   pb.Pointer_DataType
		size          int64
		metadata      []byte
	}{
		{"path/1/2/3", 10, pb.Pointer_REMOTE, int64(3), []byte("metadata")},
	} {
		mockOC := mock_overlay.NewMockClient(ctrl)
		mockEC := mock_ecclient.NewMockClient(ctrl)
		mockPDB := mock_pointerdb.NewMockClient(ctrl)
		mockES := mock_eestream.NewMockErasureScheme(ctrl)
		rs := eestream.RedundancyStrategy{
			ErasureScheme: mockES,
		}

		ss := segmentStore{mockOC, mockEC, mockPDB, rs, tt.thresholdSize, nodeStats{}}
		assert.NotNil(t, ss)

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
			mockPDB.EXPECT().SignedMessage(),
			mockEC.EXPECT().Delete(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			),
			mockPDB.EXPECT().Delete(
				gomock.Any(), gomock.Any(),
			),
		}
		gomock.InOrder(calls...)

		err := ss.Delete(ctx, tt.pathInput)
		assert.NoError(t, err)
	}
}

func TestSegmentStoreList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range []struct {
		prefix        string
		startAfter    string
		thresholdSize int
		itemPath      string
		inlineContent []byte
		metadata      []byte
	}{
		{"bucket1", "s0/path/1", 10, "s0/path/1", []byte("inline"), []byte("metadata")},
	} {
		mockOC := mock_overlay.NewMockClient(ctrl)
		mockEC := mock_ecclient.NewMockClient(ctrl)
		mockPDB := mock_pointerdb.NewMockClient(ctrl)
		mockES := mock_eestream.NewMockErasureScheme(ctrl)
		rs := eestream.RedundancyStrategy{
			ErasureScheme: mockES,
		}

		ss := segmentStore{mockOC, mockEC, mockPDB, rs, tt.thresholdSize, nodeStats{}}
		assert.NotNil(t, ss)

		ti := time.Unix(0, 0).UTC()
		someTime, err := ptypes.TimestampProto(ti)
		assert.NoError(t, err)

		calls := []*gomock.Call{
			mockPDB.EXPECT().List(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				gomock.Any(), gomock.Any(), gomock.Any(),
			).Return([]pdb.ListItem{
				{
					Path: tt.itemPath,
					Pointer: &pb.Pointer{
						Type:           pb.Pointer_INLINE,
						InlineSegment:  tt.inlineContent,
						CreationDate:   someTime,
						ExpirationDate: someTime,
						SegmentSize:    int64(4),
						Metadata:       tt.metadata,
					},
				},
			}, true, nil),
		}
		gomock.InOrder(calls...)

		_, _, err = ss.List(ctx, tt.prefix, tt.startAfter, "", false, 10, meta.Modified)
		assert.NoError(t, err)
	}
}
