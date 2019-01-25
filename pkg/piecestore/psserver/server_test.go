// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/bwagreement/test"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
)

func TestPiece(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	snID, upID := newTestID(ctx, t), newTestID(ctx, t)
	s, c, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
	defer cleanup()

	if err := writeFile(s, "11111111111111111111"); err != nil {
		t.Errorf("Error: %v\nCould not create test piece", err)
		return
	}

	defer func() { _ = s.storage.Delete("11111111111111111111") }()

	// set up test cases
	tests := []struct {
		id         string
		size       int64
		expiration int64
		err        string
	}{
		{ // should successfully retrieve piece meta-data
			id:         "11111111111111111111",
			size:       5,
			expiration: 9999999999,
			err:        "",
		},
		{ // server should err with invalid id
			id:         "123",
			size:       5,
			expiration: 9999999999,
			err:        "rpc error: code = Unknown desc = piecestore error: invalid id length",
		},
		{ // server should err with nonexistent file
			id:         "22222222222222222222",
			size:       5,
			expiration: 9999999999,
			err: fmt.Sprintf("rpc error: code = Unknown desc = stat %s: no such file or directory", func() string {
				path, _ := s.storage.PiecePath("22222222222222222222")
				return path
			}()),
		},
		{ // server should err with invalid TTL
			id:         "22222222222222222222;DELETE*FROM TTL;;;;",
			size:       5,
			expiration: 9999999999,
			err:        "rpc error: code = Unknown desc = PSServer error: invalid ID",
		},
	}

	for _, tt := range tests {
		t.Run("should return expected PieceSummary values", func(t *testing.T) {
			assert := assert.New(t)

			// simulate piece TTL entry
			_, err := s.DB.DB.Exec(fmt.Sprintf(`INSERT INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, 1234567890, tt.expiration))
			assert.NoError(err)

			defer func() {
				_, err := s.DB.DB.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, tt.id))
				assert.NoError(err)
			}()

			req := &pb.PieceId{Id: tt.id}
			resp, err := c.Piece(ctx, req)

			if tt.err != "" {
				assert.NotNil(err)
				if runtime.GOOS == "windows" && strings.Contains(tt.err, "no such file or directory") {
					//TODO (windows): ignoring for windows due to different underlying error
					return
				}
				assert.Equal(tt.err, err.Error())
				return
			}

			assert.NoError(err)

			assert.Equal(tt.id, resp.GetId())
			assert.Equal(tt.size, resp.GetPieceSize())
			assert.Equal(tt.expiration, resp.GetExpirationUnixSec())
		})
	}
}

func TestRetrieve(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	snID, upID := newTestID(ctx, t), newTestID(ctx, t)
	s, c, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
	defer cleanup()

	if err := writeFile(s, "11111111111111111111"); err != nil {
		t.Errorf("Error: %v\nCould not create test piece", err)
		return
	}

	defer func() { _ = s.storage.Delete("11111111111111111111") }()

	// set up test cases
	tests := []struct {
		id        string
		reqSize   int64
		respSize  int64
		allocSize int64
		offset    int64
		content   []byte
		err       string
	}{
		{ // should successfully retrieve data
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
			err:       "",
		},
		{ // should successfully retrieve data in customizeable increments
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  5,
			allocSize: 2,
			offset:    0,
			content:   []byte("xyzwq"),
			err:       "",
		},
		{ // should successfully retrieve data with lower allocations
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  3,
			allocSize: 3,
			offset:    0,
			content:   []byte("xyz"),
			err:       "",
		},
		{ // should successfully retrieve data
			id:        "11111111111111111111",
			reqSize:   -1,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
			err:       "",
		},
		{ // server should err with invalid id
			id:        "123",
			reqSize:   5,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
			err:       "rpc error: code = Unknown desc = piecestore error: invalid id length",
		},
		{ // server should err with nonexistent file
			id:        "22222222222222222222",
			reqSize:   5,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
			err: fmt.Sprintf("rpc error: code = Unknown desc = retrieve error: stat %s: no such file or directory", func() string {
				path, _ := s.storage.PiecePath("22222222222222222222")
				return path
			}()),
		},
		{ // server should return expected content and respSize with offset and excess reqSize
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  4,
			allocSize: 5,
			offset:    1,
			content:   []byte("yzwq"),
			err:       "",
		},
		{ // server should return expected content with reduced reqSize
			id:        "11111111111111111111",
			reqSize:   4,
			respSize:  4,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzw"),
			err:       "",
		},
	}

	for _, tt := range tests {
		t.Run("should return expected PieceRetrievalStream values", func(t *testing.T) {
			assert := assert.New(t)
			stream, err := c.Retrieve(ctx)
			assert.NoError(err)

			// send piece database
			err = stream.Send(&pb.PieceRetrieval{PieceData: &pb.PieceRetrieval_PieceData{Id: tt.id, PieceSize: tt.reqSize, Offset: tt.offset}})
			assert.NoError(err)

			pba, err := test.GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, snID, upID, time.Hour)
			assert.NoError(err)

			totalAllocated := int64(0)
			var data string
			var totalRetrieved = int64(0)
			var resp *pb.PieceRetrievalStream
			for totalAllocated < tt.respSize {
				// Send bandwidth bandwidthAllocation
				totalAllocated += tt.allocSize

				rba, err := test.GenerateRenterBandwidthAllocation(pba, snID.ID, upID, totalAllocated)
				assert.NoError(err)

				err = stream.Send(
					&pb.PieceRetrieval{
						BandwidthAllocation: rba,
					},
				)
				assert.NoError(err)

				resp, err = stream.Recv()
				if tt.err != "" {
					assert.NotNil(err)
					if runtime.GOOS == "windows" && strings.Contains(tt.err, "no such file or directory") {
						//TODO (windows): ignoring for windows due to different underlying error
						return
					}
					assert.Equal(tt.err, err.Error())
					return
				}

				data = fmt.Sprintf("%s%s", data, string(resp.GetContent()))
				totalRetrieved += resp.GetPieceSize()
			}

			assert.NoError(err)
			assert.NotNil(resp)
			if resp != nil {
				assert.Equal(tt.respSize, totalRetrieved)
				assert.Equal(string(tt.content), data)
			}
		})
	}
}

func TestStore(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satID := newTestID(ctx, t)

	tests := []struct {
		id            string
		satelliteID   storj.NodeID
		whitelist     []storj.NodeID
		ttl           int64
		content       []byte
		message       string
		totalReceived int64
		err           string
	}{
		{ // should successfully store data with no approved satellites
			id:            "99999999999999999999",
			satelliteID:   satID.ID,
			whitelist:     []storj.NodeID{},
			ttl:           9999999999,
			content:       []byte("xyzwq"),
			message:       "OK",
			totalReceived: 5,
			err:           "",
		},
		{ // should err with invalid id length
			id:            "butts",
			satelliteID:   satID.ID,
			whitelist:     []storj.NodeID{satID.ID},
			ttl:           9999999999,
			content:       []byte("xyzwq"),
			message:       "",
			totalReceived: 0,
			err:           "rpc error: code = Unknown desc = piecestore error: invalid id length",
		},
		{ // should err with piece ID not specified
			id:            "",
			satelliteID:   satID.ID,
			whitelist:     []storj.NodeID{satID.ID},
			ttl:           9999999999,
			content:       []byte("xyzwq"),
			message:       "",
			totalReceived: 0,
			err:           "rpc error: code = Unknown desc = store error: piece ID not specified",
		},
	}

	for _, tt := range tests {
		t.Run("should return expected PieceStoreSummary values", func(t *testing.T) {
			snID, upID := newTestID(ctx, t), newTestID(ctx, t)
			s, c, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
			defer cleanup()
			db := s.DB.DB

			assert := assert.New(t)
			stream, err := c.Store(ctx)
			assert.NoError(err)

			// Write the buffer to the stream we opened earlier
			err = stream.Send(&pb.PieceStore{PieceData: &pb.PieceStore_PieceData{Id: tt.id, ExpirationUnixSec: tt.ttl}})
			assert.NoError(err)
			// Send Bandwidth Allocation Data
			pba, err := test.GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_PUT, snID, upID, time.Hour)
			assert.NoError(err)
			rba, err := test.GenerateRenterBandwidthAllocation(pba, snID.ID, upID, tt.totalReceived)
			assert.NoError(err)
			msg := &pb.PieceStore{
				PieceData:           &pb.PieceStore_PieceData{Content: tt.content},
				BandwidthAllocation: rba,
			}
			// Write the buffer to the stream we opened earlier
			err = stream.Send(msg)
			if err != io.EOF && err != nil {
				assert.NoError(err)
			}

			resp, err := stream.CloseAndRecv()
			if tt.err != "" {
				assert.NotNil(err)
				assert.True(strings.HasPrefix(err.Error(), tt.err), "expected")
				return
			}
			if !assert.NoError(err) {
				t.Fatal(err)
			}

			defer func() {
				_, err := db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, tt.id))
				assert.NoError(err)
			}()

			// check db to make sure agreement and signature were stored correctly
			rows, err := db.Query(`SELECT agreement, signature FROM bandwidth_agreements`)
			assert.NoError(err)

			defer func() { assert.NoError(rows.Close()) }()
			for rows.Next() {
				var (
					agreement []byte
					signature []byte
				)

				err = rows.Scan(&agreement, &signature)
				assert.NoError(err)

				decoded := &pb.RenterBandwidthAllocation_Data{}

				err = proto.Unmarshal(agreement, decoded)
				assert.NoError(err)
				assert.Equal(msg.BandwidthAllocation.GetSignature(), signature)
				assert.True(proto.Equal(pba, decoded.GetPayerAllocation()))
				assert.Equal(int64(len(tt.content)), decoded.GetTotal())

			}
			err = rows.Err()
			assert.NoError(err)
			if !assert.NotNil(resp) {
				t.Fatalf("resp is null")
			}
			assert.Equal(tt.message, resp.Message)
			assert.Equal(tt.totalReceived, resp.TotalReceived)
		})
	}
}

func TestPbaValidation(t *testing.T) {
	ctx := testcontext.New(t)
	snID, upID := newTestID(ctx, t), newTestID(ctx, t)
	satID1, satID2, satID3 := newTestID(ctx, t), newTestID(ctx, t), newTestID(ctx, t)
	defer ctx.Cleanup()

	tests := []struct {
		satelliteID storj.NodeID
		uplinkID    storj.NodeID
		whitelist   []storj.NodeID
		action      pb.PayerBandwidthAllocation_Action
		err         string
	}{
		{ // unapproved satellite id
			satelliteID: satID1.ID,
			uplinkID:    upID.ID,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID, satID3.ID},
			action:      pb.PayerBandwidthAllocation_PUT,
			err:         "rpc error: code = Unknown desc = store error: Satellite ID not approved",
		},
		{ // missing satellite id
			satelliteID: storj.NodeID{},
			uplinkID:    upID.ID,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID, satID3.ID},
			action:      pb.PayerBandwidthAllocation_PUT,
			err:         "rpc error: code = Unknown desc = store error: payer bandwidth allocation: missing satellite id",
		},
		{ // missing uplink id
			satelliteID: satID1.ID,
			uplinkID:    storj.NodeID{},
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID, satID3.ID},
			action:      pb.PayerBandwidthAllocation_PUT,
			err:         "rpc error: code = Unknown desc = store error: payer bandwidth allocation: missing uplink id",
		},
		{ // wrong action type
			satelliteID: satID1.ID,
			uplinkID:    upID.ID,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID, satID3.ID},
			action:      pb.PayerBandwidthAllocation_GET,
			err:         "rpc error: code = Unknown desc = store error: payer bandwidth allocation: invalid action GET",
		},
	}

	for _, tt := range tests {
		t.Run("should validate payer bandwidth allocation struct", func(t *testing.T) {
			s, c, cleanup := NewTest(ctx, t, snID, upID, tt.whitelist)
			defer cleanup()

			assert := assert.New(t)
			stream, err := c.Store(ctx)
			assert.NoError(err)

			//cleanup incase tests previously paniced
			_ = s.storage.Delete("99999999999999999999")
			// Write the buffer to the stream we opened earlier
			err = stream.Send(&pb.PieceStore{PieceData: &pb.PieceStore_PieceData{Id: "99999999999999999999", ExpirationUnixSec: 9999999999}})
			assert.NoError(err)
			// Send Bandwidth Allocation Data
			content := []byte("content")
			pba, err := test.GeneratePayerBandwidthAllocation(tt.action, satID1, upID, time.Hour)
			assert.NoError(err)
			rba, err := test.GenerateRenterBandwidthAllocation(pba, snID.ID, upID, int64(len(content)))
			assert.NoError(err)
			msg := &pb.PieceStore{
				PieceData:           &pb.PieceStore_PieceData{Content: content},
				BandwidthAllocation: rba,
			}

			// Write the buffer to the stream we opened earlier
			err = stream.Send(msg)
			if err != io.EOF && err != nil {
				assert.NoError(err)
			}

			_, err = stream.CloseAndRecv()
			if err != nil {
				//assert.NotNil(err)
				t.Log("Expected err string", tt.err)
				t.Log("Actual err.Error:", err.Error())
				assert.Equal(tt.err, err.Error())
				return
			}
		})
	}
}

func TestDelete(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	snID, upID := newTestID(ctx, t), newTestID(ctx, t)
	s, c, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
	defer cleanup()

	db := s.DB.DB

	// set up test cases
	tests := []struct {
		id      string
		message string
		err     string
	}{
		{ // should successfully delete data
			id:      "11111111111111111111",
			message: "OK",
			err:     "",
		},
		{ // should err with invalid id length
			id:      "123",
			message: "rpc error: code = Unknown desc = piecestore error: invalid id length",
			err:     "rpc error: code = Unknown desc = piecestore error: invalid id length",
		},
		{ // should return OK with nonexistent file
			id:      "22222222222222222223",
			message: "OK",
			err:     "",
		},
	}

	for _, tt := range tests {
		t.Run("should return expected PieceDeleteSummary values", func(t *testing.T) {
			assert := assert.New(t)

			// simulate piece stored with storagenode
			if err := writeFile(s, "11111111111111111111"); err != nil {
				t.Errorf("Error: %v\nCould not create test piece", err)
				return
			}

			// simulate piece TTL entry
			_, err := db.Exec(fmt.Sprintf(`INSERT INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, 1234567890, 1234567890))
			assert.NoError(err)

			defer func() {
				_, err := db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, tt.id))
				assert.NoError(err)
			}()

			defer func() {
				assert.NoError(s.storage.Delete("11111111111111111111"))
			}()

			req := &pb.PieceDelete{Id: tt.id}
			resp, err := c.Delete(ctx, req)

			if tt.err != "" {
				assert.Equal(tt.err, err.Error())
				return
			}

			assert.NoError(err)
			assert.Equal(tt.message, resp.GetMessage())

			// if test passes, check if file was indeed deleted
			filePath, err := s.storage.PiecePath(tt.id)
			assert.NoError(err)
			if _, err = os.Stat(filePath); os.IsExist(err) {
				t.Errorf("File not deleted")
				return
			}
		})
	}
}

