// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storage/segments"
)

var (
	ctx = context.Background()
)

func TestStreamStoreMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSegmentStore := segments.NewMockStore(ctrl)

	streamStore, err := NewStreamStore(mockSegmentStore, 10)
	if err != nil {
		t.Fatal(err)
	}

	staticTime := time.Now()
	segmentMeta := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       10,
		Data:       []byte("data"),
	}
	streamMeta, err := convertMeta(segmentMeta)

	for i, test := range []struct {
		path        string
		segmentMeta segments.Meta
		streamMeta  Meta
	}{
		{"bucket", segmentMeta, streamMeta},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockSegmentStore.EXPECT().Meta(gomock.Any(), gomock.Any()).Return(test.segmentMeta, nil)

		meta, err := streamStore.Meta(ctx, paths.New(test.path))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, meta, test.streamMeta, errTag)
	}
}
