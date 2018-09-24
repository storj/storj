package audit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"crypto/ecdsa"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/storage/teststore"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/piecestore/rpc/client"
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

func instantiatePSClient(t *testing.T)(psclient client.PSClient, err error){
	ca, err := provider.NewCA(ctx, 12, 4)
	if err != nil {
		t.Error(err)
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		t.Error(err)
	}
	identOpt, err := identity.DialOption()
	if err != nil {
		t.Error(err)
	}

	// Set up connection with rpc server
	var conn *grpc.ClientConn
	conn, err = grpc.Dial(":7777", identOpt)
	if err != nil {
		t.Error("did not connect: ", err)
	}
	defer conn.Close()

	psClient, err := client.NewPSClient(conn, 1024*32, identity.Key.(*ecdsa.PrivateKey))
	if err != nil {
		t.Error("could not initialize PSClient: ", err)
	}
	return psClient, nil
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
				startAfter: paths.New("file3/file4"),
				limit:      10,
				items:      nil,
				more:       false,
				err:        ErrNoLimitGiven,
			},
		}

		db := teststore.New()
		c := pointerdb.Config{MaxInlineSegmentSize: 8000}
		pdbw := newPointerDBWrapper(pointerdb.NewServer(db, zap.NewNop(), c))

	t.Run("List", func(t *testing.T) {
		for i, tt := range tests {
			t.Run(tt.bm, func(t *testing.T) {
				assert1 := assert.New(t)
				errTag := fmt.Sprintf("Test case #%d", i)

				// create a pointer and put in db
				putRequest := makePointer(tt.path, tt.APIKey)

				// create putreq. object
				req := &pb.PutRequest{Path: tt.path.String(), Pointer: putRequest.Pointer, APIKey: tt.APIKey}

				//Put pointer into db
				_, err := pdbw.Put(ctx, req)
				if err != nil {
					t.Fatalf("failed to put %v: error: %v", req.Pointer, err)
				}

				fmt.Println("this is the err for put request: ", errTag, err)

				// create a pdb client and instance of audit
				pdbc := pdbclient.New(pdbw, tt.APIKey)
				psc, err := instantiatePSClient(t)
				if err != nil {
					t.Error("cant instantiate the piece store client")
				}
				a := NewAudit(pdbc, psc)

				// make  a List request
				items, more, err := a.List(ctx, nil, tt.limit)

				if err != nil {
					assert1.NotNil(err)
				}

				fmt.Println("items at 0: ", items[0].Pointer)
				fmt.Println("this is items: ", items, more, err)
			})
		}
	})

	t.Run("GetPieceID", func(t *testing.T) {
		for _, tt := range tests {
			pdbc := pdbclient.New(pdbw, tt.APIKey)
			psc, err := instantiatePSClient(t)
				if err != nil {
					t.Error("cant instantiate the piece store client")
				}
			a := NewAudit(pdbc, psc)
			pieceID, err := a.GetPieceID(ctx, tt.path)
			if err != nil {
				t.Error("error in getting pieceID")
			}
			fmt.Println("this is piece id: ", pieceID)
		}
	})
} // end of all fn

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
