// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"fmt"
	"io"
	"strings"
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

	staticTime := time.Now()
	segmentMeta := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       10,
		Data:       []byte("data"),
	}
	streamMeta, err := convertMeta(segmentMeta)
	if err != nil {
		t.Fatal(err)
	}

	for i, test := range []struct {
		path         string
		segmentMeta  segments.Meta
		segmentError error
		streamMeta   Meta
		streamError  error
	}{
		{"bucket", segmentMeta, nil, streamMeta, nil},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockSegmentStore.EXPECT().
			Meta(gomock.Any(), gomock.Any()).
			Return(test.segmentMeta, test.segmentError)

		streamStore, err := NewStreamStore(mockSegmentStore, 10)
		if err != nil {
			t.Fatal(err)
		}

		meta, err := streamStore.Meta(ctx, paths.New(test.path))
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, meta, test.streamMeta, errTag)
	}
}

func TestStreamStorePut(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSegmentStore := segments.NewMockStore(ctrl)

	staticTime := time.Now()
	segmentMeta := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       10,
		Data:       []byte("data"),
	}
	streamMeta := Meta{
		Modified:   segmentMeta.Modified,
		Expiration: segmentMeta.Expiration,
		Size:       10,
		Data:       []byte("data"),
	}

	for i, test := range []struct {
		path         string
		data         io.Reader
		metadata     []byte
		expiration   time.Time
		segmentMeta  segments.Meta
		segmentError error
		streamMeta   Meta
		streamError  error
	}{
		{"bucket", strings.NewReader("data"), []byte("metadata"), staticTime, segmentMeta, nil, streamMeta, nil},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockSegmentStore.EXPECT().
			Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(test.segmentMeta, test.streamError).
			Do(func() {
				for {
					buf := make([]byte, 4)
					_, err := test.data.Read(buf)
					if err == io.EOF {
						break
					}
				}
			})

		streamStore, err := NewStreamStore(mockSegmentStore, 10)
		if err != nil {
			t.Fatal(err)
		}

		meta, err := streamStore.Put(ctx, paths.New(test.path), test.data, test.metadata, test.expiration)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, meta, test.streamMeta, errTag)
	}
}
