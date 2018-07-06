// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	//"bytes"
	"context"
	"fmt"
	//"net"
	"testing"

	//"github.com/golang/protobuf/proto"
	//"github.com/spf13/viper"
	//"go.uber.org/zap"
	//"google.golang.org/grpc"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	pb "storj.io/storj/protos/pointerdb"
	p "storj.io/storj/pkg/paths"
)

var (
	ctx = context.Background()
)


func TestNewNetStateClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	//mdb := test.NewMockKeyValueStore(test.KvStore{})

	 gc:= NewMockNetStateClient(ctrl)
	 nsc := NetState{grpcClient: gc}
	 
	 assert.NotNil(t, nsc)


	
	 nsc.Put(ctx, "file1/file2", "pointer", "abc123" )



	// viper.Reset()
	// viper.Set("key", "abc123")

	// tests should always listen on "localhost:0"
	// lis, err := net.Listen("tcp", "localhost:0")
	// if err != nil {
	// 	panic(err)
	// }

	// grpcServer := grpc.NewServer()
	// pb.RegisterNetStateServer(grpcServer, NewServer(mdb, zap.L()))
	// go grpcServer.Serve(lis)

	// conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	// if err != nil {
	// 	grpcServer.GracefulStop()
	// 	lis.Close()
	// 	t.Fatal(err)
	// }

	// return &NetStateClientTest{
	// 	T:      t,
	// 	server: grpcServer,
	// 	lis:    lis,
	// 	mdb:    mdb,
	// 	c:      pb.NewNetStateClient(conn),
	// }
}

// func (nt *NetStateClientTest) Close() {
// 	nt.server.GracefulStop()
// 	nt.lis.Close()
// }

func MakePointer(path p.Path, auth bool) pb.PutRequest {
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
		APIKey: []byte(APIKey),
	}
	return pr
}

func MakePointers(howMany int) []pb.PutRequest {
	var pointers []pb.PutRequest
	for i := 1; i <= howMany; i++ {
		var path = p.New("file/path/"+ fmt.Sprintf("%d", i))

		newPointer := MakePointer(path, true)
		pointers = append(pointers, newPointer)
	}
	return pointers
}


// func (m *MockedNetState) Put(ctx context.Context, path p.Path, pointer *pb.Pointer, APIKey []byte) error {
// 	args := m.Called(ctx, path, pointer, APIKey)
// 	pre := m.mdb.PutCalled
// 	_, err := m.c.Put(ctx, &pb.PutRequest{Path: path.Bytes(), Pointer: pointer, APIKey: APIKey})
// 	if err != nil {
// 		m.HandleErr(err, "Failed to put")
// 	}
// 	if pre+1 != m.mdb.PutCalled {
// 		m.HandleErr(nil, "Failed to call Put correct number of times")
// 	}
// 	return args.Error(0)
// }








// func (nt *NetStateClientTest) Put(pr pb.PutRequest) *pb.PutResponse {
// 	pre := nt.mdb.PutCalled
// 	putRes, err := nt.c.Put(ctx, &pr)
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to put")
// 	}
// 	if pre+1 != nt.mdb.PutCalled {
// 		nt.HandleErr(nil, "Failed to call Put correct number of times")
// 	}
// 	return putRes
// }


// func (m *MockedNetState) Get(ctx context.Context, path p.Path, APIKey []byte) (*pb.Pointer, error) {
// 	args := m.Called(ctx, path, APIKey)
// 	pre := m.mdb.GetCalled

// 	_, err := m.c.Get(ctx, &pb.GetRequest{Path: path.Bytes(), APIKey: APIKey})
	
// 	if err != nil {
// 		m.HandleErr(err, "Failed to get")
// 	}

// 	if pre+1 != m.mdb.GetCalled {
// 		m.HandleErr(nil, "Failed to call Get correct number of times")
// 	}
// 	return  args.Get(0).(*pb.Pointer), args.Error(1)
// }



// func (nt *NetStateClientTest) Get(gr pb.GetRequest) *pb.GetResponse {
// 	pre := nt.mdb.GetCalled
// 	getRes, err := nt.c.Get(ctx, &gr)
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to get")
// 	}
// 	if pre+1 != nt.mdb.GetCalled {
// 		nt.HandleErr(nil, "Failed to call Get correct number of times")
// 	}
// 	return getRes
// }

// func (nt *NetStateClientTest) List(lr pb.ListRequest) (listRes *pb.ListResponse) {
// 	pre := nt.mdb.ListCalled
// 	listRes, err := nt.c.List(ctx, &lr)
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to list")
// 	}
// 	if pre+1 != nt.mdb.ListCalled {
// 		nt.HandleErr(nil, "Failed to call List correct number of times")
// 	}
// 	return listRes
// }

