// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"bytes"
	"context"
	"net"
	"os"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/internal/test"
	pb "storj.io/storj/protos/netstate"
)

var (
	APIKey = "abc123"
	ctx    = context.Background()
)

type NetStateClientTest struct {
	*testing.T

	server *grpc.Server
	lis    net.Listener
	mdb    *test.MockKeyValueStore
	c      pb.NetStateClient
}

func NewNetStateClientTest(t *testing.T) *NetStateClientTest {
	mdb := test.NewMockKeyValueStore(test.KvStore{})

	// tests should always listen on "localhost:0"
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterNetStateServer(grpcServer, NewServer(mdb, zap.L()))
	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	if err != nil {
		grpcServer.GracefulStop()
		lis.Close()
		t.Fatal(err)
	}

	return &NetStateClientTest{
		T:      t,
		server: grpcServer,
		lis:    lis,
		mdb:    mdb,
		c:      pb.NewNetStateClient(conn),
	}
}

func (nt *NetStateClientTest) Close() {
	nt.server.GracefulStop()
	nt.lis.Close()
}

func TestMain(m *testing.M) {
	viper.SetEnvPrefix("API")
	os.Setenv("APIKey", APIKey)
	viper.AutomaticEnv()
	os.Exit(m.Run())
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

func MakePointers(howMany int) []pb.PutRequest {
	var pointers []pb.PutRequest
	for i := 1; i == howMany; i++ {
		pointers = append(pointers, MakePointer([]byte("file/path/"+string(i)), true))
	}
	return pointers
}

func (nt *NetStateClientTest) Put(pr pb.PutRequest) {
	pre := nt.mdb.PutCalled
	_, err := nt.c.Put(ctx, &pr)
	if err != nil {
		panic(err)
	}
	assert.Equal(nt, pre+1, nt.mdb.PutCalled)
}

func (nt *NetStateClientTest) Get(path string) (getRes *pb.GetResponse) {
	pre := nt.mdb.GetCalled
	getRes, err := nt.c.Get(ctx, &pb.GetRequest{
		Path:   []byte(path),
		APIKey: []byte(APIKey),
	})
	if err != nil {
		panic(err)
	}
	assert.Equal(nt, pre+1, nt.mdb.GetCalled)

	return getRes
}

func (nt *NetStateClientTest) List(lr pb.ListRequest) (listRes *pb.ListResponse) {
	pre := nt.mdb.ListCalled
	listRes, err := nt.c.List(ctx, &lr)
	if err != nil {
		panic(err)
	}
	assert.Equal(nt, pre+1, nt.mdb.ListCalled)

	return listRes
}

func (nt *NetStateClientTest) Delete(dr pb.DeleteRequest) (delRes *pb.DeleteResponse) {
	pre := nt.mdb.DeleteCalled
	delRes, err := nt.c.Delete(ctx, &dr)
	assert.NoError(nt, err)
	assert.Equal(nt, pre+1, nt.mdb.DeleteCalled)

	return delRes
}

func TestNetStatePutGet(t *testing.T) {
	nt := NewNetStateClientTest(t)
	defer nt.Close()

	preGet := nt.mdb.GetCalled
	prePut := nt.mdb.PutCalled

	gr := nt.Get("file/path/1")
	if gr.GetPointer() != nil {
		t.Error("expected no pointer")
	}

	pr := MakePointer([]byte("file/path/1"), true)
	nt.Put(pr)

	gr = nt.Get("file/path/1")
	if gr == nil {
		t.Error("failed to get the put pointer")
	}

	pointerBytes, err := proto.Marshal(pr.Pointer)
	if err != nil {
		t.Error("Failed to marshal test pointer")
	}
	if !bytes.Equal(gr.Pointer, pointerBytes) {
		t.Error("Expected to get same content that was put")
	}
	if nt.mdb.GetCalled != preGet+2 {
		t.Error("Failed to call get correct number of times")
	}
	if nt.mdb.PutCalled != prePut+1 {
		t.Error("Failed to call put correct number of times")
	}
}

func TestGetAuth(t *testing.T) {
	nt := NewNetStateClientTest(t)
	defer nt.Close()

	getReq := pb.GetRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte("wrong key"),
	}
	_, err := nt.c.Get(ctx, &getReq)
	if err == nil {
		t.Error("Failed to error for wrong auth key")
		panic(err)
	}
}

