// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storage/meta"
	pb "storj.io/storj/protos/pointerdb"
	"storj.io/storj/storage"
)

var (
	ctx = context.Background()
)

func TestServicePut(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		apiKey    []byte
		err       error
		errString string
	}{
		{nil, nil, ""},
		{[]byte("wrong key"), nil, grpc.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{nil, errors.New("put error"), status.Errorf(codes.Internal, "put error").Error()},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		db := NewMockKeyValueStore(ctrl)
		s := Server{DB: db, logger: zap.NewNop()}

		path := "a/b/c"
		pr := pb.Pointer{}

		if tt.err != nil || tt.errString == "" {
			db.EXPECT().Put(storage.Key([]byte(path)), gomock.Any()).Return(tt.err)
		}

		req := pb.PutRequest{Path: path, Pointer: &pr, APIKey: tt.apiKey}
		_, err := s.Put(ctx, &req)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestServiceGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		apiKey    []byte
		err       error
		errString string
	}{
		{nil, nil, ""},
		{[]byte("wrong key"), nil, grpc.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{nil, errors.New("get error"), status.Errorf(codes.Internal, "get error").Error()},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		db := NewMockKeyValueStore(ctrl)
		s := Server{DB: db, logger: zap.NewNop()}

		path := "a/b/c"
		pr := pb.Pointer{}
		prBytes, err := proto.Marshal(&pr)
		assert.NoError(t, err, errTag)

		if tt.err != nil || tt.errString == "" {
			db.EXPECT().Get(storage.Key([]byte(path))).Return(prBytes, tt.err)
		}

		req := pb.GetRequest{Path: path, APIKey: tt.apiKey}
		resp, err := s.Get(ctx, &req)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
			respPr := pb.Pointer{}
			err := proto.Unmarshal(resp.GetPointer(), &respPr)
			assert.NoError(t, err, errTag)
			assert.Equal(t, pr, respPr, errTag)
		}
	}
}

func TestServiceDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		apiKey    []byte
		err       error
		errString string
	}{
		{nil, nil, ""},
		{[]byte("wrong key"), nil, grpc.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{nil, errors.New("delete error"), status.Errorf(codes.Internal, "delete error").Error()},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		db := NewMockKeyValueStore(ctrl)
		s := Server{DB: db, logger: zap.NewNop()}

		path := "a/b/c"

		if tt.err != nil || tt.errString == "" {
			db.EXPECT().Delete(storage.Key([]byte(path))).Return(tt.err)
		}

		req := pb.DeleteRequest{Path: path, APIKey: tt.apiKey}
		_, err := s.Delete(ctx, &req)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestServiceList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	keys := storage.Keys{
		storage.Key(paths.New("sample.jpg").Bytes()),
		storage.Key(paths.New("music").Bytes()),
		storage.Key(paths.New("music/song1.mp3").Bytes()),
		storage.Key(paths.New("music/song2.mp3").Bytes()),
		storage.Key(paths.New("music/album/song3.mp3").Bytes()),
		storage.Key(paths.New("music/song4.mp3").Bytes()),
		storage.Key(paths.New("videos").Bytes()),
		storage.Key(paths.New("videos/movie.mkv").Bytes()),
	}

	items := storage.Items{}

	for _, key := range keys {

		item := storage.ListItem{
			Key:      key,
			Value:    storage.Value([]byte("value")),
			IsPrefix: false,
		}
		items = append(items, item)
	}

	keyList := items.GetKeys()

	for i, tt := range []struct {
		prefix        string
		startAfter    string
		endBefore     string
		recursive     bool
		limit         int
		metaFlags     uint32
		apiKey        []byte
		returnedItems storage.Items
		expectedKeys  storage.Keys
		expectedMore  storage.More
		err           error
		errString     string
	}{
		{"", "", "", true, 0, meta.None, nil, items, keyList, false, nil, ""},
		{"", "", "", true, 0, meta.All, nil, items, keyList, false, nil, ""},
		{"", "", "", true, 0, meta.None, []byte("wrong key"), items, keyList, false,
			nil, grpc.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{"", "", "", true, 0, meta.None, nil, items, keyList, false,
			errors.New("list error"), status.Errorf(codes.Internal, "list error").Error()},
		{"", "", "", true, 2, meta.None, nil, items, keyList[:2], true, nil, ""},
		{"", "", "", false, 0, meta.None, nil, items, storage.Keys{keyList[0], keyList[1], keyList[6]}, false, nil, ""},
		{"", "", "videos", false, 0, meta.None, nil, items, keyList[:2], false, nil, ""},
		{"music", "", "", false, 0, meta.None, nil, items[2:], storage.Keys{keyList[2], keyList[3], keyList[5]}, false, nil, ""},
		{"music", "", "", true, 0, meta.None, nil, items[2:], keyList[2:6], false, nil, ""},
		{"music", "song1.mp3", "", true, 0, meta.None, nil, items, keyList[3:6], false, nil, ""},
		{"music", "song1.mp3", "album/song3.mp3", true, 0, meta.None, nil, items, keyList[3:4], false, nil, ""},
		{"music", "", "song4.mp3", true, 0, meta.None, nil, items, keyList[2:5], false, nil, ""},
		{"music", "", "song4.mp3", true, 1, meta.None, nil, items, keyList[4:5], true, nil, ""},
		{"music", "", "song4.mp3", false, 0, meta.None, nil, items, keyList[2:4], false, nil, ""},
		{"music", "song2.mp3", "song4.mp3", true, 0, meta.None, nil, items, keyList[4:5], false, nil, ""},
		{"mus", "", "", true, 0, meta.None, nil, items[1:], nil, false, nil, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		db := NewMockKeyValueStore(ctrl)
		s := Server{DB: db, logger: zap.NewNop()}

		if tt.err != nil || tt.errString == "" {
			prefix := storage.Key([]byte(tt.prefix + "/"))

			opts := storage.ListOptions{
				Start: prefix,
				Limit: 0,
			}

			db.EXPECT().List(opts).Return(tt.returnedItems, tt.expectedMore, tt.err)
			keys := tt.returnedItems.GetKeys()

			if tt.metaFlags != meta.None {
				pr := pb.Pointer{}
				b, err := proto.Marshal(&pr)
				assert.NoError(t, err, errTag)
				for _, key := range keys {
					db.EXPECT().Get(key).Return(b, nil)
				}
			}
		}

		req := pb.ListRequest{
			Prefix:     tt.prefix,
			StartAfter: tt.startAfter,
			EndBefore:  tt.endBefore,
			Recursive:  tt.recursive,
			Limit:      int32(tt.limit),
			MetaFlags:  tt.metaFlags,
			APIKey:     tt.apiKey,
		}
		resp, err := s.List(ctx, &req)

		var getMore storage.More

		// assures same types are gtting compared
		getMore = storage.More(resp.GetMore())

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
			assert.Equal(t, tt.expectedMore, getMore, errTag)
			assert.Equal(t, len(tt.expectedKeys), len(resp.GetItems()), errTag)
			for i, item := range resp.GetItems() {
				assert.Equal(t, tt.expectedKeys[i].String(), item.Path, errTag)
			}
		}
	}
}
