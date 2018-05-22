// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	nodereputation "storj.io/storj/node_reputation"
	proto "storj.io/storj/protos/node_reputation"
)

func main() {

	db, err := nodereputation.SetServerDB("./Server.db")
	if err != nil {
		fmt.Println("err")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 7777))
	if err != nil {
		fmt.Println("err")
	}

	s := nodereputation.Server{}

	grpcServer := grpc.NewServer()

	proto.RegisterNodeReputationServer(grpcServer, &s)

	if err := grpcServer.Serve(lis); err != nil {
		fmt.Println("err")
	}

	defer grpcServer.GracefulStop()
	defer nodereputation.EndServerDB(db)

}
