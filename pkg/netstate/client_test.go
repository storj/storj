// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"bytes"
	"context"
	"net"
	"fmt"
	"testing"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "storj.io/storj/protos/netstate"
)

func TestNetStateClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	fmt.Println("in clinet: ", os.Getenv("API_KEY"))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 9000))
	assert.NoError(t, err)

	mdb := &MockDB{
		timesCalled: 0,
	}

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
<<<<<<< HEAD
	pr1 := pb.PutRequest{
		Path: []byte("here/is/a/path"),
		Pointer: &pb.Pointer{
			Type: pb.Pointer_INLINE,
			Encryption: &pb.EncryptionScheme{
				EncryptedEncryptionKey: []byte("key"),
				EncryptedStartingNonce: []byte("nonce"),
			},
			InlineSegment: []byte("oatmeal"),
		},
=======
	fp := proto.FilePath{
		Path:       []byte("here/is/a/path"),
		SmallValue: []byte("oatmeal"),
>>>>>>> work in progress for modifying test suite to accomodate credentials
		XApiKey: []byte("abc123"),
	}

	if mdb.timesCalled != 0 {
		t.Error("Expected mockdb to be called 0 times")
	}

	// Tests Server.Put
	_, err = c.Put(ctx, &pr1)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to Put")
	}

	if mdb.timesCalled != 1 {
		t.Error("Failed to call mockdb correctly")
	}

	if !bytes.Equal(mdb.puts[0].Path, pr1.Path) {
		t.Error("Expected saved path to equal given path")
	}

	pointerBytes, err := proto.Marshal(pr1.Pointer)
	if err != nil {
		t.Error("failed to marshal test pointer")
	}

	if !bytes.Equal(mdb.puts[0].Pointer, pointerBytes) {
		t.Error("Expected saved value to equal given value")
	}

<<<<<<< HEAD
<<<<<<< HEAD
	// Tests Server.Get
	getReq := pb.GetRequest{
		Path: []byte("here/is/a/path"),
		XApiKey: []byte("abc123"),
	}
=======
	// // Tests Server.Get
	// getReq := proto.GetRequest{
	// 	Path: []byte("here/is/a/path"),
	// }
>>>>>>> work in progress for modifying test suite to accomodate credentials
=======
	// Tests Server.Get
	getReq := proto.GetRequest{
		Path: []byte("here/is/a/path"),
		XApiKey: []byte("abc123"),
	}
>>>>>>> modified tests

	getRes, err := c.Get(ctx, &getReq)
	assert.NoError(t, err)

<<<<<<< HEAD
<<<<<<< HEAD
	if !bytes.Equal(getRes.Pointer, pointerBytes) {
		t.Error("Expected to get same content that was put")
	}
=======
	// if !bytes.Equal(getRes.SmallValue, fp.SmallValue) {
	// 	t.Error("Expected to get same content that was put")
	// }
>>>>>>> work in progress for modifying test suite to accomodate credentials
=======
	if !bytes.Equal(getRes.SmallValue, fp.SmallValue) {
		t.Error("Expected to get same content that was put")
	}
>>>>>>> modified tests

	if mdb.timesCalled != 2 {
		t.Error("Failed to call mockdb correct number of times")
	}

<<<<<<< HEAD
<<<<<<< HEAD
	// Puts another pointer entry to test delete and list
	pr2 := pb.PutRequest{
		Path: []byte("here/is/another/path"),
		Pointer: &pb.Pointer{
			Type: pb.Pointer_INLINE,
			Encryption: &pb.EncryptionScheme{
				EncryptedEncryptionKey: []byte("key"),
				EncryptedStartingNonce: []byte("nonce"),
			},
			InlineSegment: []byte("raisins"),
		},
		XApiKey: []byte("abc123"),
	}

	_, err = c.Put(ctx, &pr2)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to Put")
	}
=======
	// // Puts another file path to test delete and list
	// fp2 := proto.FilePath{
	// 	Path:       []byte("here/is/another/path"),
	// 	SmallValue: []byte("raisins"),
	// }
=======
	// Puts another file path to test delete and list
	fp2 := proto.FilePath{
		Path:       []byte("here/is/another/path"),
		SmallValue: []byte("raisins"),
		XApiKey: []byte("abc123"),
	}
>>>>>>> modified tests

	putRes2, err := c.Put(ctx, &fp2)
	assert.NoError(t, err)

<<<<<<< HEAD
	// if putRes2.Confirmation != "success" {
	// 	t.Error("Failed to receive success Put response")
	// }
>>>>>>> work in progress for modifying test suite to accomodate credentials
=======
	if putRes2.Confirmation != "success" {
		t.Error("Failed to receive success Put response")
	}
>>>>>>> modified tests

	if mdb.timesCalled != 3 {
		t.Error("Failed to call mockdb correct number of times")
	}

<<<<<<< HEAD
<<<<<<< HEAD
	// Test Server.Delete
	delReq := pb.DeleteRequest{
		Path: []byte("here/is/a/path"),
		XApiKey: []byte("abc123"),
	}

	_, err = c.Delete(ctx, &delReq)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to delete")
	}
=======
	// // Test Server.Delete
	// delReq := proto.DeleteRequest{
	// 	Path: []byte("here/is/a/path"),
	// }
=======
	// Test Server.Delete
	delReq := proto.DeleteRequest{
		Path: []byte("here/is/a/path"),
		XApiKey: []byte("abc123"),
	}
>>>>>>> modified tests

	delRes, err := c.Delete(ctx, &delReq)
	if err != nil {
		t.Error("Failed to delete file path")
	}

<<<<<<< HEAD
	// if delRes.Confirmation != "success" {
	// 	t.Error("Failed to receive success delete response")
	// }
>>>>>>> work in progress for modifying test suite to accomodate credentials
=======
	if delRes.Confirmation != "success" {
		t.Error("Failed to receive success delete response")
	}
>>>>>>> modified tests

	if mdb.timesCalled != 4 {
		t.Error("Failed to call mockdb correct number of times")
	}

<<<<<<< HEAD
<<<<<<< HEAD
	// Tests Server.List
	listReq := pb.ListRequest{
		// This pagination functionality doesn't work yet.
		// The given arguments are placeholders.
		StartingPathKey: []byte("test/pointer/path"),
		Limit:           5,
		XApiKey: []byte("abc123"),
	}
=======
	// // Tests Server.List
	// listReq := proto.ListRequest{
	// 	Bucket: []byte("files"),
	// }
>>>>>>> work in progress for modifying test suite to accomodate credentials
=======
	// Tests Server.List
	listReq := proto.ListRequest{
		Bucket: []byte("files"),
		XApiKey: []byte("abc123"),
	}
>>>>>>> modified tests

	listRes, err := c.List(ctx, &listReq)
	if err != nil {
		t.Error("Failed to list file paths")
	}

<<<<<<< HEAD
<<<<<<< HEAD
	if !bytes.Equal(listRes.Paths[0], []byte("here/is/another/path")) {
		t.Error("Failed to list correct file path")
	}
=======
	// if !bytes.Equal(listRes.Filepaths[0], []byte("here/is/another/path")) {
	// 	t.Error("Failed to list correct file path")
	// }
>>>>>>> work in progress for modifying test suite to accomodate credentials
=======
	if !bytes.Equal(listRes.Filepaths[0], []byte("here/is/another/path")) {
		t.Error("Failed to list correct file path")
	}
>>>>>>> modified tests

	if mdb.timesCalled != 5 {
		t.Error("Failed to call mockdb correct number of times")
	}
}
