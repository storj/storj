// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pdbclient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/stretchr/testify/assert"
	grpc "google.golang.org/grpc"

	p "storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
)

const (
	noPathGiven = "file path not given"
)

var (
	ctx            = context.Background()
	ErrNoFileGiven = errors.New(noPathGiven)
)

func TestNewPointerDBClient(t *testing.T) {
	// mocked grpcClient so we don't have
	// to call the network to test the code
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gc := NewMockPointerDBClient(ctrl)
	pdb := PointerDB{grpcClient: gc}

	assert.NotNil(t, pdb)
	assert.NotNil(t, pdb.grpcClient)
}

func makePointer(path p.Path, auth []byte) pb.PutRequest {
	// rps is an example slice of RemotePieces to add to this
	// REMOTE pointer type.
	var rps []*pb.RemotePiece
	rps = append(rps, &pb.RemotePiece{
		PieceNum: 1,
		NodeId:   "testId",
	})
	pr := pb.PutRequest{
		Path: path.String(),
		Pointer: &pb.Pointer{
			Type: pb.Pointer_REMOTE,
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					Type:             pb.RedundancyScheme_RS,
					MinReq:           1,
					Total:            3,
					RepairThreshold:  2,
					SuccessThreshold: 3,
				},
				PieceId:      "testId",
				RemotePieces: rps,
			},
			Size: int64(1),
		},
	}
	return pr
}

func TestPut(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		APIKey    []byte
		path      p.Path
		err       error
		errString string
	}{
		{[]byte("abc123"), p.New("file1/file2"), nil, ""},
		//{[]byte("wrong key"), p.New("file1/file2"), ErrUnauthenticated, unauthenticated},
		{[]byte("abc123"), p.New(""), ErrNoFileGiven, noPathGiven},
		//{[]byte("wrong key"), p.New(""), ErrUnauthenticated, unauthenticated},
		//{[]byte(""), p.New(""), ErrUnauthenticated, unauthenticated},
	} {
		putRequest := makePointer(tt.path, tt.APIKey)

		errTag := fmt.Sprintf("Test case #%d", i)
		gc := NewMockPointerDBClient(ctrl)
		pdb := PointerDB{grpcClient: gc}

		// here we don't care what type of context we pass
		gc.EXPECT().Put(gomock.Any(), &putRequest, gomock.Any()).Return(nil, tt.err)

		err := pdb.Put(ctx, tt.path, putRequest.Pointer)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		APIKey    []byte
		path      p.Path
		err       error
		errString string
	}{
		//{[]byte("wrong key"), p.New("file1/file2"), ErrUnauthenticated, unauthenticated},
		{[]byte("abc123"), p.New(""), ErrNoFileGiven, noPathGiven},
		//{[]byte("wrong key"), p.New(""), ErrUnauthenticated, unauthenticated},
		//{[]byte(""), p.New(""), ErrUnauthenticated, unauthenticated},
		{[]byte("abc123"), p.New("file1/file2"), nil, ""},
	} {
		getPointer := makePointer(tt.path, tt.APIKey)
		getRequest := pb.GetRequest{Path: tt.path.String()}

		data, err := proto.Marshal(getPointer.Pointer)
		if err != nil {
			log.Fatal("marshaling error: ", err)
		}

		byteData := data

		getResponse := pb.GetResponse{Pointer: byteData}

		errTag := fmt.Sprintf("Test case #%d", i)

		gc := NewMockPointerDBClient(ctrl)
		pdb := PointerDB{grpcClient: gc}

		gc.EXPECT().Get(gomock.Any(), &getRequest, gomock.Any()).Return(&getResponse, tt.err)

		pointer, err := pdb.Get(ctx, tt.path)

		if err != nil {
			assert.True(t, strings.Contains(err.Error(), tt.errString), errTag)
			assert.Nil(t, pointer)
		} else {
			assert.NotNil(t, pointer)
			assert.NoError(t, err, errTag)
		}
	}
}

