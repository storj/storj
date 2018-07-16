// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"fmt"
	"errors"
	"testing"
	"log"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	pb "storj.io/storj/protos/pointerdb"
	p "storj.io/storj/pkg/paths"
)

const (
	unauthenticated = "failed API creds"
	noPathGiven = "file path not given"
	noLimitGiven = "limit not given"
)

var (
	ctx = context.Background()
	ErrUnauthenticated = errors.New(unauthenticated)
	ErrNoFileGiven = errors.New(noPathGiven)
	ErrNoLimitGiven = errors.New(noLimitGiven)
)

func TestNewPointerDBClient(t *testing.T) {
	// mocked grpcClient so we don't have
	// to call the network to test the code
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gc:= NewMockPointerDBClient(ctrl)
	pdb := PointerDB{grpcClient: gc}

	assert.NotNil(t, pdb)
	assert.NotNil(t, pdb.grpcClient)
}

func makePointer(path p.Path, auth []byte) pb.PutRequest {
	// rps is an example slice of RemotePieces to add to this
	// REMOTE pointer type.
	var rps []*pb.RemotePiece
	rps = append(rps, &pb.RemotePiece{
		PieceNum: int64(1),
		NodeId:   "testId",
	})
	pr := pb.PutRequest{
		Path: path.Bytes(),
		Pointer: &pb.Pointer{
			Type: pb.Pointer_REMOTE,
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					Type:             pb.RedundancyScheme_RS,
					MinReq:           int64(1),
					Total:            int64(3),
					RepairThreshold:  int64(2),
					SuccessThreshold: int64(3),
				},
				PieceId:      "testId",
				RemotePieces: rps,
			},
			Size: int64(1),
		},
		APIKey: auth,
	}
	return pr
}

func TestPut(t *testing.T){
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		APIKey []byte
		path p.Path
		err error 
		errString string
	}{
		{[]byte("abc123"), p.New("file1/file2"), nil, ""},
		{[]byte("wrong key"), p.New("file1/file2"), ErrUnauthenticated,unauthenticated},
		{[]byte("abc123"), p.New(""), ErrNoFileGiven, noPathGiven},
		{[]byte("wrong key"), p.New(""), ErrUnauthenticated, unauthenticated},
		{[]byte(""), p.New(""), ErrUnauthenticated, unauthenticated},
	}{
		putRequest:= makePointer(tt.path, tt.APIKey)

		errTag := fmt.Sprintf("Test case #%d", i)
		gc:= NewMockPointerDBClient(ctrl)
		pdb := PointerDB{grpcClient: gc}

		// here we don't care what type of context we pass
		gc.EXPECT().Put(gomock.Any(), &putRequest).Return(nil, tt.err)

		err := pdb.Put(ctx, tt.path, putRequest.Pointer, tt.APIKey)
		
		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestGet(t *testing.T){
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		APIKey []byte
		path p.Path
		err error 
		errString string
	}{
		{[]byte("wrong key"), p.New("file1/file2"), ErrUnauthenticated,unauthenticated},
		{[]byte("abc123"), p.New(""), ErrNoFileGiven, noPathGiven},
		{[]byte("wrong key"), p.New(""), ErrUnauthenticated, unauthenticated},
		{[]byte(""), p.New(""), ErrUnauthenticated, unauthenticated},
		{[]byte("abc123"), p.New("file1/file2"), nil, ""},
	}{
		getPointer := makePointer(tt.path, tt.APIKey)
		getRequest:= pb.GetRequest{Path: tt.path.Bytes(), APIKey: tt.APIKey}
		
		data, err := proto.Marshal(getPointer.Pointer)
		if err != nil {
			log.Fatal("marshaling error: ", err)
		}

		byteData := []byte(data)

		getResponse := pb.GetResponse{Pointer: byteData}

		errTag := fmt.Sprintf("Test case #%d", i)
		
		gc:= NewMockPointerDBClient(ctrl)
		pdb := PointerDB{grpcClient: gc}

		gc.EXPECT().Get(gomock.Any(), &getRequest).Return(&getResponse, tt.err)

		pointer, err := pdb.Get(ctx, tt.path, tt.APIKey)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
			assert.Nil(t, pointer)
		} else {
			assert.NotNil(t, pointer)
			assert.NoError(t, err, errTag)
		}
	}
}

