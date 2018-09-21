package audit

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	grpc "google.golang.org/grpc"

	p "storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	pdbclient "storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/storage/teststore"
)

const (
	noLimitGiven        = "limit not given"
	pointerdbClientPort = ":8080"
)

var (
	ctx             = context.Background()
	ErrNoLimitGiven = errors.New(noLimitGiven)
	APIKey          = []byte("abc123")
)

type pointerDBWrapper struct {
	s pb.PointerDBServer
}

func newPointerDBWrapper(pdbs pb.PointerDBServer) *pointerDBWrapper {
	return &pointerDBWrapper{pdbs}
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

// func (pbd *PointerDBWrapper) Delete(ctx context.Context, in *pb.DeleteRequest, opts ...grpc.CallOption) (*pb.DeleteResponse, error)

func TestAuditSegment(t *testing.T) {

	t.Run("List", func(t *testing.T) {

		tests := []struct {
			bm         string
			path       p.Path
			APIKey     []byte
			startAfter p.Path
			limit      int
			items      []pdbclient.ListItem
			more       bool
			err        error
		}{
			{
				bm:         "should fail with no limit given",
				path:       p.New("file1/file2"),
				APIKey:     nil,
				startAfter: p.New("file3/file4"),
				limit:      0,
				items:      nil,
				more:       false,
				err:        ErrNoLimitGiven,
			},
		}

		for i, tt := range tests {
			t.Run(tt.bm, func(t *testing.T) {
				assert := assert.New(t)
				errTag := fmt.Sprintf("Test case #%d", i)

				// create a pointer and put in db
				putRequest := makePointer(tt.path, tt.APIKey)
				fmt.Println("this is the pr: ", putRequest)

				db := teststore.New()
				c := pointerdb.Config{MaxInlineSegmentSize: 8000}

				pdbw := newPointerDBWrapper(pointerdb.NewServer(db, zap.NewNop(), c))
				req := pb.PutRequest{Path: tt.path.String(), Pointer: putRequest.Pointer, APIKey: tt.APIKey}

				_, err := pdbw.Put(ctx, &req)

				fmt.Println("this is the err for put request: ", errTag, err)

				if err != nil {
					assert.NotNil(t, err, errTag)
				} else {
					assert.Nil(t, err, errTag)
				}

				// call LIST
				//items, more, err := pdbw.List(ctx, tt.startAfter, tt.limit)

				// if err != nil {
				// 	assert.NotNil(err)
				// 	//assert.Equal(tt.err, tt.err)
				// 	t.Errorf("Error: %s", err.Error())
				// }

				//fmt.Println("this is items: ", items, more)
				// write rest of  test
			})
		}
	})
}

func makePointer(path p.Path, auth []byte) pb.PutRequest {
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
				},
				PieceId:      "testId",
				RemotePieces: rps,
			},
			Size: int64(1),
		},
		APIKey: auth,
	}
	return pr
}
