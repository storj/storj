// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/piecestore"
	pb "storj.io/storj/pkg/rpcClientServer/protobuf"
	"storj.io/storj/pkg/rpcClientServer/server/api"
)

var tempDir string = path.Join(os.TempDir(), "test-data", "3000")
var tempDBPath string = path.Join(os.TempDir(), "test.db")
var s = api.Server{tempDir, tempDBPath}
var db *sql.DB
var c pb.PieceStoreRoutesClient
var testHash string = "11111111111111111111"
var testCreatedDate int64 = 1234567890
var testExpiration int64 = 9999999999

func TestPiece(t *testing.T) {
  t.Run("should return expected PieceSummary values", func(t *testing.T) {

    // simulate piece stored with farmer
    _, err := pstore.Store(testHash, bytes.NewReader([]byte("butts")), 5, 0, s.PieceStoreDir)
    if err != nil {
			t.Errorf("Error: %v\nCould not create test piece", err)
			return
    }

		// simulate piece TTL entry
		_, err = db.Exec(fmt.Sprintf(`INSERT INTO ttl (hash, created, expires) VALUES ("%s", "%d", "%d")`, testHash, testCreatedDate, testExpiration))
	  if err != nil {
	    t.Errorf("Error: %v\nCould not make TTL entry", err)
			return
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
          expiration: testExpiration,
          err: "",
        },
        { // server should err with invalid hash
          hash: "123",
          size: 5,
          expiration: testExpiration,
          err: "rpc error: code = Unknown desc = argError: Invalid hash length",
        },
        { // server should err with nonexistent file
          hash: "22222222222222222222",
          size: 5,
          expiration: testExpiration,
          err: fmt.Sprintf("rpc error: code = Unknown desc = stat %stest-data/3000/22/22/2222222222222222: no such file or directory", os.TempDir()),
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
		// clean up DB entry
		_, err = db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE hash="%s"`, testHash))
		if err != nil {
			t.Errorf("Error cleaning test DB entry")
			return
		}
  })
}

func TestRetrieve(t *testing.T) {
  t.Run("should return expected PieceRetrievalStream values", func(t *testing.T) {

    // simulate piece stored with farmer
    _, err := pstore.Store(testHash, bytes.NewReader([]byte("butts")), 5, 0, s.PieceStoreDir)
    if err != nil {
			t.Errorf("Error: %v\nCould not create test piece", err)
			return
    }
    defer pstore.Delete(testHash, s.PieceStoreDir)

    // set up test cases
    tests := []struct{
        hash string
        reqSize int64
				respSize int64
        offset int64
        content []byte
        err string
    } {
        { // should successfully retrieve data
          hash: testHash,
          reqSize: 5,
					respSize: 5,
          offset: 0,
          content: []byte("butts"),
          err: "",
        },
        { // server should err with invalid hash
          hash: "123",
          reqSize: 5,
					respSize: 5,
          offset: 0,
          content: []byte("butts"),
          err: "rpc error: code = Unknown desc = argError: Invalid hash length",
        },
				{ // server should err with nonexistent file
          hash: "22222222222222222222",
          reqSize: 5,
					respSize: 5,
          offset: 0,
          content: []byte("butts"),
          err: fmt.Sprintf("rpc error: code = Unknown desc = stat %stest-data/3000/22/22/2222222222222222: no such file or directory", os.TempDir()),
        },
				{ // server should return expected content and respSize with offset and excess reqSize
          hash: testHash,
          reqSize: 5,
					respSize: 4,
          offset: 1,
          content: []byte("utts"),
          err: "",
        },
				{ // server should return expected content with reduced reqSize
          hash: testHash,
          reqSize: 4,
					respSize: 4,
          offset: 0,
          content: []byte("butt"),
          err: "",
        },
      }

      for _, tt := range tests {
          req := &pb.PieceRetrieval{Hash: tt.hash, Size: tt.reqSize, StoreOffset: tt.offset}
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

          if resp.Size != tt.respSize || bytes.Equal(resp.Content, tt.content) != true {
            t.Errorf("Expected: %v, %v\nGot: %v, %v\n", tt.respSize, tt.content, resp.Size, resp.Content)
            continue
          }
      }
  })
}

func TestStore(t *testing.T) {
  t.Run("should return expected PieceStoreSummary values", func(t *testing.T) {

    tests := []struct{
			hash string
			size int64
			ttl int64
			offset int64
			content []byte
      message string
      totalReceived int64
      err string
    } {
        { // should successfully store data
					hash: testHash,
					size: 5,
					ttl: testExpiration,
					offset: 0,
					content: []byte("butts"),
          message: "Successfully stored data",
          totalReceived: 5,
          err: "",
        },
				{ // should successfully store data
					hash: "butts",
					size: 5,
					ttl: testExpiration,
					offset: 0,
					content: []byte("butts"),
					message: "",
					totalReceived: 0,
					err: "rpc error: code = Unknown desc = argError: Invalid hash length",
				},
				{ // should successfully store data
					hash: "ABCDEFGHIJKLMNOPQRST",
					size: 10,
					ttl: testExpiration,
					offset: 0,
					content: []byte("butts"),
					message: "",
					totalReceived: 5,
					err: "rpc error: code = Unknown desc = Recieved 5 bytes of total 10 bytes",
				},
				{ // should successfully store data
					hash: testHash,
					size: 5,
					ttl: testExpiration,
					offset: 10,
					content: []byte("butts"),
					message: "Successfully stored data",
					totalReceived: 5,
					err: "",
				},
				{ // should successfully store data
					hash: testHash,
					size: 5,
					ttl: testExpiration,
					offset: 0,
					content: []byte(""),
					message: "",
					totalReceived: 0,
					err: "rpc error: code = Unknown desc = No data received",
				},
      }

      for i, tt := range tests {

				tmpfile, err := ioutil.TempFile("", fmt.Sprintf("test%v", i))
				if err != nil {
					t.Errorf("Error: %v\nCould not create test file", err)
					return
				}

				defer os.Remove(tmpfile.Name()) // clean up
				defer tmpfile.Close()

				if _, err := tmpfile.Write(tt.content); err != nil {
					t.Errorf("Error: %v\nCould not create test file", err)
					return
				}
				tmpfile.Seek(0, 0)

        stream, err := c.Store(context.Background())
        if err != nil {
          t.Errorf("Unexpected error: %v\n", err)
          continue
        }
				buffer := make([]byte, 4096)
				tmpfile.Seek(0,0)
				for {
					// Read data from read stream into buffer
					n, err := tmpfile.Read(buffer)
					if err != nil {
						if err == io.EOF {
							break
						}
						t.Errorf("Unexpected error: %v\n", err)
					}

					// Write the buffer to the stream we opened earlier
					if err := stream.Send(&pb.PieceStore{Hash: tt.hash, Size: tt.size, Ttl: tt.ttl, StoreOffset: tt.offset, Content: buffer[:n]}); err != nil {
						t.Errorf("Unexpected error: %v\n", err)
						continue
					}
				}

				resp, err := stream.CloseAndRecv()
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

				if resp.Message != tt.message || resp.TotalReceived != tt.totalReceived {
					t.Errorf("Expected: %v, %v\nGot: %v, %v\n", tt.message, tt.totalReceived, resp.Message, resp.TotalReceived)
				}

				// clean up DB entry
				_, err = db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE hash="%s"`, testHash))
				if err != nil {
					t.Errorf("Error cleaning test DB entry")
					return
				}
      }
  })
}

