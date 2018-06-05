// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package butts

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/cmd/piecestore-farmer/config"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	proto "storj.io/storj/protos/overlay"
	pb "storj.io/storj/protos/piecestore"
)

// Connect to the Kademlia network
func connectToKad(id, ip, port string) *kademlia.Kademlia {
	node := proto.Node{
		Id: string(id),
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   "130.211.168.182:4242",
		},
	}

	kad, err := kademlia.NewKademlia([]proto.Node{node}, ip, port)
	if err != nil {
		log.Fatalf("Failed to instantiate new Kademlia: %s", err.Error())
	}

	if err := kad.ListenAndServe(); err != nil {
		log.Fatalf("Failed to ListenAndServe on new Kademlia: %s", err.Error())
	}

	if err := kad.Bootstrap(context.Background()); err != nil {
		log.Fatalf("Failed to Bootstrap on new Kademlia: %s", err.Error())
	}

	return kad
}

func main() {
	// Get default folder for storing data and database
	dataFolder, err := os.Getwd()
	if err != nil {
		log.Fatalf(err.Error())
	}
	dir := flag.String("dir", dataFolder, "Folder where data is stored")

	manager, err := config.NewManager(*dir)
	if err != nil {
		log.Fatalf("Failed to create manager: %s\n", err.Error())
	}

	// READ FROM CONFIG
	config, err := manager.ReadConfig()
	if err != nil {
		log.Fatalf(err.Error())
	}

	ip := flag.String("ip", config.IP, "Server's public IP")
	port := flag.String("port", config.Port, "Port to run the server at")
	flag.Parse()

	_ = connectToKad(config.NodeID, *ip, *port)

	fileInfo, err := os.Stat(*dir)
	if err != nil {
		log.Fatalf(err.Error())
	}

	if fileInfo.IsDir() != true {
		log.Fatalf("dataFolder %s is not a directory", *dir)
	}

	// Suggestion for whoever implements this: Instead of using port use node id
	dataDir := path.Join(*dir, id, "/piece-store-data/")
	dbPath := path.Join(*dir, id, "/ttl-data.db")

	ttlDB, err := ttl.NewTTL(dbPath)
	if err != nil {
		log.Fatalf("failed to open DB")
	}

	// create a listener on TCP port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	// create a server instance
	s := server.Server{PieceStoreDir: dataDir, DB: ttlDB}

	// create a gRPC server object
	grpcServer := grpc.NewServer()

	// attach the api service to the server
	pb.RegisterPieceStoreRoutesServer(grpcServer, &s)

	// routinely check DB and delete expired entries
	go func() {
		err := s.DB.DBCleanup(dataDir)
		log.Fatalf("Error in DBCleanup: %v", err)
	}()

	// start the server
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
