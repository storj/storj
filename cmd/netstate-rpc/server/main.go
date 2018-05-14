// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/netstate"
	proto "storj.io/storj/protos/netstate"
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
	flag.BoolVar(&prod, "prod", false, "type of environment where this service runs")
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
		logger.Fatal("failed to listen", zap.Error(err))
	}

	grpcServer := grpc.NewServer()
	proto.RegisterNetStateServer(grpcServer, netstate.NewServer(bdb, logger))

	defer grpcServer.GracefulStop()
	err = grpcServer.Serve(lis)
	if err != nil {
		logger.Error("Failed to serve:", zap.Error(err))
	}
}
