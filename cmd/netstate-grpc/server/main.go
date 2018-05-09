// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc/grpclog"

	"storj.io/storj/pkg/netstate"
	"storj.io/storj/storage/boltdb"
)

var (
	port   int
	dbPath string
	prod   bool
)

func initializeFlags() {
	flag.IntVar(&port, "port", 8080, "port")
	flag.StringVar(&dbPath, "db", "netstate.db", "db path")
	flag.BoolVar(&prod, "prod", false, "environment this service is running in")
	flag.Parse()
}

func main() {
	initializeFlags()

	// No err here because no vars passed into NewDevelopment().
	// The default won't return an error, but if args are passed in,
	// then there will need to be error handling.
	logger, _ := zap.NewDevelopment()
	if prod {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()

	bdb, err := boltdb.New(logger, dbPath)
	if err != nil {
		return
	}
	defer bdb.Close()

	// start grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}

	ns := netstate.NewServer(logger, bdb)
	go ns.Serve(lis)
	defer ns.GracefulStop()
}
