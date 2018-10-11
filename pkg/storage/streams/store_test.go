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

	proto "github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	ranger "storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/segments"
)

var (
	ctx = context.Background()
)

func TestStreamStoreMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSegmentStore := segments.NewMockStore(ctrl)

	md := pb.MetaStreamInfo{
		NumberOfSegments: 2,
		SegmentsSize:     10,
		LastSegmentSize:  0,
		Metadata:         []byte{},
	}
	lastSegmentMetadata, err := proto.Marshal(&md)
	if err != nil {
		t.Fatal(err)
	}

	staticTime := time.Now()
	segmentMeta := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       10,
		Data:       lastSegmentMetadata,
	}
	streamMeta, err := convertMeta(segmentMeta)
	if err != nil {
		t.Fatal(err)
	}

	for i, test := range []struct {
		// input for test function
		path string
		// output for mock function
		segmentMeta  segments.Meta
		segmentError error
		// assert on output of test function
		streamMeta  Meta
		streamError error
	}{
		{"bucket", segmentMeta, nil, streamMeta, nil},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockSegmentStore.EXPECT().
			Meta(gomock.Any(), gomock.Any()).
			Return(test.segmentMeta, test.segmentError)

		streamStore, err := NewStreamStore(mockSegmentStore, 10, "key", 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		meta, err := streamStore.Meta(ctx, paths.New(test.path))
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, test.streamMeta, meta, errTag)
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
		Data:       []byte{},
	}

	streamMeta := Meta{
		Modified:   segmentMeta.Modified,
		Expiration: segmentMeta.Expiration,
		Size:       4,
		Data:       []byte("metadata"),
	}

	for i, test := range []struct {
		// input for test function
		path       string
		data       io.Reader
		metadata   []byte
		expiration time.Time
		// output for mock function
		segmentMeta  segments.Meta
		segmentError error
		// assert on output of test function
		streamMeta  Meta
		streamError error
	}{
		{"bucket", strings.NewReader("data"), []byte("metadata"), staticTime, segmentMeta, nil, streamMeta, nil},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockSegmentStore.EXPECT().
			Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(test.segmentMeta, test.segmentError).
			Do(func(ctx context.Context, data io.Reader, expiration time.Time, info func() (paths.Path, []byte, error)) {
				for {
					buf := make([]byte, 4)
					_, err := data.Read(buf)
					if err == io.EOF {
						break
					}
				}
			})

		mockSegmentStore.EXPECT().
			Meta(gomock.Any(), gomock.Any()).
			Return(test.segmentMeta, test.segmentError)
		mockSegmentStore.EXPECT().
			Delete(gomock.Any(), gomock.Any()).
			Return(test.segmentError)

		streamStore, err := NewStreamStore(mockSegmentStore, 10, "key", 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		meta, err := streamStore.Put(ctx, paths.New(test.path), test.data, test.metadata, test.expiration)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, test.streamMeta, meta, errTag)
	}
}

type stubRanger struct {
	len    int64
	closer io.ReadCloser
}

func (r stubRanger) Size() int64 {
	return r.len
}
func (r stubRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	return r.closer, nil
}

type readCloserStub struct{}

func (r readCloserStub) Read(p []byte) (n int, err error) { return 10, nil }
func (r readCloserStub) Close() error                     { return nil }

func TestStreamStoreGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSegmentStore := segments.NewMockStore(ctrl)

	staticTime := time.Now()

	segmentRanger := stubRanger{
		len:    10,
		closer: readCloserStub{},
	}

	msi := pb.MetaStreamInfo{
		NumberOfSegments: 1,
		SegmentsSize:     10,
		LastSegmentSize:  0,
	}
	lastSegmentMeta, err := proto.Marshal(&msi)
	if err != nil {
		t.Fatal(err)
	}

	segmentMeta := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       10,
		Data:       lastSegmentMeta,
	}

	streamRanger := ranger.ByteRanger(nil)

	streamMeta := Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       0,
		Data:       nil,
	}

	for i, test := range []struct {
		// input for test function
		path string
		// output for mock function
		segmentRanger ranger.Ranger
		segmentMeta   segments.Meta
		segmentError  error
		// assert on output of test function
		streamRanger ranger.Ranger
		streamMeta   Meta
		streamError  error
	}{
		{"bucket", segmentRanger, segmentMeta, nil, streamRanger, streamMeta, nil},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		calls := []*gomock.Call{
			mockSegmentStore.EXPECT().
				Get(gomock.Any(), gomock.Any()).
				Return(test.segmentRanger, test.segmentMeta, test.segmentError),
		}

		gomock.InOrder(calls...)

		streamStore, err := NewStreamStore(mockSegmentStore, 10, "key", 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		ranger, meta, err := streamStore.Get(ctx, paths.New(test.path))
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, test.streamRanger.Size(), ranger.Size(), errTag)
		assert.Equal(t, test.streamMeta, meta, errTag)
	}
}

func TestStreamStoreDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSegmentStore := segments.NewMockStore(ctrl)

	staticTime := time.Now()
	segmentMeta := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       10,
		Data:       []byte{},
	}

	for i, test := range []struct {
		// input for test function
		path string
		// output for mock functions
		segmentMeta  segments.Meta
		segmentError error
		// assert on output of test function
		streamError error
	}{
		{"bucket", segmentMeta, nil, nil},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockSegmentStore.EXPECT().
			Meta(gomock.Any(), gomock.Any()).
			Return(test.segmentMeta, test.segmentError)
		mockSegmentStore.EXPECT().
			Delete(gomock.Any(), gomock.Any()).
			Return(test.segmentError)

		streamStore, err := NewStreamStore(mockSegmentStore, 10, "key", 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		err = streamStore.Delete(ctx, paths.New(test.path))
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, test.streamError, err, errTag)
	}
}
func TestStreamStoreList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSegmentStore := segments.NewMockStore(ctrl)

	for i, test := range []struct {
		// input for test function
		prefix     string
		startAfter string
		endBefore  string
		recursive  bool
		limit      int
		metaFlags  uint32
		// output for mock function
		segments     []segments.ListItem
		segmentMore  bool
		segmentError error
		// assert on output of test function
		streamItems []ListItem
		streamMore  bool
		streamError error
	}{
		{"bucket", "", "", false, 1, 0, []segments.ListItem{}, false, nil, []ListItem{}, false, nil},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		mockSegmentStore.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(test.segments, test.segmentMore, test.segmentError)

		streamStore, err := NewStreamStore(mockSegmentStore, 10, "key", 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		items, more, err := streamStore.List(ctx, paths.New(test.prefix), paths.New(test.startAfter), paths.New(test.endBefore), test.recursive, test.limit, test.metaFlags)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, test.streamItems, items, errTag)
		assert.Equal(t, test.streamMore, more, errTag)
	}
}
