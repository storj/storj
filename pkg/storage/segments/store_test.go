// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/eestream"
	mock_eestream "storj.io/storj/pkg/eestream/mocks"
	mock_overlay "storj.io/storj/pkg/overlay/mocks"
	"storj.io/storj/pkg/paths"
	mock_pointerdb "storj.io/storj/pkg/pointerdb/mocks"
	mock_ecclient "storj.io/storj/pkg/storage/ec/mocks"
	ppb "storj.io/storj/protos/pointerdb"
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

	ss := NewSegmentStore(mockOC, mockEC, mockPDB, rs, 10)
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

	ss := segmentStore{mockOC, mockEC, mockPDB, rs, 10}
	assert.NotNil(t, ss)

	var tim time.Time
	tim = time.Now()
	newTim, err := ptypes.TimestampProto(tim)
	assert.NoError(t, err)

	var tim2 *timestamp.Timestamp
	tim2 = ptypes.TimestampNow()
	newTim2, err := ptypes.Timestamp(tim2)
	assert.NoError(t, err)

	for _, tt := range []struct {
		pathInput     string
		returnPointer *ppb.Pointer
		returnMeta    Meta
	}{
		{"path/1/2/3", &ppb.Pointer{ExpirationDate: newTim}, Meta{Modified: newTim2, Expiration: tim}},
	} {
		p := paths.New(tt.pathInput)

		calls := []*gomock.Call{
			mockPDB.EXPECT().Get(
				gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(tt.returnPointer, nil),
		}
		gomock.InOrder(calls...)

		m, err := ss.Meta(ctx, p)
		assert.NoError(t, err)
		assert.Equal(t, m, tt.returnMeta)
	}
}
