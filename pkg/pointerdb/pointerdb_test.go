// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/internal/test"
	pb "storj.io/storj/protos/pointerdb"
)

var (
	ctx = context.Background()
)

type PointerDBClientTest struct {
	*testing.T

	server *grpc.Server
	lis    net.Listener
	mdb    *test.MockKeyValueStore
	c      pb.PointerDBClient
}

func NewPointerDBClientTest(t *testing.T) *PointerDBClientTest {
	mdb := test.NewMockKeyValueStore(test.KvStore{})

	viper.Reset()
	viper.Set("key", "abc123")

	// tests should always listen on "localhost:0"
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPointerDBServer(grpcServer, NewServer(mdb, zap.L()))
	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	if err != nil {
		grpcServer.GracefulStop()
		lis.Close()
		t.Fatal(err)
	}

	return &PointerDBClientTest{
		T:      t,
		server: grpcServer,
		lis:    lis,
		mdb:    mdb,
		c:      pb.NewPointerDBClient(conn),
	}
}

func (nt *PointerDBClientTest) Close() {
	nt.server.GracefulStop()
	nt.lis.Close()
}

func MakePointer(path []byte, auth bool) pb.PutRequest {
	var APIKey = "abc123"
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
			Size: int64(1),
		},
		APIKey: []byte(APIKey),
	}
	return pr
}

func MakePointers(howMany int) []pb.PutRequest {
	var pointers []pb.PutRequest
	for i := 1; i <= howMany; i++ {
		newPointer := MakePointer([]byte("file/path/"+fmt.Sprintf("%d", i)), true)
		pointers = append(pointers, newPointer)
	}
	return pointers
}

func (nt *PointerDBClientTest) Put(pr pb.PutRequest) *pb.PutResponse {
	pre := nt.mdb.PutCalled
	putRes, err := nt.c.Put(ctx, &pr)
	if err != nil {
		nt.HandleErr(err, "Failed to put")
	}
	if pre+1 != nt.mdb.PutCalled {
		nt.HandleErr(nil, "Failed to call Put correct number of times")
	}
	return putRes
}

func (nt *PointerDBClientTest) Get(gr pb.GetRequest) *pb.GetResponse {
	pre := nt.mdb.GetCalled
	getRes, err := nt.c.Get(ctx, &gr)
	if err != nil {
		nt.HandleErr(err, "Failed to get")
	}
	if pre+1 != nt.mdb.GetCalled {
		nt.HandleErr(nil, "Failed to call Get correct number of times")
	}
	return getRes
}

func (nt *PointerDBClientTest) List(lr pb.ListRequest) (listRes *pb.ListResponse) {
	pre := nt.mdb.ListCalled
	listRes, err := nt.c.List(ctx, &lr)
	if err != nil {
		nt.HandleErr(err, "Failed to list")
	}
	if pre+1 != nt.mdb.ListCalled {
		nt.HandleErr(nil, "Failed to call List correct number of times")
	}
	return listRes
}

func (nt *PointerDBClientTest) Delete(dr pb.DeleteRequest) (delRes *pb.DeleteResponse) {
	pre := nt.mdb.DeleteCalled
	delRes, err := nt.c.Delete(ctx, &dr)
	if err != nil {
		nt.HandleErr(err, "Failed to delete")
	}
	if pre+1 != nt.mdb.DeleteCalled {
		nt.HandleErr(nil, "Failed to call Delete correct number of times")
	}

	return delRes
}

func (nt *PointerDBClientTest) HandleErr(err error, msg string) {
	nt.Error(msg)
	if err != nil {
		panic(err)
	}
	panic(msg)
}

func TestMockList(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	err := nt.mdb.Put([]byte("k1"), []byte("v1"))
	if err != nil {
		panic(err)
	}
	err = nt.mdb.Put([]byte("k2"), []byte("v2"))
	if err != nil {
		panic(err)
	}
	err = nt.mdb.Put([]byte("k3"), []byte("v3"))
	if err != nil {
		panic(err)
	}
	err = nt.mdb.Put([]byte("k4"), []byte("v4"))
	if err != nil {
		panic(err)
	}

	keys, err := nt.mdb.List([]byte("k2"), 2)
	if err != nil {
		nt.HandleErr(err, "Failed to list")
	}
	if fmt.Sprintf("%s", keys) != "[k2 k3]" {
		nt.HandleErr(nil, "Failed to receive accepted list. Received "+fmt.Sprintf("%s", keys))
	}

	keys, err = nt.mdb.List(nil, 3)
	if err != nil {
		nt.HandleErr(err, "Failed to list")
	}
	if fmt.Sprintf("%s", keys) != "[k1 k2 k3]" {
		nt.HandleErr(nil, "Failed to receive accepted list. Received "+fmt.Sprintf("%s", keys))
	}
}

