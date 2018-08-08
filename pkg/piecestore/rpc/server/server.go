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
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	"storj.io/storj/pkg/utils"
	proto "storj.io/storj/protos/overlay"
	pb "storj.io/storj/protos/piecestore"
)

var (
	mon = monkit.Package()
)

// Server -- GRPC server meta data used in route calls
type Server struct {
	DataDir string
	DB      *psdb.PSDB
	config  Config
}

// Config stores values from a farmer node config file
type Config struct {
	NodeID        string
	PSAddress     string
	KadListenPort string
	KadAddress    string
	PieceStoreDir string
}

// Initialize -- initializes a server struct
func Initialize(ctx context.Context, config Config) (*Server, error) {
	dbPath := filepath.Join(config.PieceStoreDir, fmt.Sprintf("store-%s", config.NodeID), "piecestore.db")
	dataDir := filepath.Join(config.PieceStoreDir, fmt.Sprintf("store-%s", config.NodeID), "piece-store-data")

	psDB, err := psdb.OpenPSDB(ctx, dataDir, dbPath)
	if err != nil {
		return nil, err
	}

	fmt.Println(dbPath)

	return &Server{DataDir: dataDir, DB: psDB, config: config}, nil
}

// connectToKad joins the Kademlia network
func connectToKad(ctx context.Context, config Config) (*kademlia.Kademlia, error) {
	pshost, _, err := net.SplitHostPort(config.PSAddress)
	if err != nil {
		return nil, err
	}

	id := config.NodeID
	kadListenPort := config.KadListenPort
	kadAddress := config.KadAddress

	node := proto.Node{
		Id: id,
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   kadAddress,
		},
	}

	kad, err := kademlia.NewKademlia(kademlia.StringToNodeID(id), []proto.Node{node}, pshost, kadListenPort)
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

// Start the piecestore node
func (s *Server) Start(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = connectToKad(ctx, s.config)
	if err != nil {
		return err
	}

	// create a listener on TCP port
	lis, err := net.Listen("tcp", s.config.PSAddress)
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
		err := s.DB.CheckEntries(ctx)
		zap.S().Fatal("Error in CheckEntries: %v\n", err)
	}()

	log.Printf("Node %s started\n", s.config.NodeID)

	// start the server
	return grpcServer.Serve(lis)
}

// Stop the piececstore node
func (s *Server) Stop(ctx context.Context) (err error) {
	return s.DB.Close()
}

// Piece -- Send meta data about a stored by by Id
func (s *Server) Piece(ctx context.Context, in *pb.PieceId) (*pb.PieceSummary, error) {
	log.Println("Getting Meta data...")

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

	log.Println("Meta data retrieved.")
	return &pb.PieceSummary{Id: in.GetId(), Size: fileInfo.Size(), Expiration: ttl}, nil
}

// Delete -- Delete data by Id from piecestore
func (s *Server) Delete(ctx context.Context, in *pb.PieceDelete) (*pb.PieceDeleteSummary, error) {
	log.Println("Deleting data...")

	if err := s.deleteByID(in.GetId()); err != nil {
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

func (s *Server) verifySignature(ba *pb.RenterBandwidthAllocation) error {
	// TODO: verify signature

	// data := ba.GetData()
	// signature := ba.GetSignature()
	log.Printf("Verified signature\n")

	return nil
}
