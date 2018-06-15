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

var (
	APIKey = "abc123"
)

func TestMain(m *testing.M) {
	viper.SetEnvPrefix("API")
	os.Setenv("APIKey", APIKey)
	viper.AutomaticEnv()
	os.Exit(m.Run())
}

func SetupTests() (pb.NetStateClient, context.Context, *test.MockKeyValueStore, error) {
	logger, _ := zap.NewDevelopment()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 9000))
	if err != nil {
		logger.Fatal("SetupTests failed to listen")
		return nil, nil, nil, err
	}

	mdb := test.NewMockKeyValueStore(test.KvStore{})

	grpcServer := grpc.NewServer()
	pb.RegisterNetStateServer(grpcServer, NewServer(mdb, logger))

	defer grpcServer.GracefulStop()
	go grpcServer.Serve(lis)

	address := lis.Addr().String()
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		logger.Fatal("SetupTests conn failed to dial")
		return nil, nil, nil, err
	}
	c := pb.NewNetStateClient(conn)
	ctx := context.Background()

	return c, ctx, mdb, nil
}

func MakePointer(path []byte, auth bool) pb.PutRequest {
	if !auth {
		APIKey = "wrong key"
	}
	// rps is an example slice of RemotePieces to add to this
	// REMOTE pointer type.
	var rps []*pb.RemotePiece
	rps = append(rps, &pb.RemotePiece{
		PieceNum: int64(1),
		NodeId:   "testId",
	})
	pr := pb.PutRequest{
		Path: path,
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
	return pr
}

func MakeAndPutPointers(howMany int) error {
	c, ctx, _, err := SetupTests()
	if err != nil {
		panic(err)
	}
	var pointers []pb.PutRequest
	for i := 1; i <= howMany; i++ {
		pointers = append(pointers, MakePointer([]byte("file/path/"+string(i)), true))
	}
	for _, p := range pointers {
		_, err = c.Put(ctx, &p)
		if err != nil || status.Code(err) == codes.Internal {
			return err
		}
	}
	return nil
}

func TestPut(t *testing.T) {
	c, ctx, mdb, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}

	pr1 := MakePointer([]byte("file/path/1"), true)

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
}

func TestGet(t *testing.T) {
	c, ctx, mdb, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}

	pr1 := MakePointer([]byte("file/path/1"), true)
	_, err = c.Put(ctx, &pr1)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to Put")
	}

	getReq := pb.GetRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte(APIKey),
	}
	getRes, err := c.Get(ctx, &getReq)
	assert.NoError(t, err)

	pr := MakePointer([]byte("file/path/1"), true)
	pointerBytes, err := proto.Marshal(pr.Pointer)
	if err != nil {
		t.Error("Failed to marshal test pointer")
	}
	if !bytes.Equal(getRes.Pointer, pointerBytes) {
		t.Error("Expected to get same content that was put")
	}
	if mdb.GetCalled != 1 {
		t.Error("Failed to call mockdb correct number of times")
	}
}

func TestGetAuth(t *testing.T) {
	c, ctx, _, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}

	getReq := pb.GetRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte("wrong key"),
	}
	_, err = c.Get(ctx, &getReq)
	if err == nil {
		t.Error("Failed to error for wrong auth key")
	}
}

func TestPutAuth(t *testing.T) {
	c, ctx, _, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}
	pr := MakePointer([]byte("file/path"), false)
	_, err = c.Put(ctx, &pr)
	if err == nil {
		t.Error("Failed to error for wrong auth key")
	}
}

func TestDelete(t *testing.T) {
	c, ctx, mdb, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}
	pr := MakePointer([]byte("delete/me"), true)
	_, err = c.Put(ctx, &pr)

	delReq := pb.DeleteRequest{
		Path:   []byte("delete/me"),
		APIKey: []byte(APIKey),
	}
	_, err = c.Delete(ctx, &delReq)
	if err != nil || status.Code(err) == codes.Internal {
		t.Error("Failed to delete")
	}
	if mdb.DeleteCalled != 1 {
		t.Error("Failed to call mockdb correct number of times")
	}
}

func TestDeleteAuth(t *testing.T) {
	c, ctx, _, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}
	pr := MakePointer([]byte("file/path/2"), true)
	_, err = c.Put(ctx, &pr)

	delReq := pb.DeleteRequest{
		Path:   []byte("file/path/2"),
		APIKey: []byte("wrong key"),
	}
	_, err = c.Delete(ctx, &delReq)
	if err == nil {
		t.Error("Failed to error with wrong auth key")
	}
}

func TestList(t *testing.T) {
	c, ctx, mdb, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}

	err = MakeAndPutPointers(4)
	if err != nil {
		t.Error("Failed to MakeAndPutPointers")
	}

	listReq := pb.ListRequest{
		StartingPathKey: []byte("file/path/2"),
		Limit:           5,
		APIKey:          []byte(APIKey),
	}
	listRes, err := c.List(ctx, &listReq)
	if err != nil {
		t.Error("Failed to list file paths")
	}
	if listRes.Truncated {
		t.Error("Expected list slice to not be truncated")
	}
	if !bytes.Equal(listRes.Paths[0], []byte("file/path/2")) {
		t.Error("Failed to list correct file path")
	}
	if mdb.ListCalled != 1 {
		t.Error("Failed to call mockdb correct number of times")
	}
}

func TestListTruncated(t *testing.T) {
	c, ctx, _, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}
	err = MakeAndPutPointers(5)
	if err != nil {
		t.Error("Failed to MakeAndPutPointers")
	}
	listReq := pb.ListRequest{
		StartingPathKey: []byte("file/path/3"),
		Limit:           1,
		APIKey:          []byte(APIKey),
	}
	listRes, err := c.List(ctx, &listReq)
	if err != nil {
		t.Error("Failed to list file paths")
	}
	if !listRes.Truncated {
		t.Error("Expected list slice to be truncated")
	}
}

func TestListWithoutStartingKey(t *testing.T) {
	c, ctx, _, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}
	listReq := pb.ListRequest{
		Limit:  4,
		APIKey: []byte(APIKey),
	}
	_, err = c.List(ctx, &listReq)
	if err == nil {
		t.Error("Failed to error when not given starting key")
	}
}

func TestListWithoutLimit(t *testing.T) {
	c, ctx, _, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}
	listReq := pb.ListRequest{
		StartingPathKey: []byte("file/path/3"),
		APIKey:          []byte(APIKey),
	}
	_, err = c.List(ctx, &listReq)
	if err == nil {
		t.Error("Failed to error when not given limit")
	}
}

func TestListAuth(t *testing.T) {
	c, ctx, _, err := SetupTests()
	if err != nil {
		t.Error("Failed to SetupTests")
	}
	listReq := pb.ListRequest{
		StartingPathKey: []byte("file/path/3"),
		Limit:           1,
		APIKey:          []byte("wrong key"),
	}
	_, err = c.List(ctx, &listReq)
	if err == nil {
		t.Error("Failed to error when given wrong auth key")
	}
}