func TestPointerDBPutGet(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	preGet := nt.mdb.GetCalled
	prePut := nt.mdb.PutCalled

	gr := nt.Get(pb.GetRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte("abc123"),
	})
	if gr.Pointer != nil {
		nt.HandleErr(nil, "Expected no pointer")
	}

	pr := MakePointer([]byte("file/path/1"), true)
	nt.Put(pr)

	gr = nt.Get(pb.GetRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte("abc123"),
	})
	if gr == nil {
		nt.HandleErr(nil, "Failed to get the put pointer")
	}

	pointerBytes, err := proto.Marshal(pr.Pointer)
	if err != nil {
		nt.HandleErr(err, "Failed to marshal test pointer")
	}
	if !bytes.Equal(gr.Pointer, pointerBytes) {
		nt.HandleErr(nil, "Expected to get same content that was put")
	}
	if nt.mdb.GetCalled != preGet+2 {
		nt.HandleErr(nil, "Failed to call get correct number of times")
	}
	if nt.mdb.PutCalled != prePut+1 {
		nt.HandleErr(nil, "Failed to call put correct number of times")
	}
}

func TestGetAuth(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	getReq := pb.GetRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte("wrong key"),
	}
	_, err := nt.c.Get(ctx, &getReq)
	if err == nil {
		nt.HandleErr(nil, "Failed to error for wrong auth key")
	}
}

func TestPutAuth(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	pr := MakePointer([]byte("file/path"), false)
	_, err := nt.c.Put(ctx, &pr)
	if err == nil {
		nt.HandleErr(nil, "Failed to error for wrong auth key")
	}
}

func TestDelete(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	pre := nt.mdb.DeleteCalled

	reqs := MakePointers(1)
	_, err := nt.c.Put(ctx, &reqs[0])
	if err != nil {
		nt.HandleErr(err, "Failed to put")
	}

	delReq := pb.DeleteRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte("abc123"),
	}
	_, err = nt.c.Delete(ctx, &delReq)
	if err != nil {
		nt.HandleErr(err, "Failed to delete")
	}
	if pre+1 != nt.mdb.DeleteCalled {
		nt.HandleErr(nil, "Failed to call Delete correct number of times")
	}
}

func TestDeleteAuth(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	reqs := MakePointers(1)
	_, err := nt.c.Put(ctx, &reqs[0])
	if err != nil {
		nt.HandleErr(err, "Failed to put")
	}

	delReq := pb.DeleteRequest{
		Path:   []byte("file/path/1"),
		APIKey: []byte("wrong key"),
	}
	_, err = nt.c.Delete(ctx, &delReq)
	if err == nil {
		nt.HandleErr(nil, "Failed to error with wrong auth key")
	}
}

func TestList(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	reqs := MakePointers(4)
	for _, req := range reqs {
		nt.Put(req)
	}

	listReq := pb.ListRequest{
		StartingPathKey: []byte("file/path/2"),
		Limit:           5,
		APIKey:          []byte("abc123"),
	}
	listRes := nt.List(listReq)
	if listRes.Truncated {
		nt.HandleErr(nil, "Expected list slice to not be truncated")
	}
	if !bytes.Equal(listRes.Paths[0], []byte("file/path/2")) {
		nt.HandleErr(nil, "Failed to list correct file paths")
	}
}

func TestListTruncated(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	reqs := MakePointers(3)
	for _, req := range reqs {
		_, err := nt.c.Put(ctx, &req)
		if err != nil {
			nt.HandleErr(err, "Failed to put")
		}
	}

	listReq := pb.ListRequest{
		StartingPathKey: []byte("file/path/1"),
		Limit:           1,
		APIKey:          []byte("abc123"),
	}
	listRes, err := nt.c.List(ctx, &listReq)
	if err != nil {
		nt.HandleErr(err, "Failed to list file paths")
	}
	if !listRes.Truncated {
		nt.HandleErr(nil, "Expected list slice to be truncated")
	}
}

func TestListWithoutStartingKey(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	reqs := MakePointers(3)
	for _, req := range reqs {
		_, err := nt.c.Put(ctx, &req)
		if err != nil {
			nt.HandleErr(err, "Failed to put")
		}
	}

	listReq := pb.ListRequest{
		Limit:  3,
		APIKey: []byte("abc123"),
	}
	listRes, err := nt.c.List(ctx, &listReq)
	if err != nil {
		nt.HandleErr(err, "Failed to list without starting key")
	}

	if !bytes.Equal(listRes.Paths[2], []byte("file/path/3")) {
		nt.HandleErr(nil, "Failed to list correct paths")
	}
}

func TestListWithoutLimit(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	listReq := pb.ListRequest{
		StartingPathKey: []byte("file/path/3"),
		APIKey:          []byte("abc123"),
	}
	_, err := nt.c.List(ctx, &listReq)
	if err == nil {
		t.Error("Failed to error when not given limit")
	}
}

func TestListAuth(t *testing.T) {
	nt := NewPointerDBClientTest(t)
	defer nt.Close()

	listReq := pb.ListRequest{
		StartingPathKey: []byte("file/path/3"),
		Limit:           1,
		APIKey:          []byte("wrong key"),
	}
	_, err := nt.c.List(ctx, &listReq)
	if err == nil {
		t.Error("Failed to error when given wrong auth key")
	}
}
