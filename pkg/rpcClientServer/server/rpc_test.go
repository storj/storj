// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"testing"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/piecestore"
	pb "storj.io/storj/pkg/rpcClientServer/protobuf"
	"storj.io/storj/pkg/rpcClientServer/server/api"
)

var s = api.Server{"./test-data/3000", "test.db"}
var c pb.PieceStoreRoutesClient
var testHash string = "11111111111111111111"
var testCreated int64 = 1234567890
var testExpires int64 = 9999999999

func TestPiece(t *testing.T) {
  t.Run("should return expected PieceSummary values", func(t *testing.T) {

    // simulate piece stored with farmer
    err := pstore.Store(testHash, bytes.NewReader([]byte("butts")), 5, 0, s.PieceStoreDir)
    if err != nil {
      log.Fatal(err)
    }
    defer pstore.Delete(testHash, s.PieceStoreDir)

    // set up test cases
    tests := []struct{
        hash string
        size int64
        expiration int64
        err string
    } {
        { // should successfully retrieve piece meta-data
          hash: testHash,
          size: 5,
          expiration: testExpires,
          err: "",
        },
        { // server should err with invalid hash
          hash: "123",
          size: 5,
          expiration: testExpires,
          err: "rpc error: code = Unknown desc = argError: Invalid hash length",
        },
        { // server should err with nonexistent file
          hash: "22222222222222222222",
          size: 5,
          expiration: testExpires,
          err: "rpc error: code = Unknown desc = stat test-data/3000/22/22/2222222222222222: no such file or directory",
        },
    }

    for _, tt := range tests {
      req := &pb.PieceHash{Hash: tt.hash}
      resp, err := c.Piece(context.Background(), req)
      if len(tt.err) > 0 {
        if err != nil {
          if err.Error() == tt.err {
            continue
          }
        }
        t.Errorf("\nExpected: %s\nGot: %v\n", tt.err, err)
        continue
      }
      if err != nil && tt.err == "" {
        t.Errorf("\nExpected: %s\nGot: %v\n", tt.err, err)
        continue
      }

      if resp.Hash != tt.hash || resp.Size != tt.size || resp.Expiration != tt.expiration {
          t.Errorf("Expected: %v, %v, %v\nGot: %v, %v, %v\n", tt.hash, tt.size, tt.expiration, resp.Hash, resp.Size, resp.Expiration)
          continue
      }
    }
  })
}

func TestRetrieve(t *testing.T) {
  t.Run("should return expected PieceRetrievalStream values", func(t *testing.T) {

    // simulate piece stored with farmer
    err := pstore.Store(testHash, bytes.NewReader([]byte("butts")), 5, 0, s.PieceStoreDir)
    if err != nil {
      log.Fatal(err)
    }
    defer pstore.Delete(testHash, s.PieceStoreDir)

    // set up test cases
    tests := []struct{
        hash string
        size int64
        offset int64
        content []byte
        err string
    } {
        { // should successfully retrieve data
          hash: testHash,
          size: 5,
          offset: 0,
          content: []byte("butts"),
          err: "",
        },
        { // server should err with invalid hash
          hash: "123",
          size: 5,
          offset: 0,
          content: []byte("butts"),
          err: "rpc error: code = Unknown desc = argError: Invalid hash length",
        },
      }

      for _, tt := range tests {
          req := &pb.PieceRetrieval{Hash: tt.hash, Size: tt.size, StoreOffset: tt.offset}
          stream, err := c.Retrieve(context.Background(), req)
          if err != nil {
            t.Errorf("Unexpected error: %v\n", err)
            continue
          }

          resp, err := stream.Recv()
          if len(tt.err) > 0 {
            if err != nil {
              if err.Error() == tt.err {
                continue
              }
            }
            t.Errorf("\nExpected: %s\nGot: %v\n", tt.err, err)
            continue
          }
          if err != nil && tt.err == "" {
            t.Errorf("\nExpected: %s\nGot: %v\n", tt.err, err)
            continue
          }

          if resp.Size != tt.size || bytes.Equal(resp.Content, tt.content) != true {
            t.Errorf("Expected: %v, %v\nGot: %v, %v\n", tt.size, tt.content, resp.Size, resp.Content)
            continue
          }
      }
  })
}

func TestStore(t *testing.T) {
  t.Run("should return expected PieceStoreSummary values", func(t *testing.T) {
    tests := []struct{
      status int64
      message string
      totalReceived int64
      err string
    } {
        { // should successfully store data
          status: 0,
          message: "OK",
          totalReceived: 5,
          err: "",
        },
        { // server should err with invalid hash
          status: 0,
          message: "OK",
          totalReceived: 5,
          err: "",
        },
      }

      for _, tt := range tests {
          stream, err := c.Store(context.Background())
          if err != nil {
            t.Errorf("Unexpected error: %v\n", err)
            continue
          }

          resp, err := stream.Recv()
          if len(tt.err) > 0 {
            if err != nil {
              if err.Error() == tt.err {
                continue
              }
            }
            t.Errorf("\nExpected: %s\nGot: %v\n", tt.err, err)
            continue
          }
          if err != nil && tt.err == "" {
            t.Errorf("\nExpected: %s\nGot: %v\n", tt.err, err)
            continue
          }

          if resp.Size != tt.size || bytes.Equal(resp.Content, tt.content) != true {
            t.Errorf("Expected: %v, %v\nGot: %v, %v\n", tt.size, tt.content, resp.Size, resp.Content)
            continue
          }
      }
  })
}

// func TestDelete(t *testing.T) {
//   t.Run("should return expected PieceDeleteSummary values", func(t *testing.T) {
//
//   })
// }

func StartServer() {
    lis, err := net.Listen("tcp", ":3000")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }
    grpcs := grpc.NewServer()
    pb.RegisterPieceStoreRoutesServer(grpcs, &s)
    if err := grpcs.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}

func TestMain(m *testing.M) {
  go StartServer()

  // Set up a connection to the Server.
  const address = "localhost:3000"
  conn, err := grpc.Dial(address, grpc.WithInsecure())
  if err != nil {
    fmt.Printf("did not connect: %v", err)
    return
  }
  defer conn.Close()
  c = pb.NewPieceStoreRoutesClient(conn)

  // create temp DB
  db, err := sql.Open("sqlite3", s.DbPath)
  if err != nil {
    log.Fatal(err)
  }
  _, err = db.Exec("CREATE TABLE IF NOT EXISTS `ttl` (`hash` TEXT, `created` INT(10), `expires` INT(10));")
	if err != nil {
		log.Fatal(err)
	}

  _, err = db.Exec(fmt.Sprintf(`INSERT INTO ttl (hash, created, expires) VALUES ("%s", "%d", "%d")`, testHash, testCreated, testExpires))
  if err != nil {
    log.Fatal(err)
  }

	m.Run()

  // clean up temp files
  os.RemoveAll("./test-data")
  db.Close()
  os.Remove("./test.db")
}
