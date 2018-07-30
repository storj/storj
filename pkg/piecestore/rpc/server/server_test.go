// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	pb "storj.io/storj/protos/piecestore"
)

var ctx = context.Background()

func writeFileToDir(name, dir string) error {
	file, err := pstore.StoreWriter(name, dir)
	if err != nil {
		return err
	}

	// Close when finished
	defer file.Close()

	_, err = io.Copy(file, bytes.NewReader([]byte("butts")))

	return err
}

func TestPiece(t *testing.T) {
	s := newTestServerStruct()

	grpcs := grpc.NewServer()

	go startServer(s, grpcs)
	defer grpcs.Stop()

	c, conn := connect()

	defer conn.Close()

	db := s.DB.DB

	defer cleanup(db)

	if err := writeFileToDir("11111111111111111111", s.DataDir); err != nil {
		t.Errorf("Error: %v\nCould not create test piece", err)
		return
	}

	defer pstore.Delete("11111111111111111111", s.DataDir)

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
			err:        "rpc error: code = Unknown desc = argError: Invalid id length",
		},
		{ // server should err with nonexistent file
			id:         "22222222222222222222",
			size:       5,
			expiration: 9999999999,
			err:        fmt.Sprintf("rpc error: code = Unknown desc = stat %s: no such file or directory", path.Join(os.TempDir(), "/test-data/3000/22/22/2222222222222222")),
		},
	}

	for _, tt := range tests {
		t.Run("should return expected PieceSummary values", func(t *testing.T) {
			assert := assert.New(t)

			// simulate piece TTL entry
			_, err := db.Exec(fmt.Sprintf(`INSERT INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, 1234567890, tt.expiration))
			assert.Nil(err)

			defer db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, tt.id))

			req := &pb.PieceId{Id: tt.id}
			resp, err := c.Piece(ctx, req)

			if tt.err != "" {
				assert.NotNil(err)
				assert.Equal(tt.err, err.Error())
				return
			}

			assert.Nil(err)

			assert.Equal(tt.id, resp.Id)
			assert.Equal(tt.size, resp.Size)
			assert.Equal(tt.expiration, resp.Expiration)
		})
	}
}

func TestRetrieve(t *testing.T) {
	s := newTestServerStruct()

	grpcs := grpc.NewServer()

	go startServer(s, grpcs)
	defer grpcs.Stop()

	c, conn := connect()

	defer conn.Close()

	db := s.DB.DB

	defer cleanup(db)

	// simulate piece stored with farmer
	if err := writeFileToDir("11111111111111111111", s.DataDir); err != nil {
		t.Errorf("Error: %v\nCould not create test piece", err)
		return
	}

	defer pstore.Delete("11111111111111111111", s.DataDir)

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
			content:   []byte("butts"),
			err:       "",
		},
		{ // should successfully retrieve data with lower allocations
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  3,
			allocSize: 3,
			offset:    0,
			content:   []byte("but"),
			err:       "",
		},
		{ // should successfully retrieve data
			id:        "11111111111111111111",
			reqSize:   -1,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("butts"),
			err:       "",
		},
		{ // server should err with invalid id
			id:        "123",
			reqSize:   5,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("butts"),
			err:       "rpc error: code = Unknown desc = argError: Invalid id length",
		},
		{ // server should err with nonexistent file
			id:        "22222222222222222222",
			reqSize:   5,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("butts"),
			err:       fmt.Sprintf("rpc error: code = Unknown desc = stat %s: no such file or directory", path.Join(os.TempDir(), "/test-data/3000/22/22/2222222222222222")),
		},
		{ // server should return expected content and respSize with offset and excess reqSize
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  4,
			allocSize: 5,
			offset:    1,
			content:   []byte("utts"),
			err:       "",
		},
		{ // server should return expected content with reduced reqSize
			id:        "11111111111111111111",
			reqSize:   4,
			respSize:  4,
			allocSize: 5,
			offset:    0,
			content:   []byte("butt"),
			err:       "",
		},
	}

	for _, tt := range tests {
		t.Run("should return expected PieceRetrievalStream values", func(t *testing.T) {
			assert := assert.New(t)
			stream, err := c.Retrieve(ctx)

			// send piece database
			err = stream.Send(&pb.PieceRetrieval{PieceData: &pb.PieceRetrieval_PieceData{Id: tt.id, Size: tt.reqSize, Offset: tt.offset}})
			assert.Nil(err)

			totalAllocated := int64(0)
			var resp *pb.PieceRetrievalStream
			for totalAllocated < tt.respSize {
				// Send bandwidth bandwidthAllocation
				totalAllocated += tt.allocSize
				err = stream.Send(&pb.PieceRetrieval{Bandwidthallocation: &pb.BandwidthAllocation{Signature: []byte{'A', 'B'}, Data: &pb.BandwidthAllocation_Data{Payer: "payer-id", Renter: "renter-id", Size: tt.allocSize, Total: totalAllocated}}})
				assert.Nil(err)

				resp, err = stream.Recv()
				if tt.err != "" {
					assert.NotNil(err)
					assert.Equal(tt.err, err.Error())
					return
				}

			}

			assert.Nil(err)
			assert.Equal(tt.respSize, resp.Size)
			assert.Equal(tt.content, resp.Content)
		})
	}
}

func TestStore(t *testing.T) {
	s := newTestServerStruct()

	grpcs := grpc.NewServer()

	go startServer(s, grpcs)
	defer grpcs.Stop()

	c, conn := connect()

	defer conn.Close()

	db := s.DB.DB

	defer cleanup(db)

	tests := []struct {
		id            string
		ttl           int64
		content       []byte
		message       string
		totalReceived int64
		err           string
	}{
		{ // should successfully store data
			id:            "99999999999999999999",
			ttl:           9999999999,
			content:       []byte("butts"),
			message:       "OK",
			totalReceived: 5,
			err:           "",
		},
		{ // should err with invalid id length
			id:            "butts",
			ttl:           9999999999,
			content:       []byte("butts"),
			message:       "",
			totalReceived: 0,
			err:           "rpc error: code = Unknown desc = argError: Invalid id length",
		},
		{ // should err with piece ID not specified
			id:            "",
			ttl:           9999999999,
			content:       []byte("butts"),
			message:       "",
			totalReceived: 0,
			err:           "rpc error: code = Unknown desc = store error: Piece ID not specified",
		},
	}

	for _, tt := range tests {
		t.Run("should return expected PieceStoreSummary values", func(t *testing.T) {
			assert := assert.New(t)
			stream, err := c.Store(ctx)
			assert.Nil(err)

			// Write the buffer to the stream we opened earlier
			err = stream.Send(&pb.PieceStore{Piecedata: &pb.PieceStore_PieceData{Id: tt.id, Ttl: tt.ttl}})
			assert.Nil(err)

			// Send Bandwidth Allocation Data
			msg := &pb.PieceStore{
				Piecedata: &pb.PieceStore_PieceData{Content: tt.content},
				Bandwidthallocation: &pb.BandwidthAllocation{
					Signature: []byte{'A', 'B'},
					Data: &pb.BandwidthAllocation_Data{
						Payer: "payer-id", Renter: "renter-id", Size: int64(len(tt.content)), Total: int64(len(tt.content)),
					},
				},
			}

			// Write the buffer to the stream we opened earlier
			err = stream.Send(msg)
			assert.Nil(err)

			resp, err := stream.CloseAndRecv()
			if tt.err != "" {
				assert.Equal(tt.err, err.Error())
				return
			}

			assert.Nil(err)

			defer db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, tt.id))

			// check db to make sure agreement and signature were stored correctly
			rows, err := db.Query(`SELECT * FROM bandwidth_agreements`)
			assert.Nil(err)

			defer rows.Close()
			for rows.Next() {
				var (
					agreement []byte
					signature []byte
				)

				err = rows.Scan(&agreement, &signature)
				assert.Nil(err)

				decoded := &pb.BandwidthAllocation_Data{}

				err = proto.Unmarshal(agreement, decoded)

				assert.Equal(msg.Bandwidthallocation.GetSignature(), signature)
				assert.Equal(msg.Bandwidthallocation.Data.GetPayer(), decoded.GetPayer())
				assert.Equal(msg.Bandwidthallocation.Data.GetRenter(), decoded.GetRenter())
				assert.Equal(msg.Bandwidthallocation.Data.GetSize(), decoded.GetSize())
				assert.Equal(msg.Bandwidthallocation.Data.GetTotal(), decoded.GetTotal())

			}
			err = rows.Err()
			assert.Nil(err)

			assert.Equal(tt.message, resp.Message)
			assert.Equal(tt.totalReceived, resp.TotalReceived)
		})
	}
}

func TestDelete(t *testing.T) {
	s := newTestServerStruct()

	grpcs := grpc.NewServer()

	go startServer(s, grpcs)
	defer grpcs.Stop()

	c, conn := connect()

	defer conn.Close()

	db := s.DB.DB

	defer cleanup(db)

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
			message: "rpc error: code = Unknown desc = argError: Invalid id length",
			err:     "rpc error: code = Unknown desc = argError: Invalid id length",
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

			// simulate piece stored with farmer
			if err := writeFileToDir("11111111111111111111", s.DataDir); err != nil {
				t.Errorf("Error: %v\nCould not create test piece", err)
				return
			}

			// simulate piece TTL entry
			_, err := db.Exec(fmt.Sprintf(`INSERT INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, 1234567890, 1234567890))
			assert.Nil(err)

			defer db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, tt.id))

			defer pstore.Delete("11111111111111111111", s.DataDir)

			req := &pb.PieceDelete{Id: tt.id}
			resp, err := c.Delete(ctx, req)

			if tt.err != "" {
				assert.Equal(tt.err, err.Error())
				return
			}

			assert.Nil(err)
			assert.Equal(tt.message, resp.Message)

			// if test passes, check if file was indeed deleted
			filePath, err := pstore.PathByID(tt.id, s.DataDir)
			if _, err = os.Stat(filePath); os.IsNotExist(err) != true {
				t.Errorf("File not deleted")
				return
			}
		})
	}
}

func newTestServerStruct() *Server {
	tempDBPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-test.db", time.Now().String()))

	psDB, err := psdb.OpenPSDB(tempDBPath)
	if err != nil {
		log.Fatal(err)
	}

	tempDir := filepath.Join(os.TempDir(), "test-data", "3000")

	return &Server{DataDir: tempDir, DB: psDB}
}

func connect() (pb.PieceStoreRoutesClient, *grpc.ClientConn) {
	conn, err := grpc.Dial("localhost:3000", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	c := pb.NewPieceStoreRoutesClient(conn)

	return c, conn
}

func startServer(s *Server, grpcs *grpc.Server) {
	lis, err := net.Listen("tcp", ":3000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	pb.RegisterPieceStoreRoutesServer(grpcs, s)
	if err := grpcs.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func cleanup(db *sql.DB) {
	db.Close()
	os.RemoveAll(filepath.Join(os.TempDir(), "test-data"))
	os.Remove(filepath.Join(os.TempDir(), "test.db"))
	return
}

func TestMain(m *testing.M) {
	m.Run()
}
