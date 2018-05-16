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
  "storj.io/storj/examples/piecestore/rpc/server/api"
  "storj.io/storj/pkg/piecestore"

  _ "github.com/mattn/go-sqlite3"
  pb "storj.io/storj/examples/piecestore/rpc/protobuf"
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
    } {
        {
          hash: testHash,
          size: 5,
          expiration: testExpires,
        },
    }

    for _, tt := range tests {
      req := &pb.PieceHash{Hash: tt.hash}
      resp, err := c.Piece(context.Background(), req)
      if err != nil {
          t.Errorf("PieceTest(%v) got unexpected error", err)
          return
      }
      if resp.Hash != tt.hash || resp.Size != tt.size || resp.Expiration != tt.expiration {
          t.Errorf("Expected: %v, %v, %v\nGot: %v, %v, %v\n", tt.hash, tt.size, tt.expiration, resp.Hash, resp.Size, resp.Expiration)
          return
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

    // Set up a connection to the Server.
    const address = "localhost:3000"
    conn, err := grpc.Dial(address, grpc.WithInsecure())
    if err != nil {
        t.Fatalf("did not connect: %v", err)
    }
    defer conn.Close()
    c := pb.NewPieceStoreRoutesClient(conn)

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
            return
          }
          resp, err := stream.Recv()
          if len(tt.err) > 0 {
            if err != nil {
              if err.Error() == tt.err {
                return
              }
            }
            t.Errorf("\nExpected: %s\nGot: %v\n", tt.err, err)
            return
          }

          if resp.Size != tt.size || bytes.Equal(resp.Content, tt.content) != true {
            t.Errorf("Expected: %v, %v\nGot: %v, %v\n", tt.size, tt.content, resp.Size, resp.Content)
            return
          }
      }
  })
}

// func TestStore(t *testing.T) {
//   t.Run("should return expected PieceStoreSummary values", func(t *testing.T) {
//
//   }
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
