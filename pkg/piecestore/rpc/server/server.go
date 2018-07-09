// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"log"
	"os"

	"golang.org/x/net/context"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	pb "storj.io/storj/protos/piecestore"
)

// Server -- GRPC server meta data used in route calls
type Server struct {
	PieceStoreDir string
	DB            *ttl.TTL
	id            kademlia.NodeID
}

// Piece -- Send meta data about a stored by by Id
func (s *Server) Piece(ctx context.Context, in *pb.PieceId) (*pb.PieceSummary, error) {
	log.Println("Getting Meta data...")

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

	log.Println("Meta data retrieved.")
	return &pb.PieceSummary{Id: in.Id, Size: fileInfo.Size(), Expiration: ttl}, nil
}

// Delete -- Delete data by Id from piecestore
func (s *Server) Delete(ctx context.Context, in *pb.PieceDelete) (*pb.PieceDeleteSummary, error) {
	log.Println("Deleting data...")

	if err := s.deleteByID(in.Id); err != nil {
		return nil, err
	}

	log.Println("Successfully deleted data.")
	return &pb.PieceDeleteSummary{Message: OK}, nil
}

func (s *Server) deleteByID(id string) error {
	if err := pstore.Delete(id, s.PieceStoreDir); err != nil {
		return err
	}

	if err := s.DB.DeleteTTLByID(id); err != nil {
		return err
	}

	log.Printf("Deleted data of id (%s) from piecestore\n", id)

	return nil
}

func (s *Server) verifySignature(signature []byte) error {
	//verify signature
	log.Printf("Verified signature: %s\n", signature)

	return nil
}
