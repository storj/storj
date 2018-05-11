// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"google.golang.org/grpc"
	//"storj.io/storj/node_reputation"
)

func TestNodeReputationClient(t *testing.T) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 7777))
	if err != nil {
		fmt.Println("err")
	}

	grpcServer := grpc.NewServer()
	server := Server{}
	RegisterNodeReputationServer(grpcServer, &server)

	defer grpcServer.GracefulStop()
	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	if err != nil {
		fmt.Println("conn err")
	}

	client := NewNodeReputationClient(conn)

	response, err := client.UpdateReputation(context.Background(),
		&NodeUpdate{
			Source:      "Bob",
			NodeName:    "Alice",
			ColumnName:  "Uptime",
			ColumnValue: "2",
		},
	)

	if err != nil {
		fmt.Println("Update response err")
	}

	if response.Status != 0 {
		t.Error("expected UPDATE_SUCCESS, got: ", response.Status)
	}

	os.Remove("./Server.db")
}
