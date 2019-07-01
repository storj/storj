// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storj"
)

var (
	ctx = context.Background()
)

func newStore() *encryption.Store {
	store := encryption.NewStore()
	store.SetDefaultKey(new(storj.Key))
	return store
}

func TestStreamStoreMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSegmentStore := segments.NewMockStore(ctrl)

	streamInfo := pb.StreamInfo{
		NumberOfSegments: 2,
		SegmentsSize:     10,
		LastSegmentSize:  0,
		Metadata:         nil,
	}
	stream, err := proto.Marshal(&streamInfo)
	if err != nil {
		t.Fatal(err)
	}

	lastSegmentMetadata, err := proto.Marshal(&pb.StreamMeta{
		EncryptedStreamInfo: stream, EncryptionType: int32(storj.EncNull),
	})
	if err != nil {
		t.Fatal(err)
	}
	staticTime := time.Now()
	segmentMeta := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       50,
		Data:       lastSegmentMetadata,
	}

	streamMetaUnmarshaled := pb.StreamMeta{}
	err = proto.Unmarshal(segmentMeta.Data, &streamMetaUnmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	segmentMetaStreamInfo := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       50,
		Data:       streamMetaUnmarshaled.EncryptedStreamInfo,
	}

	streamMeta := convertMeta(segmentMetaStreamInfo, streamInfo, streamMetaUnmarshaled)

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

		streamStore, err := NewStreamStore(mockSegmentStore, 10, newStore(), 10, storj.EncAESGCM, 4)
		if err != nil {
			t.Fatal(err)
		}

		meta, err := streamStore.Meta(ctx, test.path, storj.EncAESGCM)
		if err != nil {
			t.Fatalf("%+v", err)
		}

		assert.Equal(t, test.streamMeta, meta, errTag)
	}
}

func TestStreamStorePut(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSegmentStore := segments.NewMockStore(ctrl)

	const (
		encBlockSize = 10
		segSize      = 10
		pathCipher   = storj.EncAESGCM
		dataCipher   = storj.EncNull
		inlineSize   = 0
	)

	staticTime := time.Now()
	segmentMeta := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       segSize,
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
			Do(func(ctx context.Context, data io.Reader, expiration time.Time, info func() (storj.Path, []byte, error)) {
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

		streamStore, err := NewStreamStore(mockSegmentStore, segSize, newStore(), encBlockSize, dataCipher, inlineSize)
		if err != nil {
			t.Fatal(err)
		}

		meta, err := streamStore.Put(ctx, test.path, pathCipher, test.data, test.metadata, test.expiration)
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

	const (
		segSize      = 10
		inlineSize   = 5
		encBlockSize = 10
		dataCipher   = storj.EncNull
		pathCipher   = storj.EncAESGCM
	)

	mockSegmentStore := segments.NewMockStore(ctrl)

	staticTime := time.Now()

	segmentRanger := stubRanger{
		len:    10,
		closer: readCloserStub{},
	}

	stream, err := proto.Marshal(&pb.StreamInfo{
		NumberOfSegments: 1,
		SegmentsSize:     segSize,
		LastSegmentSize:  0,
	})
	if err != nil {
		t.Fatal(err)
	}

	lastSegmentMeta, err := proto.Marshal(&pb.StreamMeta{
		EncryptedStreamInfo: stream,
		EncryptionType:      int32(dataCipher),
		EncryptionBlockSize: encBlockSize,
	})
	if err != nil {
		t.Fatal(err)
	}

	segmentMeta := segments.Meta{
		Modified:   staticTime,
		Expiration: staticTime,
		Size:       segSize,
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

		streamStore, err := NewStreamStore(mockSegmentStore, segSize, newStore(), encBlockSize, dataCipher, inlineSize)
		if err != nil {
			t.Fatal(err)
		}

		ranger, meta, err := streamStore.Get(ctx, test.path, pathCipher)
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

		streamStore, err := NewStreamStore(mockSegmentStore, 10, newStore(), 10, 0, 0)
		if err != nil {
			t.Fatal(err)
		}

		err = streamStore.Delete(ctx, test.path, storj.EncAESGCM)
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

		streamStore, err := NewStreamStore(mockSegmentStore, 10, newStore(), 10, 0, 0)
		if err != nil {
			t.Fatal(err)
		}

		items, more, err := streamStore.List(ctx, test.prefix, test.startAfter, test.endBefore, storj.EncAESGCM, test.recursive, test.limit, test.metaFlags)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, test.streamItems, items, errTag)
		assert.Equal(t, test.streamMore, more, errTag)
	}
}