// func (nt *NetStateClientTest) Delete(dr pb.DeleteRequest) (delRes *pb.DeleteResponse) {
// 	pre := nt.mdb.DeleteCalled
// 	delRes, err := nt.c.Delete(ctx, &dr)
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to delete")
// 	}
// 	if pre+1 != nt.mdb.DeleteCalled {
// 		nt.HandleErr(nil, "Failed to call Delete correct number of times")
// 	}

// 	return delRes
// }


// func (m *MockedNetState) HandleErr(err error, msg string) {
// 	if err != nil {
// 		panic(err)
// 	}
// 	panic(msg)
// }


// func (nt *NetStateClientTest) HandleErr(err error, msg string) {
// 	nt.Error(msg)
// 	if err != nil {
// 		panic(err)
// 	}
// 	panic(msg)
// }

// func TestMockList(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	err := nt.mdb.Put([]byte("k1"), []byte("v1"))
// 	if err != nil {
// 		panic(err)
// 	}
// 	err = nt.mdb.Put([]byte("k2"), []byte("v2"))
// 	if err != nil {
// 		panic(err)
// 	}
// 	err = nt.mdb.Put([]byte("k3"), []byte("v3"))
// 	if err != nil {
// 		panic(err)
// 	}
// 	err = nt.mdb.Put([]byte("k4"), []byte("v4"))
// 	if err != nil {
// 		panic(err)
// 	}

// 	keys, err := nt.mdb.List([]byte("k2"), 2)
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to list")
// 	}
// 	if fmt.Sprintf("%s", keys) != "[k2 k3]" {
// 		nt.HandleErr(nil, "Failed to receive accepted list. Received "+fmt.Sprintf("%s", keys))
// 	}

// 	keys, err = nt.mdb.List(nil, 3)
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to list")
// 	}
// 	if fmt.Sprintf("%s", keys) != "[k1 k2 k3]" {
// 		nt.HandleErr(nil, "Failed to receive accepted list. Received "+fmt.Sprintf("%s", keys))
// 	}
// }




// func TestNetStatePutGet(t *testing.T) {
// 	mockNetStateService := new(MockedNetState)

	//path := p.New("fold1/fold2/fold3/file.txt")
	//pr := MakePointer(path, true)
	//mockNetStateService.Put(ctx, path, pr.Pointer, pr.APIKey)

	// mockNetStateService.On("Put", ctx, path, pr.Pointer, []byte("abc123")).Return(nil)
	// mockNetStateService.On("Get", ctx, path, []byte("abc123")).Return(pr.Pointer, nil)

	// assert.Equal(t, pr.Pointer, pr.Pointer, "they should be equal")
	
	
	//(t, process.Main(func() error { return nil }, mockService))
//	mockNetStateService.AssertExpectations(t)

	// preGet := mockNetStateService.mdb.GetCalled
	// prePut := mockNetStateService.mdb.PutCalled

	//get fails here 
	//pointerA, err := mockNetStateService.Get(ctx, p.New("file/path/1"), []byte("abc123"))
	
	// if pointerA != nil {
	// 	mockNetStateService.HandleErr(nil, "Expected no pointer")
	// }
	
	// path := p.New("fold1/fold2/fold3/file.txt")
	
	// pr := MakePointer(path, true)
	// mockNetStateService.Put(ctx, path, pr.Pointer, pr.APIKey)

	
	// pointerB, err := mockNetStateService.Get(ctx, path,[]byte("abc123"))
	// if err != nil {
	// 	mockNetStateService.HandleErr(nil, "Failed to get the put pointer")
	// }

	// pointerBytes, err := proto.Marshal(pr.Pointer)
	
	// if err != nil {
	// 	mockNetStateService.HandleErr(err, "Failed to marshal test pointer")
	// }

	// if !bytes.Equal(pointerB, pointerBytes) {
	// 	mockNetStateService.HandleErr(nil, "Expected to get same content that was put")
	// }

	// if mockNetStateService.mdb.GetCalled != preGet+2 {
	// 	mockNetStateService.HandleErr(nil, "Failed to call get correct number of times")
	// }

	// if mockNetStateService.mdb.PutCalled != prePut+1 {
	// 	mockNetStateService.HandleErr(nil, "Failed to call put correct number of times")
	// }
//}





// func TestGetAuth(t *testing.T) {
// 	mockNetStateService := new(MockedNetState)

