package main

import (
	"fmt"
	"net"

	"storj.io/storj/reputation"

	"google.golang.org/grpc"
)

func main() {

	_, err := reputation.SetServerDB("./Server.db")
	if err != nil {
		fmt.Println("err")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 7777))
	if err != nil {
		fmt.Println("err")
	}

	s := reputation.Server{}

	grpcServer := grpc.NewServer()

	reputation.RegisterBridgeServer(grpcServer, &s)

	if err := grpcServer.Serve(lis); err != nil {
		fmt.Println("err")
	}

}
