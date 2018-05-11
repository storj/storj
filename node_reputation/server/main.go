package main

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"storj.io/storj/node_reputation"
)

func main() {

	_, err := nodereputation.SetServerDB("./Server.db")
	if err != nil {
		fmt.Println("err")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 7777))
	if err != nil {
		fmt.Println("err")
	}

	s := nodereputation.Server{}

	grpcServer := grpc.NewServer()

	nodereputation.RegisterNodeReputationServer(grpcServer, &s)

	if err := grpcServer.Serve(lis); err != nil {
		fmt.Println("err")
	}

}