func TestList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		prefix     string
		startAfter string
		endBefore  string
		recursive  bool
		limit      int
		metaFlags  uint32
		//APIKey     string
		items     []*pb.ListResponse_Item
		more      bool
		err       error
		errString string
	}{
		{"", "", "", false, 0, meta.None,
			[]*pb.ListResponse_Item{}, false, nil, ""},
		{"", "", "", false, 0, meta.None,
			[]*pb.ListResponse_Item{{}}, false, nil, ""},
		// {"", "", "", false, -1, meta.None,
		// 	[]*pb.ListResponse_Item{}, false, ErrUnauthenticated, unauthenticated},
		{"prefix", "after", "before", false, 1, meta.None,
			[]*pb.ListResponse_Item{
				{Path: "a/b/c"},
			},
			true, nil, ""},
		{"prefix", "after", "before", false, 1, meta.All,
			[]*pb.ListResponse_Item{
				{Path: "a/b/c", Pointer: &pb.Pointer{
					Size:           1234,
					CreationDate:   ptypes.TimestampNow(),
					ExpirationDate: ptypes.TimestampNow(),
				}},
			},
			true, nil, ""},
		{"some/prefix", "start/after", "end/before", true, 123, meta.Size,
			[]*pb.ListResponse_Item{
				{Path: "a/b/c", Pointer: &pb.Pointer{Size: 1234}},
				{Path: "x/y", Pointer: &pb.Pointer{Size: 789}},
			},
			true, nil, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		listRequest := pb.ListRequest{
			Prefix:     tt.prefix,
			StartAfter: tt.startAfter,
			EndBefore:  tt.endBefore,
			Recursive:  tt.recursive,
			Limit:      int32(tt.limit),
			MetaFlags:  tt.metaFlags,
		}

		listResponse := pb.ListResponse{Items: tt.items, More: tt.more}

		gc := NewMockPointerDBClient(ctrl)
		pdb := PointerDB{grpcClient: gc}

		gc.EXPECT().List(gomock.Any(), &listRequest, gomock.Any()).Return(&listResponse, tt.err)

		items, more, err := pdb.List(ctx, p.New(tt.prefix), p.New(tt.startAfter),
			p.New(tt.endBefore), tt.recursive, tt.limit, tt.metaFlags)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
			assert.False(t, more)
			assert.Nil(t, items)
		} else {
			assert.NoError(t, err, errTag)
			assert.Equal(t, tt.more, more)
			assert.NotNil(t, items)
			assert.Equal(t, len(tt.items), len(items))

			for i := 0; i < len(items); i++ {
				assert.Equal(t, tt.items[i].GetPath(), items[i].Path.String())
				assert.Equal(t, tt.items[i].GetPointer().GetSize(),
					items[i].Pointer.GetSize())
				assert.Equal(t, tt.items[i].GetPointer().GetCreationDate(),
					items[i].Pointer.GetCreationDate())
				assert.Equal(t, tt.items[i].GetPointer().GetExpirationDate(),
					items[i].Pointer.GetExpirationDate())
			}
		}
	}
}

func TestDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		path      p.Path
		err       error
		errString string
	}{
		{p.New(""), ErrNoFileGiven, noPathGiven},
		{p.New("file1/file2"), nil, ""},
	} {
		deleteRequest := pb.DeleteRequest{Path: tt.path.String()}

		errTag := fmt.Sprintf("Test case #%d", i)
		gc := NewMockPointerDBClient(ctrl)
		pdb := PointerDB{grpcClient: gc}

		gc.EXPECT().Delete(gomock.Any(), &deleteRequest, gomock.Any()).Return(nil, tt.err)

		err := pdb.Delete(ctx, tt.path)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestApiKeyInjector(t *testing.T) {
	for _, tt := range []struct {
		APIKey string
		err    error
	}{
		{"abc123", nil},
		{"", nil},
	} {
		injector := apiKeyInjector(tt.APIKey)

		// mock for method invoker
		var outputCtx context.Context
		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			outputCtx = ctx
			return nil
		}

		ctx := context.Background()
		err := injector(ctx, "/test.method", nil, nil, nil, invoker)

		assert.Equal(t, err, tt.err)
		assert.Equal(t, tt.APIKey, metautils.ExtractOutgoing(outputCtx).Get("apikey"))
	}
}
