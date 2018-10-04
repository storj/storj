package audit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	//"math/rand"
	"storj.io/storj/pkg/storage/meta"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	//"github.com/golang/protobuf/proto"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/storage/teststore"
	//"storj.io/storj/pkg/storage/meta"
)

const (
	noPointer = "pointer error: no pointers exist"
)

var (
	ctx             = context.Background()
	ErrNoPointer = errors.New(noPointer)
)

// The client and server implementation are different; 
// This is a  wrapper so the pointerdb client can be implemented

//R***********R***********/PointerDB Wrapper/***********R***********R********//
type pointerDBWrapper struct {
	 s  pb.PointerDBServer
}

func (pbd *pointerDBWrapper) Put(ctx context.Context, in *pb.PutRequest, opts ...grpc.CallOption) (*pb.PutResponse, error) {
	return pbd.s.Put(ctx, in)
}

func (pbd *pointerDBWrapper) Get(ctx context.Context, in *pb.GetRequest, opts ...grpc.CallOption) (*pb.GetResponse, error) {
	return pbd.s.Get(ctx, in)
}

func (pbd *pointerDBWrapper) List(ctx context.Context, in *pb.ListRequest, opts ...grpc.CallOption) (*pb.ListResponse, error) {
	return pbd.s.List(ctx, in)
}

func (pbd *pointerDBWrapper) Delete(ctx context.Context, in *pb.DeleteRequest, opts ...grpc.CallOption) (*pb.DeleteResponse, error) {
	return pbd.s.Delete(ctx, in)
}

func newPointerDBWrapper(pdbs pb.PointerDBServer) pb.PointerDBClient {
	return &pointerDBWrapper{pdbs}
}

func TestAuditSegment(t *testing.T) {
		tests := []struct {
			bm         string
			path       paths.Path
			APIKey     []byte
			limit      int
			items      []pdbclient.ListItem
			more       bool
			err        error
		}{
			{
				bm:         "success-1",
				path:       paths.New("folder1/file1"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success-2",
				path:       paths.New("foodFolder1/file1/file2"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success-3",
				path:       paths.New("foodFolder1/file1/file2/foodFolder2/file3"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success-4",
				path:       paths.New("projectFolder/project1.txt/"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success-5",
				path:       paths.New("newProjectFolder/project2.txt"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success-6",
				path:       paths.New("Pictures/image1.png"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success-7",
				path:       paths.New("Pictures/Nature/mountains.png"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success-8",
				path:       paths.New("Pictures/City/streets.png"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success-9",
				path:       paths.New("Pictures/Animals/Dogs/dogs.png"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success-10",
				path:       paths.New("Random/ビデオ/😶"),
				APIKey:     nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
		}

		//PointerDB instantation
		db := teststore.New()
		c := pointerdb.Config{MaxInlineSegmentSize: 8000}
		pdbw := newPointerDBWrapper(pointerdb.NewServer(db, zap.NewNop(), c))
		pointers := pdbclient.New(pdbw, nil)

		// create a pdb client and instance of audit
		a := NewAudit(pointers)

	t.Run("putToDB", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.bm, func(t *testing.T) {
				assert1 := assert.New(t)
				//errTag := fmt.Sprintf("Test case #%d", i)

				// create a pointer and put in db
				putRequest := makePointer(tt.path, tt.APIKey)

				// create putreq. object
				req := &pb.PutRequest{Path: tt.path.String(), Pointer: putRequest.Pointer, APIKey: tt.APIKey}

				//Put pointer into db
				_, err := pdbw.Put(ctx, req)
				if err != nil {
					t.Fatalf("failed to put %v: error: %v", req.Pointer, err)
					assert1.NotNil(err)
				}
				
				if err != nil {
					t.Error("cant instantiate the piece store client")
				}
			})
		}
	
			//probalisticTest(a, pdbw, pointers)
	
	})//end of teststripe


	t.Run("NextStripe", func(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.bm, func(t *testing.T) {
			assert1 := assert.New(t)
			stripe, err := a.NextStripe(ctx)
			if err != nil {
				fmt.Println("err is in error: ", err)
				assert1.Error(ErrNoPointer)
				assert1.Nil(stripe)
			}

			if stripe != nil {
				assert1.Nil(err)
			}			
		})	
	}
})//end of nextstripefn



} // end of all fn










// test to see how random paths are

	// put 10 things into db, 
	// write some aspect of randomness from the library
	// test the call 100 times
	// store everything in a list
	// get statistical cound of how many things are in the db
	// do some analysis


func probalisticTest(a *Audit, pdbw pb.PointerDBClient, pointers *pdbclient.PointerDB){
	fmt.Println("testing random")

	list,more, err := pointers.List(ctx, nil, nil, nil, true, 10, meta.None)
	fmt.Println("List is: ", list, more, err)
}





func makePointer(path paths.Path, auth []byte) pb.PutRequest {
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
					ErasureShareSize: 2,
				},
				PieceId:      "testId",
				RemotePieces: rps,
			},
			Size: int64(10),
		},
		APIKey: auth,
	}
	return pr
}
