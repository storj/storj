// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/internal/test"
	pb "storj.io/storj/protos/netstate"
)

const (
	APIKey = "abc123"
)

func TestMain(m *testing.M) {
	viper.SetEnvPrefix("API")
	os.Setenv("APIKey", APIKey)
	viper.AutomaticEnv()
	os.Exit(m.Run())
}

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

	// Tests Put
	pr1 := pb.PutRequest{
		Path: []byte("file/path/1"),
		Pointer: &pb.Pointer{
			Type: pb.Pointer_INLINE,
			Encryption: &pb.EncryptionScheme{
				EncryptedEncryptionKey: []byte("key"),
				EncryptedStartingNonce: []byte("nonce"),
			},
			InlineSegment: []byte("oatmeal"),
		},
		APIKey: []byte(APIKey),
	}
	_, err = c.Put(ctx, &pr1)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to Put")
	}
	if mdb.PutCalled != 1 {
		t.Error("Failed to call mockdb correctly")
	}
	pointerBytes, err := proto.Marshal(pr1.Pointer)
	if err != nil {
		t.Error("Failed to marshal test pointer")
	}
	if !bytes.Equal(mdb.Data[string(pr1.Path)], pointerBytes) {
		t.Error("Expected saved pointer to equal given pointer")
	}

	// Tests Get
	getReq := pb.GetRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte(APIKey),
	}
	getRes, err := c.Get(ctx, &getReq)
	assert.NoError(t, err)

	if !bytes.Equal(getRes.Pointer, pointerBytes) {
		t.Error("Expected to get same content that was put")
	}
	if mdb.GetCalled != 1 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Tests Get with bad auth
	getReq2 := pb.GetRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte("wrong key"),
	}
	_, err = c.Get(ctx, &getReq2)
	if err == nil {
		t.Error("Failed to error for wrong auth key")
	}

	// Puts more pointer entries to test Delete and List
	pr2 := pb.PutRequest{
		Path: []byte("file/path/2"),
		Pointer: &pb.Pointer{
			Type: pb.Pointer_INLINE,
			Encryption: &pb.EncryptionScheme{
				EncryptedEncryptionKey: []byte("key"),
				EncryptedStartingNonce: []byte("nonce"),
			},
			InlineSegment: []byte("raisins"),
		},
		APIKey: []byte(APIKey),
	}
	// rps is an example slice of RemotePieces to add to this
	// REMOTE pointer type.
	var rps []*pb.RemotePiece
	rps = append(rps, &pb.RemotePiece{
		PieceNum: int64(1),
		NodeId:   "testId",
	})
	pr3 := pb.PutRequest{
		Path: []byte("file/path/3"),
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
			EncryptedUnencryptedSize: []byte("this big"),
		},
		APIKey: []byte(APIKey),
	}
	pr4 := pb.PutRequest{
		Path: []byte("file/path/4"),
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
			EncryptedUnencryptedSize: []byte("this big"),
		},
		APIKey: []byte(APIKey),
	}

	_, err = c.Put(ctx, &pr2)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to Put")
	}
	if mdb.PutCalled != 2 {
		t.Error("Failed to call mockdb correct number of times")
	}
	_, err = c.Put(ctx, &pr3)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to Put")
	}
	if mdb.PutCalled != 3 {
		t.Error("Failed to call mockdb correct number of times")
	}
	_, err = c.Put(ctx, &pr4)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to Put")
	}
	if mdb.PutCalled != 4 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Tests Put with bad auth
	pr5 := pb.PutRequest{
		Path: []byte("file/path/5"),
		Pointer: &pb.Pointer{
			Type: pb.Pointer_INLINE,
			Encryption: &pb.EncryptionScheme{
				EncryptedEncryptionKey: []byte("key"),
				EncryptedStartingNonce: []byte("nonce"),
			},
			InlineSegment: []byte("oatmeal"),
		},
		APIKey: []byte("wrong key"),
	}
	_, err = c.Put(ctx, &pr5)
	if err == nil {
		t.Error("Failed to error for wrong auth key")
	}

	// Test Delete
	delReq1 := pb.DeleteRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte(APIKey),
	}
	_, err = c.Delete(ctx, &delReq1)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to delete")
	}
	if mdb.DeleteCalled != 1 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Test Delete with bad auth
	delReq2 := pb.DeleteRequest{
		Path:   []byte("file/path/2"),
		APIKey: []byte("bad auth"),
	}
	_, err = c.Delete(ctx, &delReq2)
	if err == nil {
		t.Error("Failed to error with bad auth key")
	}

	// Tests List
	listReq := pb.ListRequest{
		StartingPathKey: []byte("file/path/2"),
		Limit:           4,
		APIKey:          []byte(APIKey),
	}
	listRes, err := c.List(ctx, &listReq)
	if err != nil {
		t.Error("Failed to list file paths")
	}
	if !listRes.Truncated {
		t.Error("Expected list slice to be truncated")
	}
	if !bytes.Equal(listRes.Paths[0], []byte("file/path/2")) {
		t.Error("Failed to list correct file path")
	}
	if mdb.ListCalled != 1 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Tests list with truncated value
	listReq2 := pb.ListRequest{
		StartingPathKey: []byte("file/path/3"),
		Limit:           1,
		APIKey:          []byte(APIKey),
	}
	listRes2, err := c.List(ctx, &listReq2)
	if err != nil {
		t.Error("Failed to list file paths")
	}
	if listRes2.Truncated {
		t.Error("Expected list slice to not be truncated")
	}
	if mdb.ListCalled != 2 {
		t.Error("Failed to call mockdb correct number of times")
	}

	// Tests List without starting key
	listReq3 := pb.ListRequest{
		Limit:  4,
		APIKey: []byte(APIKey),
	}
	_, err = c.List(ctx, &listReq3)
	if err == nil {
		t.Error("Failed to error when not given starting key")
	}

	// Tests List without limit
	listReq4 := pb.ListRequest{
		StartingPathKey: []byte("file/path/3"),
		APIKey:          []byte(APIKey),
	}
	_, err = c.List(ctx, &listReq4)
	if err == nil {
		t.Error("Failed to error when not given limit")
	}

	// Tests List with bad auth
	listReq5 := pb.ListRequest{
		StartingPathKey: []byte("file/path/3"),
		Limit:           1,
		APIKey:          []byte("bad key"),
	}
	_, err = c.List(ctx, &listReq5)
	if err == nil {
		t.Error("Failed to error when given wrong auth key")
	}
}
