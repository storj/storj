// Package main implements a simple gRPC server that demonstrates how to use gRPC-Go libraries
// to perform unary, client streaming, server streaming and full duplex RPCs.
//
// It implements the route guide service whose definition can be found in routeguide/route_guide.proto.
package api

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"golang.org/x/net/context"

	pb "storj.io/storj/examples/piecestore/rpc/protobuf"

	"storj.io/storj/pkg/piecestore"
)

type Server struct {
  PieceStoreDir string
	DbPath string
}

type StoreData struct {
	Ttl int64
	Hash string
	Size int64
}

func (s *Server) Store(stream pb.RouteGuide_StoreServer) error {
  fmt.Println("Storing data...")
	startTime := time.Now()
	var total int64 = 0
	var storeMeta *StoreData
	for {
		shardData, err := stream.Recv()
		if err == io.EOF {
			fmt.Println("Successfully stored data...")
			endTime := time.Now()

			db, err := sql.Open("sqlite3", s.DbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			_, err = db.Exec(fmt.Sprintf(`INSERT INTO ttl (hash, created, expires) VALUES ("%s", "%d", "%d")`, storeMeta.Hash, time.Now().Unix(), storeMeta.Ttl))
			if err != nil {
				return err
			}
			return stream.SendAndClose(&pb.ShardStoreSummary{
				Status:   0,
				Message: "OK",
				TotalReceived: total,
				ElapsedTime:  int64(endTime.Sub(startTime).Seconds()),
			})
		}
		if err != nil {
			return err
		}

		if storeMeta == nil {
			storeMeta = &StoreData{Ttl: shardData.Ttl, Hash: shardData.Hash, Size: shardData.Size}
		}

		length := int64(len(shardData.Content))

		// Write chunk received to disk
		err = pstore.Store(shardData.Hash, bytes.NewReader(shardData.Content), length, total + shardData.StoreOffset, s.PieceStoreDir)

		if err != nil {
			fmt.Println("Store data Error: ", err.Error())
			endTime := time.Now()
			return stream.SendAndClose(&pb.ShardStoreSummary{
				Status:   -1,
				Message: err.Error(),
				TotalReceived: total,
				ElapsedTime:  int64(endTime.Sub(startTime).Seconds()),
			})
		}

		total += length
	}
  return nil
}

func (s *Server) Retrieve(shardMeta *pb.ShardRetrieval, stream pb.RouteGuide_RetrieveServer) error {
  fmt.Println("Retrieving data...")

	path, err := pstore.PathByHash(shardMeta.Hash, s.PieceStoreDir)

	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	var total int64 = 0
	for total < fileInfo.Size() {

		b := []byte{}
		writeBuff := bytes.NewBuffer(b)

		n, err := pstore.Retrieve(shardMeta.Hash, writeBuff, 4096, shardMeta.StoreOffset + total, s.PieceStoreDir)
		if err != nil {
			return err
		}

		// Write the buffer to the stream we opened earlier
		if err := stream.Send(&pb.ShardRetrievalStream{Size: n, Content: writeBuff.Bytes()}); err != nil {
			fmt.Println("%v.Send() = %v", stream, err)
			return err
		}

		total += n
	}

	return nil
}

func (s *Server) Shard(ctx context.Context, in *pb.ShardHash) (*pb.ShardSummary, error) {
	fmt.Println("Getting Meta data...")

	path, err := pstore.PathByHash(in.Hash, s.PieceStoreDir)

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// TODO: Read database to calculate expiration
	db, err := sql.Open("sqlite3", s.DbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(fmt.Sprintf(`SELECT expires FROM ttl WHERE hash="%s"`, in.Hash))
	if err != nil {
		return nil, err
	}

	var ttl int64

	for rows.Next() {
		 err = rows.Scan(&ttl)
		 if err != nil {
			 return nil, err
		 }
	}

	return &pb.ShardSummary{Hash: in.Hash, Size: fileInfo.Size(), Expiration: ttl}, nil
}

func (s *Server) Delete(ctx context.Context, in *pb.ShardDelete) (*pb.ShardDeleteSummary, error) {
	fmt.Println("Deleting data")
	startTime := time.Now()
	err := pstore.Delete(in.Hash, s.PieceStoreDir)
	if err != nil {
		endTime := time.Now()
		return &pb.ShardDeleteSummary{
			Status:   -1,
		  Message: err.Error(),
		  ElapsedTime: int64(endTime.Sub(startTime).Seconds()),
		}, err
	}
	db, err := sql.Open("sqlite3", s.DbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result, err := db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE hash="%s"`, in.Hash))
	if err != nil {
		return &pb.ShardDeleteSummary{
			Status:   -1,
		  Message: err.Error(),
		  ElapsedTime: int64(time.Now().Sub(startTime).Seconds()),
		}, err
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return nil, err
		}
	if rowsDeleted == 0 || rowsDeleted > 1 {
		return &pb.ShardDeleteSummary{
			Status:   -1,
		  Message: fmt.Sprintf("Rows affected: (%d) does not equal 1", rowsDeleted),
		  ElapsedTime: int64(time.Now().Sub(startTime).Seconds()),
		}, nil
		}
	endTime := time.Now()
  return &pb.ShardDeleteSummary{
		Status:  0,
		Message: "OK",
		ElapsedTime: int64(endTime.Sub(startTime).Seconds()),
	}, nil
}