// 	_, err := mockNetStateService.Get(ctx, p.New("file/path/1"), []byte("wrong key"))
// 	mockNetStateService.On("Get", ctx, p.New("file/path/1"),[]byte("wrong key")).Return(nil, err)

// 	if err == nil {
// 		mockNetStateService.HandleErr(nil, "Failed to Get because of wrong auth key")
// 	}

// 	mockNetStateService.AssertExpectations(t)
// }

// func TestPutAuth(t *testing.T) {
// 	mockNetStateService := new(MockedNetState)

// 	path := p.New("file/path")
// 	pr := MakePointer(path, false)

// 	err := mockNetStateService.Put(ctx, path, pr.Pointer, pr.APIKey)
// 	mockNetStateService.On("Put", ctx, path, pr.Pointer, pr.APIKey).Return(nil, err)

// 	if err == nil {
// 		mockNetStateService.HandleErr(nil, "Failed to error for wrong auth key")
// 	}
// 	mockNetStateService.AssertExpectations(t)
// }

// func TestDelete(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	pre := nt.mdb.DeleteCalled

// 	reqs := MakePointers(1)
// 	_, err := nt.c.Put(ctx, &reqs[0])
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to put")
// 	}

// 	delReq := pb.DeleteRequest{
// 		Path:   []byte("file/path/1"),
// 		APIKey: []byte("abc123"),
// 	}
// 	_, err = nt.c.Delete(ctx, &delReq)
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to delete")
// 	}
// 	if pre+1 != nt.mdb.DeleteCalled {
// 		nt.HandleErr(nil, "Failed to call Delete correct number of times")
// 	}
// }

// func TestDeleteAuth(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	reqs := MakePointers(1)
// 	_, err := nt.c.Put(ctx, &reqs[0])
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to put")
// 	}

// 	delReq := pb.DeleteRequest{
// 		Path:   []byte("file/path/1"),
// 		APIKey: []byte("wrong key"),
// 	}
// 	_, err = nt.c.Delete(ctx, &delReq)
// 	if err == nil {
// 		nt.HandleErr(nil, "Failed to error with wrong auth key")
// 	}
// }

// func TestList(t *testing.T) {
// 	// nt := NewNetStateClientTest(t)
// 	// defer nt.Close()
// 	mockNetStateService := new(MockedNetState)

// 	reqs := MakePointers(4)
// 	for _, req := range reqs {
// 		mockNetStateService.Put(req)
// 	}

// 	listReq := pb.ListRequest{
// 		StartingPathKey: []byte("file/path/2"),
// 		Limit:           5,
// 		APIKey:          []byte("abc123"),
// 	}
// 	listRes := nt.List(listReq)
// 	if listRes.Truncated {
// 		nt.HandleErr(nil, "Expected list slice to not be truncated")
// 	}
// 	if !bytes.Equal(listRes.Paths[0], []byte("file/path/2")) {
// 		nt.HandleErr(nil, "Failed to list correct file paths")
// 	}
// }

// func TestListTruncated(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	reqs := MakePointers(3)
// 	for _, req := range reqs {
// 		_, err := nt.c.Put(ctx, &req)
// 		if err != nil {
// 			nt.HandleErr(err, "Failed to put")
// 		}
// 	}

// 	listReq := pb.ListRequest{
// 		StartingPathKey: []byte("file/path/1"),
// 		Limit:           1,
// 		APIKey:          []byte("abc123"),
// 	}
// 	listRes, err := nt.c.List(ctx, &listReq)
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to list file paths")
// 	}
// 	if !listRes.Truncated {
// 		nt.HandleErr(nil, "Expected list slice to be truncated")
// 	}
// }

// func TestListWithoutStartingKey(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	reqs := MakePointers(3)
// 	for _, req := range reqs {
// 		_, err := nt.c.Put(ctx, &req)
// 		if err != nil {
// 			nt.HandleErr(err, "Failed to put")
// 		}
// 	}

// 	listReq := pb.ListRequest{
// 		Limit:  3,
// 		APIKey: []byte("abc123"),
// 	}
// 	listRes, err := nt.c.List(ctx, &listReq)
// 	if err != nil {
// 		nt.HandleErr(err, "Failed to list without starting key")
// 	}

// 	if !bytes.Equal(listRes.Paths[2], []byte("file/path/3")) {
// 		nt.HandleErr(nil, "Failed to list correct paths")
// 	}
// }

// func TestListWithoutLimit(t *testing.T) {
// 	nt := NewNetStateClientTest(t)
// 	defer nt.Close()

// 	listReq := pb.ListRequest{
// 		StartingPathKey: []byte("file/path/3"),
// 		APIKey:          []byte("abc123"),
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
