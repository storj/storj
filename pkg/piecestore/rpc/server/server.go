// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"io"
	"log"
	"os"

	"golang.org/x/net/context"

	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	pb "storj.io/storj/protos/piecestore"
)

// OK - Success!
const OK = "OK"

// Server -- GRPC server meta data used in route calls
type Server struct {
	PieceStoreDir string
	DB            *ttl.TTL
}

func cleanup(s *Server, id string) error {
	if err := pstore.Delete(id, s.PieceStoreDir); err != nil {
		return err
	}

	if err := s.DB.DeleteTTLByID(id); err != nil {
		return err
	}

	return nil
}

// Store -- Store incoming data using piecestore
func (s *Server) Store(stream pb.PieceStoreRoutes_StoreServer) error {
	// Receive initial meta data about what's being stored
	piece, err := stream.Recv()
	if err != nil {
		return err
	}

	log.Printf("Storing %s...", piece.Id)

	// If we put in the database first then that checks if the data already exists
	if err = s.DB.AddTTLToDB(piece.Id, piece.Ttl); err != nil {
		log.Println(err)
		return err
	}

	// Initialize file for storing data
	storeFile, err := pstore.StoreWriter(piece.Id, s.PieceStoreDir)
	if err != nil {
		if err = cleanup(s, piece.Id); err != nil {
			log.Printf("Failed on cleanup in Store: %s", err.Error())
		}

		return err
	}

	defer func() {
		if err := storeFile.Close(); err != nil {
			log.Printf("failed to close store file: %s\n", err)
		}
	}()

	reader := NewStreamReader(stream)
	total, err := io.Copy(storeFile, reader)
	if err != nil {
		if err = cleanup(s, piece.Id); err != nil {
			log.Printf("Failed on cleanup in Store: %s", err.Error())
		}

		return err
	}

	log.Printf("Successfully stored %s.", piece.Id)

	return stream.SendAndClose(&pb.PieceStoreSummary{Message: OK, TotalReceived: int64(total)})
}

// Retrieve -- Retrieve data from piecestore and send to client
func (s *Server) Retrieve(pieceMeta *pb.PieceRetrieval, stream pb.PieceStoreRoutes_RetrieveServer) error {
	log.Printf("Retrieving %s...", pieceMeta.Id)

	path, err := pstore.PathByID(pieceMeta.Id, s.PieceStoreDir)
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

	storeFile, err := pstore.RetrieveReader(stream.Context(), pieceMeta.Id, pieceMeta.Offset, totalToRead, s.PieceStoreDir)
	if err != nil {
		return err
	}

	defer func() {
		if err := storeFile.Close(); err != nil {
			log.Printf("failed to close store file: %s\n", err)
		}
	}()

	writer := &StreamWriter{stream: stream}
	_, err = io.Copy(writer, storeFile)
	if err != nil {
		return err
	}

	log.Printf("Successfully retrieved %s.", pieceMeta.Id)
	return nil
}

// Piece -- Send meta data about a stored by by Id
func (s *Server) Piece(ctx context.Context, in *pb.PieceId) (*pb.PieceSummary, error) {
	log.Printf("Getting Meta for %s...", in.Id)

	path, err := pstore.PathByID(in.Id, s.PieceStoreDir)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Read database to calculate expiration
	ttl, err := s.DB.GetTTLByID(in.Id)
	if err != nil {
		return nil, err
	}

	log.Printf("Successfully retrieved meta for %s.", in.Id)
	return &pb.PieceSummary{Id: in.Id, Size: fileInfo.Size(), Expiration: ttl}, nil
}

// Delete -- Delete data by Id from piecestore
func (s *Server) Delete(ctx context.Context, in *pb.PieceDelete) (*pb.PieceDeleteSummary, error) {
	log.Printf("Deleting %s...", in.Id)

	if err := cleanup(s, in.Id); err != nil {
		return nil, err
	}

	log.Printf("Successfully deleted %s.", in.Id)
	return &pb.PieceDeleteSummary{Message: OK}, nil
}