func TestDelete(t *testing.T) {
  t.Run("should return expected PieceDeleteSummary values", func(t *testing.T) {

    // set up test cases
    tests := []struct{
      hash string
			message string
      err string
    } {
        { // should successfully delete data
          hash: testHash,
					message: "OK",
          err: "",
        },
				{ // should err with invalid hash length
          hash: "123",
					message: "rpc error: code = Unknown desc = argError: Invalid hash length",
          err: "rpc error: code = Unknown desc = argError: Invalid hash length",
        },
				{ // should return OK with nonexistent file
          hash: "22222222222222222222",
					message: "OK",
          err: "",
        },
      }

      for _, tt := range tests {
				// simulate piece stored with farmer
		    _, err := pstore.Store(testHash, bytes.NewReader([]byte("butts")), 5, 0, s.PieceStoreDir)
		    if err != nil {
		      t.Errorf("Error: %v\nCould not create test piece", err)
					return
		    }

				// simulate piece TTL entry
				_, err = db.Exec(fmt.Sprintf(`INSERT INTO ttl (hash, created, expires) VALUES ("%s", "%d", "%d")`, testHash, testCreatedDate, testCreatedDate))
			  if err != nil {
			    t.Errorf("Error: %v\nCould not make TTL entry", err)
					return
			  }

		    defer pstore.Delete(testHash, s.PieceStoreDir)

        req := &pb.PieceDelete{Hash: tt.hash}
        resp, err := c.Delete(context.Background(), req)
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

        if resp.Message != tt.message {
          t.Errorf("Expected: %v\nGot: %v\n", tt.message, resp.Message)
          continue
        }

				// if test passes, check if file was indeed deleted
				filePath, err := pstore.PathByHash(tt.hash, s.PieceStoreDir)
				if _, err = os.Stat(filePath); os.IsNotExist(err) != true {
					t.Errorf("File not deleted")
					continue
				}

				// clean up DB entry
				_, err = db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE hash="%s"`, testHash))
				if err != nil {
					t.Errorf("Error cleaning test DB entry")
					return
				}
      }
  })
}

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
  db, err = sql.Open("sqlite3", s.DBPath)
  if err != nil {
    log.Fatal(err)
  }
  _, err = db.Exec("CREATE TABLE IF NOT EXISTS `ttl` (`hash` TEXT, `created` INT(10), `expires` INT(10));")
	if err != nil {
		log.Fatal(err)
	}

	// clean up temp files
  defer os.RemoveAll("./test-data")
  defer os.Remove("./test.db")
	defer db.Close()

	m.Run()
}