func NewTest(ctx context.Context, t *testing.T, snID, upID *identity.FullIdentity,
	ids []storj.NodeID) (*Server, pb.PieceStoreRoutesClient, func()) {
	//init ps server backend
	tmp, err := ioutil.TempDir("", "storj-piecestore")
	assert.NoError(t, err)
	tempDBPath := filepath.Join(tmp, "test.db")
	tempDir := filepath.Join(tmp, "test-data", "3000")
	storage := pstore.NewStorage(tempDir)
	psDB, err := psdb.Open(ctx, storage, tempDBPath)
	assert.NoError(t, err)
	verifier := func(authorization *pb.SignedMessage) error {
		return nil
	}
	psServer := &Server{
		log:              zaptest.NewLogger(t),
		storage:          storage,
		DB:               psDB,
		verifier:         verifier,
		totalAllocated:   math.MaxInt64,
		totalBwAllocated: math.MaxInt64,
		whitelist:        ids,
	}
	//init ps server grpc
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	publicConfig := server.Config{Address: "127.0.0.1:0"}
	publicOptions, err := server.NewOptions(snID, publicConfig)
	assert.NoError(t, err)
	grpcServer, err := server.NewServer(publicOptions, listener, nil)
	assert.NoError(t, err)
	pb.RegisterPieceStoreRoutesServer(grpcServer.GRPC(), psServer)
	go func() { assert.NoError(t, grpcServer.Run(ctx)) }()
	//init client
	co, err := upID.DialOption(storj.NodeID{})
	assert.NoError(t, err)
	conn, err := grpc.Dial(listener.Addr().String(), co)
	assert.NoError(t, err)
	psClient := pb.NewPieceStoreRoutesClient(conn)
	//cleanup callback
	cleanup := func() {
		assert.NoError(t, conn.Close())
		assert.NoError(t, psServer.Close())
		assert.NoError(t, psServer.Stop(ctx))
		assert.NoError(t, os.RemoveAll(tmp))
	}
	return psServer, psClient, cleanup
}

func newTestID(ctx context.Context, t *testing.T) *identity.FullIdentity {
	id, err := testidentity.NewTestIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func writeFile(s *Server, pieceID string) error {
	file, err := s.storage.Writer(pieceID)
	if err != nil {
		return err
	}
	_, err = file.Write([]byte("xyzwq"))
	return errs.Combine(err, file.Close())
}
