// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storage/meta"
	pb "storj.io/storj/protos/pointerdb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

//go:generate mockgen -destination kvstore_mock_test.go -package pointerdb storj.io/storj/storage KeyValueStore

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
		{[]byte("wrong key"), nil, status.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
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
		{[]byte("wrong key"), nil, status.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
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
		{[]byte("wrong key"), nil, status.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
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
	db := teststore.New()
	server := Server{DB: db, logger: zap.NewNop()}

	key := func(s string) storage.Key {
		return storage.Key(paths.New(s).Bytes())
	}

	pointer := &pb.Pointer{}
	pointer.CreationDate = ptypes.TimestampNow()

	pointerBytes, err := proto.Marshal(pointer)
	if err != nil {
		t.Fatal(err)
	}
	pointerValue := storage.Value(pointerBytes)

	err = storage.PutAll(db, []storage.ListItem{
		{Key: key("sample.😶"), Value: pointerValue},
		{Key: key("müsic"), Value: pointerValue},
		{Key: key("müsic/söng1.mp3"), Value: pointerValue},
		{Key: key("müsic/söng2.mp3"), Value: pointerValue},
		{Key: key("müsic/album/söng3.mp3"), Value: pointerValue},
		{Key: key("müsic/söng4.mp3"), Value: pointerValue},
		{Key: key("ビデオ/movie.mkv"), Value: pointerValue},
	}...)
	if err != nil {
		t.Fatal(err)
	}

	type Test struct {
		Request  pb.ListRequest
		Expected *pb.ListResponse
		Error    func(i int, err error)
	}

	errorWithCode := func(code codes.Code) func(i int, err error) {
		t.Helper()
		return func(i int, err error) {
			t.Helper()
			if status.Code(err) != code {
				t.Fatalf("%d: should fail with %v, got: %v", i, code, err)
			}
		}
	}

	tests := []Test{
		{
			Request: pb.ListRequest{Recursive: true},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic"},
					{Path: "müsic/album/söng3.mp3"},
					{Path: "müsic/söng1.mp3"},
					{Path: "müsic/söng2.mp3"},
					{Path: "müsic/söng4.mp3"},
					{Path: "sample.😶"},
					{Path: "ビデオ/movie.mkv"},
				},
			},
		}, {
			Request: pb.ListRequest{Recursive: true, MetaFlags: meta.All},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic", Pointer: pointer},
					{Path: "müsic/album/söng3.mp3", Pointer: pointer},
					{Path: "müsic/söng1.mp3", Pointer: pointer},
					{Path: "müsic/söng2.mp3", Pointer: pointer},
					{Path: "müsic/söng4.mp3", Pointer: pointer},
					{Path: "sample.😶", Pointer: pointer},
					{Path: "ビデオ/movie.mkv", Pointer: pointer},
				},
			},
		}, {
			Request: pb.ListRequest{Recursive: true, MetaFlags: meta.All, APIKey: []byte("wrong key")},
			Error:   errorWithCode(codes.Unauthenticated),
		}, {
			Request: pb.ListRequest{Recursive: true, Limit: 3},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic"},
					{Path: "müsic/album/söng3.mp3"},
					{Path: "müsic/söng1.mp3"},
				},
				More: true,
			},
		}, {
			Request: pb.ListRequest{MetaFlags: meta.All},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic", Pointer: pointer},
					{Path: "müsic/", IsPrefix: true},
					{Path: "sample.😶", Pointer: pointer},
					{Path: "ビデオ/", IsPrefix: true},
				},
				More: false,
			},
		}, {
			Request: pb.ListRequest{EndBefore: "ビデオ"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic"},
					{Path: "müsic/", IsPrefix: true},
					{Path: "sample.😶"},
				},
				More: false,
			},
		}, {
			Request: pb.ListRequest{Recursive: true, Prefix: "müsic/"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic/album/söng3.mp3"},
					{Path: "müsic/söng1.mp3"},
					{Path: "müsic/söng2.mp3"},
					{Path: "müsic/söng4.mp3"},
				},
			},
		}, {
			Request: pb.ListRequest{Recursive: true, Prefix: "müsic/", StartAfter: "album/söng3.mp3"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic/söng1.mp3"},
					{Path: "müsic/söng2.mp3"},
					{Path: "müsic/söng4.mp3"},
				},
			},
		}, {
			Request: pb.ListRequest{Prefix: "müsic/"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic/album/", IsPrefix: true},
					{Path: "müsic/söng1.mp3"},
					{Path: "müsic/söng2.mp3"},
					{Path: "müsic/söng4.mp3"},
				},
			},
		}, {
			Request: pb.ListRequest{Prefix: "müsic/", StartAfter: "söng1.mp3"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic/söng2.mp3"},
					{Path: "müsic/söng4.mp3"},
				},
			},
		}, {
			Request: pb.ListRequest{Prefix: "müsic/", EndBefore: "söng4.mp3"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic/album/", IsPrefix: true},
					{Path: "müsic/söng1.mp3"},
					{Path: "müsic/söng2.mp3"},
				},
			},
		}, {
			Request: pb.ListRequest{Prefix: "müs", Recursive: true, EndBefore: "ic/söng4.mp3", Limit: 1},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					// {Path: "ic/söng2.mp3"},
				},
				// More: true,
			},
		},
	}

	// TODO:
	//    pb.ListRequest{Prefix: "müsic/", StartAfter: "söng1.mp3", EndBefore: "söng4.mp3"},
	//    failing database
	for i, test := range tests {
		resp, err := server.List(ctx, &test.Request)
		if test.Error == nil {
			if err != nil {
				t.Fatalf("%d: failed %v", i, err)
			}
		} else {
			test.Error(i, err)
		}

		if diff := cmp.Diff(test.Expected, resp, cmp.Comparer(proto.Equal)); diff != "" {
			t.Errorf("%d: (-want +got) %v\n%s", i, test.Request.String(), diff)
		}
	}
}
