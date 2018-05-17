// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"regexp"

	"google.golang.org/grpc"

	_ "github.com/mattn/go-sqlite3"

	pb "storj.io/storj/pkg/rpcClientServer/protobuf"
	"storj.io/storj/pkg/rpcClientServer/server/api"
	"storj.io/storj/pkg/rpcClientServer/server/utils"
)

func main() {
	port := "7777"

	if len(os.Args) > 1 {
		if matched, _ := regexp.MatchString(`^\d{2,6}$`, os.Args[1]); matched == true {
			port = os.Args[1]
		}
	}

	dataDir := path.Join("./piece-store-data/", port)
	dbPath := "ttl-data.db"

	// open ttl database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS `ttl` (`hash` TEXT, `created` INT(10), `expires` INT(10));")
	if err != nil {
		log.Fatal(err)
	}

	// create a listener on TCP port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// create a server instance
	s := api.Server{dataDir, dbPath}

	// create a gRPC server object
	grpcServer := grpc.NewServer()

	// attach the api service to the server
	pb.RegisterPieceStoreRoutesServer(grpcServer, &s)

	// routinely check DB for and delete expired entries
	go func() {
		utils.DbChecker(db, dataDir)
	}()

	// start the server
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
