// Package main implements a simple gRPC server that demonstrates how to use gRPC-Go libraries
// to perform unary, client streaming, server streaming and full duplex RPCs.
//
// It implements the route guide service whose definition can be found in routeguide/route_guide.proto.
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

  "github.com/aleitner/piece-store/rpc-server/api"
	"github.com/aleitner/piece-store/rpc-server/utils"
  pb "github.com/aleitner/piece-store/routeguide"
	_ "github.com/mattn/go-sqlite3"

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
	pb.RegisterRouteGuideServer(grpcServer, &s)

	// routinely check DB for and delete expired entries
	go func() {
		utils.DbChecker(db, dataDir)
	}()

  // start the server
  if err := grpcServer.Serve(lis); err != nil {
    log.Fatalf("failed to serve: %s", err)
  }
}
