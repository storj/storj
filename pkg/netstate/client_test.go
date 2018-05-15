// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	proto "storj.io/storj/protos/netstate"
)

func TestNetStateClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 9000))
	assert.NoError(t, err)

	mdb := &mockDB{
		timesCalled: 0,
	}

	grpcServer := grpc.NewServer()
	proto.RegisterNetStateServer(grpcServer, NewServer(mdb, logger))

	defer grpcServer.GracefulStop()
	go grpcServer.Serve(lis)

	address := lis.Addr().String()
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	assert.NoError(t, err)

	c := proto.NewNetStateClient(conn)

	ctx := context.Background()

	// example file path to put/get
	fp := proto.FilePath{
		Path:       []byte("here/is/a/path"),
		SmallValue: []byte("oatmeal"),
	}

	if mdb.timesCalled != 0 {
		t.Error("Expected mockdb to be called 0 times")
	}

	// Tests Server.Put
	putRes, err := c.Put(ctx, &fp)
	assert.NoError(t, err)

	if putRes.Confirmation != "success" {
		t.Error("Failed to receive success Put response")
	}

	if mdb.timesCalled != 1 {
		t.Error("Failed to call mockdb correctly")
	}

	if !bytes.Equal(mdb.puts[0].Path, fp.Path) {
		t.Error("Expected saved path to equal given path")
	}

	if !bytes.Equal(mdb.puts[0].Value, fp.SmallValue) {
		t.Error("Expected saved value to equal given value")
	}

	// Tests Server.Get
	getReq := proto.GetRequest{
		Path: []byte("here/is/a/path"),
	}

	getRes, err := c.Get(ctx, &getReq)
	assert.NoError(t, err)

	if !bytes.Equal(getRes.SmallValue, fp.SmallValue) {
		t.Error("Expected to get same content that was put")
	}

	if mdb.timesCalled != 2 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Puts another file path to test delete and list
	fp2 := proto.FilePath{
		Path:       []byte("here/is/another/path"),
		SmallValue: []byte("raisins"),
	}

	putRes2, err := c.Put(ctx, &fp2)
	assert.NoError(t, err)

	if putRes2.Confirmation != "success" {
		t.Error("Failed to receive success Put response")
	}

	if mdb.timesCalled != 3 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Test Server.Delete
	delReq := proto.DeleteRequest{
		Path: []byte("here/is/a/path"),
	}

	delRes, err := c.Delete(ctx, &delReq)
	if err != nil {
		t.Error("Failed to delete file path")
	}

	if delRes.Confirmation != "success" {
		t.Error("Failed to receive success delete response")
	}

	if mdb.timesCalled != 4 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Tests Server.List
	listReq := proto.ListRequest{
		Bucket: []byte("files"),
	}

	listRes, err := c.List(ctx, &listReq)
	if err != nil {
		t.Error("Failed to list file paths")
	}

	if !bytes.Equal(listRes.Filepaths[0], []byte("here/is/another/path")) {
		t.Error("Failed to list correct file path")
	}

	if mdb.timesCalled != 5 {
		t.Error("Failed to call mockdb correct number of times")
	}
}
