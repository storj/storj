// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/paths"
	meta "storj.io/storj/pkg/storage"
	pb "storj.io/storj/protos/pointerdb"
	"storj.io/storj/storage"
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
		storage.Key(paths.New("music/song1.mp3").Bytes()),
		storage.Key(paths.New("music/song2.mp3").Bytes()),
		storage.Key(paths.New("music/album/song3.mp3").Bytes()),
		storage.Key(paths.New("music/song4.mp3").Bytes()),
		storage.Key(paths.New("videos/movie.mkv").Bytes()),
	}

	for i, tt := range []struct {
		prefix       string
		startAfter   string
		endBefore    string
		recursive    bool
		limit        int
		metaFlags    uint64
		apiKey       []byte
		returnedKeys storage.Keys
		expectedKeys storage.Keys
		expectedMore bool
		err          error
		errString    string
	}{
		{"", "", "", true, 0, meta.MetaNone, nil, keys, keys, false, nil, ""},
		{"", "", "", true, 0, meta.MetaAll, nil, keys, keys, false, nil, ""},
		{"", "", "", true, 0, meta.MetaNone, []byte("wrong key"), keys, keys, false,
			nil, grpc.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{"", "", "", true, 0, meta.MetaNone, nil, keys, keys, false,
			errors.New("list error"), status.Errorf(codes.Internal, "list error").Error()},
		{"", "", "", true, 2, meta.MetaNone, nil, keys, keys[:2], true, nil, ""},
		{"", "", "", false, 0, meta.MetaNone, nil, keys, keys[:1], false, nil, ""},
		{"music", "", "", false, 0, meta.MetaNone, nil, keys[1:], storage.Keys{keys[1], keys[2], keys[4]}, false, nil, ""},
		{"music", "", "", true, 0, meta.MetaNone, nil, keys[1:], keys[1:5], false, nil, ""},
		{"music", "song1.mp3", "", true, 0, meta.MetaNone, nil, keys, keys[2:5], false, nil, ""},
		{"music", "song1.mp3", "album/song3.mp3", true, 0, meta.MetaNone, nil, keys, keys[2:3], false, nil, ""},
		{"music", "", "song4.mp3", true, 0, meta.MetaNone, nil, keys, keys[1:4], false, nil, ""},
		{"music", "", "song4.mp3", true, 1, meta.MetaNone, nil, keys, keys[3:4], true, nil, ""},
		{"music", "", "song4.mp3", false, 0, meta.MetaNone, nil, keys, keys[1:3], false, nil, ""},
		{"music", "song2.mp3", "song4.mp3", true, 0, meta.MetaNone, nil, keys, keys[3:4], false, nil, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		db := NewMockKeyValueStore(ctrl)
		s := Server{DB: db, logger: zap.NewNop()}

		if tt.err != nil || tt.errString == "" {
			prefix := storage.Key(paths.New(tt.prefix).Bytes())
			db.EXPECT().List(prefix, storage.Limit(0)).Return(tt.returnedKeys, tt.err)

			if tt.metaFlags != meta.MetaNone {
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

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
			assert.Equal(t, tt.expectedMore, resp.GetMore(), errTag)
			assert.Equal(t, len(tt.expectedKeys), len(resp.GetItems()), errTag)
			for i, item := range resp.GetItems() {
				assert.Equal(t, tt.expectedKeys[i].String(), item.Path, errTag)
			}
		}
	}
}
