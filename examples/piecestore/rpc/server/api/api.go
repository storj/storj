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

func (s *Server) Store(stream pb.PieceStoreRoutes_StoreServer) error {
  fmt.Println("Storing data...")
	startTime := time.Now()
	var total int64 = 0
	var storeMeta *StoreData
	for {
		pieceData, err := stream.Recv()
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
			return stream.SendAndClose(&pb.PieceStoreSummary{
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
			storeMeta = &StoreData{Ttl: pieceData.Ttl, Hash: pieceData.Hash, Size: pieceData.Size}
		}

		length := int64(len(pieceData.Content))

		// Write chunk received to disk
		err = pstore.Store(pieceData.Hash, bytes.NewReader(pieceData.Content), length, total + pieceData.StoreOffset, s.PieceStoreDir)

		if err != nil {
			fmt.Println("Store data Error: ", err.Error())
			endTime := time.Now()
			return stream.SendAndClose(&pb.PieceStoreSummary{
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

func (s *Server) Retrieve(pieceMeta *pb.PieceRetrieval, stream pb.PieceStoreRoutes_RetrieveServer) error {
  fmt.Println("Retrieving data...")

	path, err := pstore.PathByHash(pieceMeta.Hash, s.PieceStoreDir)

	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	var total int64 = 0
	for total < fileInfo.Size() {

		b := []byte{}
		writeBuff := bytes.NewBuffer(b)

		n, err := pstore.Retrieve(pieceMeta.Hash, writeBuff, 4096, pieceMeta.StoreOffset + total, s.PieceStoreDir)
		if err != nil {
			return err
		}

		// Write the buffer to the stream we opened earlier
		if err := stream.Send(&pb.PieceRetrievalStream{Size: n, Content: writeBuff.Bytes()}); err != nil {
			fmt.Println("%v.Send() = %v", stream, err)
			return err
		}

		total += n
	}

	return nil
}

func (s *Server) Piece(ctx context.Context, in *pb.PieceHash) (*pb.PieceSummary, error) {
	fmt.Println("Getting Meta data...")

	path, err := pstore.PathByHash(in.Hash, s.PieceStoreDir)

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// TODO: Read database to calculate expiration

	return &pb.PieceSummary{Hash: in.Hash, Size: fileInfo.Size(), Expiration: 0}, nil
}

func (s *Server) Delete(ctx context.Context, in *pb.PieceDelete) (*pb.PieceDeleteSummary, error) {
	fmt.Println("Deleting data")
	startTime := time.Now()
	err := pstore.Delete(in.Hash, s.PieceStoreDir)
	if err != nil {
		endTime := time.Now()
		return &pb.PieceDeleteSummary{
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
		return &pb.PieceDeleteSummary{
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
		return &pb.PieceDeleteSummary{
			Status:   -1,
		  Message: fmt.Sprintf("Rows affected: (%d) does not equal 1", rowsDeleted),
		  ElapsedTime: int64(time.Now().Sub(startTime).Seconds()),
		}, nil
		}
	endTime := time.Now()
  return &pb.PieceDeleteSummary{
		Status:  0,
		Message: "OK",
		ElapsedTime: int64(endTime.Sub(startTime).Seconds()),
	}, nil
}
