package audit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	//"math/rand"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	//"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/storage/teststore"
	//"storj.io/storj/pkg/storage/meta"
)

const (
	noLimitGiven = "limit not given"
)

var (
	ctx             = context.Background()
	ErrNoLimitGiven = errors.New(noLimitGiven)
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
			startAfter paths.Path
			limit      int
			items      []pdbclient.ListItem
			more       bool
			err        error
		}{
			{
				bm:         "success",
				path:       paths.New("file1/file2"),
				APIKey:     nil,
				startAfter: nil,
				limit:      10,
				items:      nil,
				more:       false,
				err:        nil,
			},
			{
				bm:         "success",
				path:       paths.New("file1/file2/file10/file9"),
				APIKey:     nil,
				startAfter: paths.New("file1"),
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

	t.Run("NextStripe", func(t *testing.T) {
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
				}
				
				// testing to see list works
				//itemList, isMore, err := pointers.List(ctx, nil, nil,nil,true, 10, meta.None)
				
				if err != nil {
					t.Error("cant instantiate the piece store client")
				}

				// make the request
				stripe, err := a.NextStripe(ctx)
				if err != nil {
					fmt.Println("err is in error: ", err)
					assert1.NotNil(err)
				}
				assert1.NotNil(stripe)
			})
		}
	})



	

} // end of all fn

func putIntoDB() {

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
