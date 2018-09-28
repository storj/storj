package audit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"io"
	"crypto/ecdsa"
	"crypto"
	"io/ioutil"
	"path/filepath"
	"log"
	"os"
	"net"
	"strings"
	"bytes"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/storage/teststore"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	"storj.io/storj/pkg/piecestore/rpc/server"
	//"storj.io/storj/pkg/piecestore/rpc/client"
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

//R***********R***********/PIECESTORE SERVER/***********R***********R********//
func newTestServerStruct(t *testing.T) (*server.Server, func()) {
	fmt.Println("temp dir")
	tmp, err := ioutil.TempDir("", "storj-piecestore")
	if err != nil {
		log.Fatalf("failed temp-dir: %v", err)
	}

	tempDBPath := filepath.Join(tmp, "test.db")
	tempDir := filepath.Join(tmp, "test-data", "3000")

	psDB, err := psdb.Open(ctx, tempDir, tempDBPath)
	if err != nil {
		t.Fatalf("failed open psdb: %v", err)
	}

	server := &server.Server{DataDir: tempDir, DB: psDB}
	return server, func() {
		if serr := server.Stop(ctx); serr != nil {
			t.Fatal(serr)
		}
		// TODO:fix this error check
		_ = os.RemoveAll(tmp)
		// if err := os.RemoveAll(tmp); err != nil {
		// 	t.Fatal(err)
		// }
	}
}

func connect(addr string, o ...grpc.DialOption) (pb.PieceStoreRoutesClient, *grpc.ClientConn) {
	conn, err := grpc.Dial(addr, o...)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	c := pb.NewPieceStoreRoutesClient(conn)

	return c, conn
}

type TestServer struct {
	s        *server.Server
	scleanup func()
	grpcs    *grpc.Server
	conn     *grpc.ClientConn
	c        pb.PieceStoreRoutesClient
	k        crypto.PrivateKey
}

func NewTestServer(t *testing.T) *TestServer {
	check := func(e error) {
		if !assert.NoError(t, e) {
			t.Fail()
		}
	}

	caS, err := provider.NewCA(context.Background(), 12, 4)
	check(err)
	fiS, err := caS.NewIdentity()
	check(err)
	so, err := fiS.ServerOption()
	check(err)

	caC, err := provider.NewCA(context.Background(), 12, 4)
	check(err)
	fiC, err := caC.NewIdentity()
	check(err)
	co, err := fiC.DialOption()
	check(err)

	s, cleanup := newTestServerStruct(t)
	grpcs := grpc.NewServer(so)

	k, ok := fiC.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	ts := &TestServer{s: s, scleanup: cleanup, grpcs: grpcs, k: k}
	addr := ts.start()
	ts.c, ts.conn = connect(addr, co)

	return ts
}

func (TS *TestServer) start() (addr string) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	pb.RegisterPieceStoreRoutesServer(TS.grpcs, TS.s)

	go func() {
		if err := TS.grpcs.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	return lis.Addr().String()
}

func (TS *TestServer) Stop() {
	if err := TS.conn.Close(); err != nil {
		panic(err)
	}
	TS.grpcs.Stop()
	TS.scleanup()
}

func serializeData(ba *pb.RenterBandwidthAllocation_Data) []byte {
	data, _ := proto.Marshal(ba)
	return data
}

// func writeFileToDir(name, dir string) error {
// 	fmt.Println("writeFiletoDir")
// 	file, err := pstore.StoreWriter(name, dir)
// 	if err != nil {
// 		return err
// 	}

// 	// Close when finished
// 	_, err = io.Copy(file, bytes.NewReader([]byte("butts")))
// 	if err != nil {
// 		_ = file.Close()
// 		return err
// 	}
// 	return file.Close()
// }

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

		//PointerDB instantation
		db := teststore.New()
		c := pointerdb.Config{MaxInlineSegmentSize: 8000}
		pdbw := newPointerDBWrapper(pointerdb.NewServer(db, zap.NewNop(), c))

		//PieceStore server instantiation
		ts := NewTestServer(t)
		defer ts.Stop()

	t.Run("List", func(t *testing.T) {
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

				// create a pdb client and instance of audit
				pdbc := pdbclient.New(pdbw, tt.APIKey)
				
				psc, err := client.NewPSClient(ts.conn, 1024*32, ts.k )
				
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

	t.Run("GetPieceInfo", func(t *testing.T) {
		for _, tt := range tests {
			pdbc := pdbclient.New(pdbw, tt.APIKey)
			psc, err := client.NewPSClient(ts.conn, 1024*32, ts.k)
				if err != nil {
					t.Error("cant instantiate the piece store client")
				}
			
				// PUT into //
				// hardcoded now for  testing to match tt.path for above
				var pieceIDTest = client.PieceID("G93UC6ccvNrMeiP8ogfdEy5bd8KQ617oGJhtKRgxz5Mq")
				r := strings.NewReader("some io.Reader stream to be read\n")
				s := io.NewSectionReader(r, 5, 17)
				var ttl = time.Now().Add(24 * time.Hour)

				err = psc.Put(ctx,pieceIDTest,s,ttl, &pb.PayerBandwidthAllocation{})
				if err != nil {
					fmt.Println("we have a n err putting into db: ", err)
					t.Errorf("error in psc put")
				}
			//func (client *Client) Put(ctx context.Context, id PieceID, data io.Reader, ttl time.Time, ba *pb.PayerBandwidthAllocation) error {

			
			a := NewAudit(pdbc, psc)
			
			pieceID, size, err := a.GetPieceInfo(ctx, tt.path)
			fmt.Print("not to fail: ", pieceID, size)
			
			if err != nil {
				fmt.Println(err)
				t.Error("error in getting pieceID")
			}
		}
	})


	t.Run("GetStripe", func(t *testing.T) {
		for _, tt := range tests {
			pdbc := pdbclient.New(pdbw, tt.APIKey)
			psc, err := client.NewPSClient(ts.conn, 1024*32, ts.k)
				if err != nil {
					t.Error("cant instantiate the piece store client")
				}
			a := NewAudit(pdbc, psc)
			pieceID, size, err := a.GetPieceInfo(ctx, tt.path)
			if err != nil {
				t.Error("error is getting piece info")
			}

			// fmt.Println("pieceid size getstripe:  ", size)
			// fmt.Println("this is piece id getstripe: ", pieceID)


			ranger, err := a.GetStripe(ctx, pieceID, size, &pb.PayerBandwidthAllocation{})
			if err != nil {
				t.Error("error in getting stripe")
			}
			fmt.Println("this is ranger, piece id, size in testing: ", ranger,`\n`, pieceID, `\n`, size)
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