func TestList(t *testing.T){
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		APIKey []byte
		startingPath p.Path
		limit int64 
		truncated bool
		paths []string
		err error 
		errString string
	}{
		{[]byte("wrong key"), p.New(""), 2, true, []string{""}, ErrUnauthenticated, unauthenticated},
		{[]byte("abc123"), p.New("file1"), 2, true, []string{"test"},  nil, ""},
		{[]byte("abc123"), p.New(""), 2, true, []string{"file1/file2", "file3/file4", "file1", "file1/file2/great3", "test"},  ErrNoFileGiven, noPathGiven},
		{[]byte("abc123"), p.New("file1"), 2, false, []string{""},  nil, ""},
		{[]byte("wrong key"), p.New("file1"), 2, true, []string{"file1/file2", "file3/file4", "file1", "file1/file2/great3", "test"}, ErrUnauthenticated,unauthenticated},
		{[]byte("abc123"), p.New("file1"), 3, true, []string{"file1/file2", "file3/file4", "file1", "file1/file2/great3", "test"},  nil, ""},
		{[]byte("abc123"), p.New("file1"), 0, true, []string{"file1/file2", "file3/file4", "file1", "file1/file2/great3", "test"},  ErrNoLimitGiven, noLimitGiven},
	}{
		listRequest := pb.ListRequest{
			StartingPathKey: tt.startingPath.Bytes(),
			Limit:           tt.limit,
			APIKey:          tt.APIKey,
		}

		var truncatedPathsBytes [][]byte

		getCorrectPaths := func(fileName string) bool { return strings.HasPrefix(fileName, "file1")}
		filterPaths := filterPathName(tt.paths, getCorrectPaths)
		
		if len(filterPaths) == 0 {
			truncatedPathsBytes = [][]byte{}
		} else{
			truncatedPaths := filterPaths[0:tt.limit]
			truncatedPathsBytes := make([][]byte, len(truncatedPaths))
		
			for i, pathName := range truncatedPaths {
				bytePathName := []byte(pathName)
				truncatedPathsBytes[i] = bytePathName
			}
		}
			
		listResponse := pb.ListResponse{Paths: truncatedPathsBytes, Truncated: tt.truncated }

		errTag := fmt.Sprintf("Test case #%d", i)

		gc:= NewMockPointerDBClient(ctrl)
		pdb := PointerDB{grpcClient: gc}

		gc.EXPECT().List(gomock.Any(), &listRequest).Return(&listResponse, tt.err)

		paths, trunc, err := pdb.List(ctx, tt.startingPath, tt.limit, tt.APIKey)
		
		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
			assert.NotNil(t, trunc)
			assert.Nil(t, paths)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func filterPathName(pathString []string, test func(string) bool) (filteredPathNames []string) {
	for _, name := range pathString{
		if test(name) {
			filteredPathNames = append(filteredPathNames, name)
		}
	}
	return
}

func TestDelete(t *testing.T){
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		APIKey []byte
		path p.Path
		err error 
		errString string
	}{
		{[]byte("wrong key"), p.New("file1/file2"), ErrUnauthenticated,unauthenticated},
		{[]byte("abc123"), p.New(""), ErrNoFileGiven, noPathGiven},
		{[]byte("wrong key"), p.New(""), ErrUnauthenticated, unauthenticated},
		{[]byte(""), p.New(""), ErrUnauthenticated, unauthenticated},
		{[]byte("abc123"), p.New("file1/file2"), nil, ""},
	}{
		deleteRequest:= pb.DeleteRequest{Path: tt.path.Bytes(), APIKey: tt.APIKey}

		errTag := fmt.Sprintf("Test case #%d", i)
		gc:= NewMockPointerDBClient(ctrl)
		pdb := PointerDB{grpcClient: gc}

		gc.EXPECT().Delete(gomock.Any(), &deleteRequest).Return(nil, tt.err)
		
		err := pdb.Delete(ctx, tt.path, tt.APIKey)
		
		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}
