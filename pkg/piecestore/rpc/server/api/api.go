// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package api

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/net/context"

	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	pb "storj.io/storj/protos/piecestore"
)

// Server -- GRPC server meta data used in route calls
type Server struct {
	PieceStoreDir string
	DB            *ttl.TTL
}

// Store -- Store incoming data using piecestore
func (s *Server) Store(stream pb.PieceStoreRoutes_StoreServer) error {
	log.Println("Storing data...")

	total := int64(0)

	// Receive initial meta data about what's being stored
	piece, err := stream.Recv()
	if err == io.EOF {
		fmt.Println("HERE")
		return err
	} else if err != nil {
		return err
	}

	// Initialize file for storing data
	storeFile, err := pstore.StoreWriter(piece.Hash, piece.Size, piece.StoreOffset, s.PieceStoreDir)
	if err != nil {
		return err
	}
	defer storeFile.Close()

	for {
		storeMsg, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// Write chunk received to disk
		n, err := storeFile.Write(storeMsg.Content)
		if err != nil {
			return err
		}

		total += int64(n)
	}

	if total < piece.Size {
		return fmt.Errorf("Recieved %v bytes of total %v bytes", total, piece.Size)
	}

	log.Println("Successfully stored data.")

	err := s.DB.AddTTLToDB(storeMeta.Hash, storeMeta.TTL)
	if err != nil {
		return err
	}

	return stream.SendAndClose(&pb.PieceStoreSummary{Message: "OK", TotalReceived: total})
}

// Retrieve -- Retrieve data from piecestore and send to client
func (s *Server) Retrieve(pieceMeta *pb.PieceRetrieval, stream pb.PieceStoreRoutes_RetrieveServer) error {
	log.Println("Retrieving data...")

	path, err := pstore.PathByHash(pieceMeta.Hash, s.PieceStoreDir)
	if err != nil {
		return err
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Read the size specified
	totalToRead := pieceMeta.Size
	// Read the entire file if specified -1
	if pieceMeta.Size <= -1 {
		totalToRead = fileInfo.Size()
	}

	storeFile, err := pstore.RetrieveReader(pieceMeta.Hash, totalToRead, pieceMeta.StoreOffset, s.PieceStoreDir)
	if err != nil {
		return err
	}

	defer storeFile.Close()

	totalRead := int64(0)
	for totalRead < totalToRead {

		b := []byte{}
		writeBuff := bytes.NewBuffer(b)

		// Read the 4kb at a time until it has to read less
		sizeToRead := int64(4096)
		if pieceMeta.Size-totalRead < sizeToRead {
			sizeToRead = pieceMeta.Size - totalRead
		}

		n, err := io.Copy(writeBuff, storeFile)
		if err != nil {
			return err
		}

		// Write the buffer to the stream we opened earlier
		if err := stream.Send(&pb.PieceRetrievalStream{Size: n, Content: writeBuff.Bytes()}); err != nil {
			return err
		}

		totalRead += n
	}

	log.Println("Successfully retrieved data.")
	return nil
}

// Piece -- Send meta data about a stored by by Hash
func (s *Server) Piece(ctx context.Context, in *pb.PieceHash) (*pb.PieceSummary, error) {
	log.Println("Getting Meta data...")

	path, err := pstore.PathByHash(in.Hash, s.PieceStoreDir)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Read database to calculate expiration
	ttl, err := s.DB.GetTTLByHash(in.Hash)
	if err != nil {
		return nil, err
	}

	log.Println("Meta data retrieved.")
	return &pb.PieceSummary{Hash: in.Hash, Size: fileInfo.Size(), Expiration: ttl}, nil
}

// Delete -- Delete data by Hash from piecestore
func (s *Server) Delete(ctx context.Context, in *pb.PieceDelete) (*pb.PieceDeleteSummary, error) {
	log.Println("Deleting data...")

	if err := pstore.Delete(in.Hash, s.PieceStoreDir); err != nil {
		return nil, err
	}

	if err := s.DB.DeleteTTLByHash(in.Hash); err != nil {
		return nil, err
	}

	log.Println("Successfully deleted data.")
	return &pb.PieceDeleteSummary{Message: "OK"}, nil
}
