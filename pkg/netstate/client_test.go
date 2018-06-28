// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/internal/test"
	pb "storj.io/storj/protos/netstate"
)

func TestNetStateClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 9000))
	assert.NoError(t, err)

	mdb := test.NewMockKeyValueStore(test.KvStore{})

	grpcServer := grpc.NewServer()
	pb.RegisterNetStateServer(grpcServer, NewServer(mdb, logger))

	defer grpcServer.GracefulStop()
	go grpcServer.Serve(lis)

	address := lis.Addr().String()
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	assert.NoError(t, err)

	c := pb.NewNetStateClient(conn)

	ctx := context.Background()

	// example file path to put/get
	pr1 := pb.PutRequest{
		Path: []byte("here/is/a/path"),
		Pointer: &pb.Pointer{
			Type:          pb.Pointer_INLINE,
			InlineSegment: []byte("oatmeal"),
		},
		APIKey: []byte("abc123"),
	}

	// Tests Server.Put
	_, err = c.Put(ctx, &pr1)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to Put")
	}

	if mdb.PutCalled != 1 {
		t.Error("Failed to call mockdb correctly")
	}

	pointerBytes, err := proto.Marshal(pr1.Pointer)
	if err != nil {
		t.Error("failed to marshal test pointer")
	}

	if !bytes.Equal(mdb.Data[string(pr1.Path)], pointerBytes) {
		t.Error("Expected saved pointer to equal given pointer")
	}

	// Tests Server.Get
	getReq := pb.GetRequest{
		Path:   []byte("here/is/a/path"),
		APIKey: []byte("abc123"),
	}

	getRes, err := c.Get(ctx, &getReq)
	assert.NoError(t, err)

	if !bytes.Equal(getRes.Pointer, pointerBytes) {
		t.Error("Expected to get same content that was put")
	}

	if mdb.GetCalled != 1 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Puts another pointer entry to test delete and list
	pr2 := pb.PutRequest{
		Path: []byte("here/is/another/path"),
		Pointer: &pb.Pointer{
			Type:          pb.Pointer_INLINE,
			InlineSegment: []byte("raisins"),
		},
		APIKey: []byte("abc123"),
	}

	_, err = c.Put(ctx, &pr2)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to Put")
	}

	if mdb.PutCalled != 2 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Test Server.Delete
	delReq := pb.DeleteRequest{
		Path:   []byte("here/is/a/path"),
		APIKey: []byte("abc123"),
	}

	_, err = c.Delete(ctx, &delReq)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to delete")
	}

	if mdb.DeleteCalled != 1 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Tests Server.List
	listReq := pb.ListRequest{
		// This pagination functionality doesn't work yet.
		// The given arguments are placeholders.
		StartingPathKey: []byte("test/pointer/path"),
		Limit:           5,
		APIKey:          []byte("abc123"),
	}

	listRes, err := c.List(ctx, &listReq)
	if err != nil {
		t.Error("Failed to list file paths")
	}

	if !bytes.Equal(listRes.Paths[0], []byte("here/is/another/path")) {
		t.Error("Failed to list correct file path")
	}

	if mdb.ListCalled != 1 {
		t.Error("Failed to call mockdb correct number of times")
	}
}