func TestPutAuth(t *testing.T) {
	nt := NewNetStateClientTest(t)
	defer nt.Close()

	pr := MakePointer([]byte("file/path"), false)
	_, err := nt.c.Put(ctx, &pr)
	if err == nil {
		t.Error("Failed to error for wrong auth key")
		panic(err)
	}
}

func TestDelete(t *testing.T) {
	nt := NewNetStateClientTest(t)
	defer nt.Close()

	pre := nt.mdb.DeleteCalled

	reqs := MakePointers(1)
	_, err := nt.c.Put(ctx, &reqs[0])
	if err != nil {
		t.Error("Failed to put")
		panic(err)
	}

	delReq := pb.DeleteRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte(APIKey),
	}
	_, err = nt.c.Delete(ctx, &delReq)
	if err != nil {
		t.Error("Failed to delete")
		panic(err)
	}

	assert.Equal(nt, pre+1, nt.mdb.DeleteCalled)
}

func TestDeleteAuth(t *testing.T) {
	nt := NewNetStateClientTest(t)
	defer nt.Close()

	reqs := MakePointers(1)
	_, err := nt.c.Put(ctx, &reqs[0])
	if err != nil {
		t.Error("Failed to put")
		panic(err)
	}

	delReq := pb.DeleteRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte("wrong key"),
	}
	_, err = nt.c.Delete(ctx, &delReq)
	if err == nil {
		t.Error("Failed to error with wrong auth key")
		panic(err)
	}
}

// func TestList(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	reqs := MakePointers(4)
// 	for _, req := range reqs {
// 		_, err := nt.c.Put(ctx, &req)
// 		assert.NoError(nt, err)
// 	}

// 	pre := nt.mdb.ListCalled

// 	listReq := pb.ListRequest{
// 		StartingPathKey: []byte("file/path/2"),
// 		Limit:           5,
// 		APIKey:          []byte(APIKey),
// 	}
// 	listRes, err := nt.c.List(ctx, &listReq)
// 	if err != nil {
// 		t.Error("Failed to list file paths")
// 	}
// 	if listRes.Truncated {
// 		t.Error("Expected list slice to not be truncated")
// 	}
// 	if !bytes.Equal(listRes.Paths[0], []byte("file/path/2")) {
// 		t.Error("Failed to list correct file path")
// 	}
// 	assert.Equal(nt, pre+1, nt.mdb.ListCalled)
// }

// func TestListTruncated(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	reqs := MakePointers(3)
// 	for _, req := range reqs {
// 		_, err := nt.c.Put(ctx, &req)
// 		assert.NoError(nt, err)
// 	}

// 	listReq := pb.ListRequest{
// 		StartingPathKey: []byte("file/path/1"),
// 		Limit:           1,
// 		APIKey:          []byte(APIKey),
// 	}
// 	listRes, err := nt.c.List(ctx, &listReq)
// 	if err != nil {
// 		t.Error("Failed to list file paths")
// 	}
// 	if !listRes.Truncated {
// 		t.Error("Expected list slice to be truncated")
// 	}
// }

// func TestListWithoutStartingKey(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	listReq := pb.ListRequest{
// 		Limit:  4,
// 		APIKey: []byte(APIKey),
// 	}
// 	_, err := nt.c.List(ctx, &listReq)
// 	if err == nil {
// 		t.Error("Failed to error when not given starting key")
// 	}
// }

// func TestListWithoutLimit(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	listReq := pb.ListRequest{
// 		StartingPathKey: []byte("file/path/3"),
// 		APIKey:          []byte(APIKey),
// 	}
// 	_, err := nt.c.List(ctx, &listReq)
// 	if err == nil {
// 		t.Error("Failed to error when not given limit")
// 	}
// }

// func TestListAuth(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	listReq := pb.ListRequest{
// 		StartingPathKey: []byte("file/path/3"),
// 		Limit:           1,
// 		APIKey:          []byte("wrong key"),
// 	}
// 	_, err := nt.c.List(ctx, &listReq)
// 	if err == nil {
// 		t.Error("Failed to error when given wrong auth key")
// 	}
// }
