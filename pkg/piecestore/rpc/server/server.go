// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"log"
	"os"
	"path/filepath"

	"golang.org/x/net/context"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	pb "storj.io/storj/protos/piecestore"
)

var (
	mon = monkit.Package()
)

// Server -- GRPC server meta data used in route calls
type Server struct {
	DataDir string
	DB      *psdb.PSDB
}

// Initialize -- initializes a server struct
func Initialize(ctx context.Context, path string) (*Server, error) {
	dbPath := filepath.Join(path, "piecestore.db")
	dataDir := filepath.Join(path, "piece-store-data")

	psDB, err := psdb.OpenPSDB(ctx, dataDir, dbPath)
	if err != nil {
		return nil, err
	}

	return &Server{DataDir: dataDir, DB: psDB}, nil
}

// Stop the piececstore node
func (s *Server) Stop(ctx context.Context) (err error) {
	return s.DB.Close()
}

// Piece -- Send meta data about a stored by by Id
func (s *Server) Piece(ctx context.Context, in *pb.PieceId) (*pb.PieceSummary, error) {
	log.Printf("Getting Meta for %s...", in.Id)

	path, err := pstore.PathByID(in.GetId(), s.DataDir)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Read database to calculate expiration
	ttl, err := s.DB.GetTTLByID(in.GetId())
	if err != nil {
		return nil, err
	}

	log.Printf("Successfully retrieved meta for %s.", in.Id)
	return &pb.PieceSummary{Id: in.GetId(), Size: fileInfo.Size(), Expiration: ttl}, nil
}

// Delete -- Delete data by Id from piecestore
func (s *Server) Delete(ctx context.Context, in *pb.PieceDelete) (*pb.PieceDeleteSummary, error) {
	log.Printf("Deleting %s...", in.Id)

	if err := s.deleteByID(in.GetId()); err != nil {
		return nil, err
	}

	log.Printf("Successfully deleted %s.", in.Id)
	return &pb.PieceDeleteSummary{Message: OK}, nil
}

func (s *Server) deleteByID(id string) error {
	if err := pstore.Delete(id, s.DataDir); err != nil {
		return err
	}

	if err := s.DB.DeleteTTLByID(id); err != nil {
		return err
	}

	log.Printf("Deleted data of id (%s) from piecestore\n", id)

	return nil
}

func (s *Server) verifySignature(ba *pb.RenterBandwidthAllocation) error {
	// TODO: verify signature

	// data := ba.GetData()
	// signature := ba.GetSignature()
	log.Printf("Verified signature\n")

	return nil
}
