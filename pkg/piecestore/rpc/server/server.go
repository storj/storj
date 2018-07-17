// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	"storj.io/storj/pkg/utils"
	proto "storj.io/storj/protos/overlay"
	pb "storj.io/storj/protos/piecestore"
)

// Server -- GRPC server meta data used in route calls
type Server struct {
	DataDir string
	DB      *ttl.TTL
	config  Config
}

// Config stores values from a farmer node config file
type Config struct {
	NodeID        string
	PsHost        string
	PsPort        string
	KadListenPort string
	KadPort       string
	KadHost       string
	PieceStoreDir string
}

// New -- creates a new server struct
func New(config Config) (*Server, error) {
	dbPath := filepath.Join(config.PieceStoreDir, fmt.Sprintf("store-%s", config.NodeID), "ttl-data.db")
	dataDir := filepath.Join(config.PieceStoreDir, fmt.Sprintf("store-%s", config.NodeID), "piece-store-data")

	ttlDB, err := ttl.NewTTL(dbPath)
	if err != nil {
		return nil, err
	}

	return &Server{DataDir: dataDir, DB: ttlDB, config: config}, nil
}

// connectToKad joins the Kademlia network
func connectToKad(ctx context.Context, id, ip, kadListenPort, kadAddress string) (*kademlia.Kademlia, error) {
	node := proto.Node{
		Id: id,
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   kadAddress,
		},
	}

	kad, err := kademlia.NewKademlia(kademlia.StringToNodeID(id), []proto.Node{node}, ip, kadListenPort)
	if err != nil {
		return nil, errs.New("Failed to instantiate new Kademlia: %s", err.Error())
	}

	if err := kad.ListenAndServe(); err != nil {
		return nil, errs.New("Failed to ListenAndServe on new Kademlia: %s", err.Error())
	}

	if err := kad.Bootstrap(ctx); err != nil {
		return nil, errs.New("Failed to Bootstrap on new Kademlia: %s", err.Error())
	}

	return kad, nil
}

// Start the piececstore node
func (s *Server) Start() error {
	ctx := context.Background()

	_, err := connectToKad(ctx, s.config.NodeID, s.config.PsHost, s.config.KadListenPort, fmt.Sprintf("%s:%s", s.config.KadHost, s.config.KadPort))
	if err != nil {
		return err
	}

	// create a listener on TCP port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.config.PsPort))
	if err != nil {
		return err
	}

	defer utils.Close(lis)

	// create a gRPC server object
	grpcServer := grpc.NewServer()

	// attach the api service to the server
	pb.RegisterPieceStoreRoutesServer(grpcServer, s)

	// routinely check DB and delete expired entries
	go func() {
		err := s.DB.DBCleanup(s.DataDir)
		zap.S().Errorf("Error in DBCleanup: %v\n", err)
	}()

	fmt.Printf("Node %s started\n", s.config.NodeID)

	// start the server
	if err := grpcServer.Serve(lis); err != nil {
		zap.S().Errorf("failed to serve: %s\n", err)
	}

	return err
}

// Piece -- Send meta data about a stored by by Id
func (s *Server) Piece(ctx context.Context, in *pb.PieceId) (*pb.PieceSummary, error) {
	log.Println("Getting Meta data...")

	path, err := pstore.PathByID(in.Id, s.DataDir)
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
	if err := pstore.Delete(id, s.DataDir); err != nil {
		return err
	}

	if err := s.DB.DeleteTTLByID(id); err != nil {
		return err
	}

	log.Printf("Deleted data of id (%s) from piecestore\n", id)

	return nil
}

func (s *Server) verifySignature(signature []byte) error {
	// TODO: verify signature
	log.Printf("Verified signature: %s\n", signature)

	return nil
}

func (s *Server) writeBandwidthAllocToDB(ba *pb.BandwidthAllocation) error {
	log.Printf("Payer: %s, Renter: %s, Size: %v\n", ba.Data.Payer, ba.Data.Renter, ba.Data.Size)

	// TODO: Write ba to database

	return nil
}
