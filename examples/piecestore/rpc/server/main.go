// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"regexp"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	pb "storj.io/storj/protos/piecestore"
)

func main() {
	port := "7777"

	if len(os.Args) > 1 && os.Args[1] != "" {
		if matched, _ := regexp.MatchString(`^\d{2,6}$`, os.Args[1]); matched == true {
			port = os.Args[1]
		}
	}

	// Get default folder for storing data and database
	dataFolder, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Check if directory for storing data and database was passed in
	if len(os.Args) > 2 && os.Args[2] != "" {
		dataFolder = os.Args[2]
	}

	fileInfo, err := os.Stat(dataFolder)
	if err != nil {
		log.Fatalf(err.Error())
	}

	if fileInfo.IsDir() != true {
		log.Fatalf("dataFolder %s is not a directory", dataFolder)
	}

	// Suggestion for whoever implements this: Instead of using port use node id
	dataDir := path.Join(dataFolder, port, "/piece-store-data/")
	dbPath := path.Join(dataFolder, port, "/ttl-data.db")

	ttlDB, err := ttl.NewTTL(dbPath)
	if err != nil {
		log.Fatalf("failed to open DB")
	}

	// create a listener on TCP port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
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
