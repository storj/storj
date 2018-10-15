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

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

func TestServicePut(t *testing.T) {
	for i, tt := range []struct {
		apiKey    []byte
		err       error
		errString string
	}{
		{nil, nil, ""},
		{[]byte("wrong key"), nil, status.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{nil, errors.New("put error"), status.Errorf(codes.Internal, "internal error").Error()},
	} {
		ctx := context.Background()
		ctx = auth.WithAPIKey(ctx, tt.apiKey)

		errTag := fmt.Sprintf("Test case #%d", i)

		db := teststore.New()
		s := Server{DB: db, logger: zap.NewNop()}

		path := "a/b/c"
		pr := pb.Pointer{}

		if tt.err != nil {
			db.ForceError++
		}

		req := pb.PutRequest{Path: path, Pointer: &pr}
		_, err := s.Put(ctx, &req)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestServiceGet(t *testing.T) {
	for i, tt := range []struct {
		apiKey    []byte
		err       error
		errString string
	}{
		{nil, nil, ""},
		{[]byte("wrong key"), nil, status.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{nil, errors.New("get error"), status.Errorf(codes.Internal, "internal error").Error()},
	} {
		ctx := context.Background()
		ctx = auth.WithAPIKey(ctx, tt.apiKey)

		errTag := fmt.Sprintf("Test case #%d", i)

		db := teststore.New()
		s := Server{DB: db, logger: zap.NewNop()}

		path := "a/b/c"

		pr := &pb.Pointer{Size: 123}
		prBytes, err := proto.Marshal(pr)
		assert.NoError(t, err, errTag)

		_ = db.Put(storage.Key(path), storage.Value(prBytes))

		if tt.err != nil {
			db.ForceError++
		}

		req := pb.GetRequest{Path: path}
		resp, err := s.Get(ctx, &req)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
			assert.NoError(t, err, errTag)
			assert.True(t, proto.Equal(pr, resp.Pointer), errTag)
		}
	}
}

func TestServiceDelete(t *testing.T) {
	for i, tt := range []struct {
		apiKey    []byte
		err       error
		errString string
	}{
		{nil, nil, ""},
		{[]byte("wrong key"), nil, status.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{nil, errors.New("delete error"), status.Errorf(codes.Internal, "internal error").Error()},
	} {
		ctx := context.Background()
		ctx = auth.WithAPIKey(ctx, tt.apiKey)

		errTag := fmt.Sprintf("Test case #%d", i)

		path := "a/b/c"

		db := teststore.New()
		_ = db.Put(storage.Key(path), storage.Value("hello"))
		s := Server{DB: db, logger: zap.NewNop()}

		if tt.err != nil {
			db.ForceError++
		}

		req := pb.DeleteRequest{Path: path}
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
		APIKey   string
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
			APIKey:  "wrong key",
			Request: pb.ListRequest{Recursive: true, MetaFlags: meta.All}, //, APIKey: []byte("wrong key")},
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
					{Path: "album/söng3.mp3"},
					{Path: "söng1.mp3"},
					{Path: "söng2.mp3"},
					{Path: "söng4.mp3"},
				},
			},
		}, {
			Request: pb.ListRequest{Recursive: true, Prefix: "müsic/", StartAfter: "album/söng3.mp3"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "söng1.mp3"},
					{Path: "söng2.mp3"},
					{Path: "söng4.mp3"},
				},
			},
		}, {
			Request: pb.ListRequest{Prefix: "müsic/"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "album/", IsPrefix: true},
					{Path: "söng1.mp3"},
					{Path: "söng2.mp3"},
					{Path: "söng4.mp3"},
				},
			},
		}, {
			Request: pb.ListRequest{Prefix: "müsic/", StartAfter: "söng1.mp3"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "söng2.mp3"},
					{Path: "söng4.mp3"},
				},
			},
		}, {
			Request: pb.ListRequest{Prefix: "müsic/", EndBefore: "söng4.mp3"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "album/", IsPrefix: true},
					{Path: "söng1.mp3"},
					{Path: "söng2.mp3"},
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
		ctx := context.Background()
		ctx = auth.WithAPIKey(ctx, []byte(test.APIKey))

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
