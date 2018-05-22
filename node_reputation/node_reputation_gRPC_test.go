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
	proto "storj.io/storj/protos/nodereputation"
)

func TestNodeReputationClient(t *testing.T) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8081))
	if err != nil {
		fmt.Println("net listen err")
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
		&proto.NodeUpdate{
			Source:      "Bob",
			NodeName:    "Alice",
			ColumnName:  ColumnName_uptime,
			ColumnValue: "2",
		},
	)

	if err != nil {
		fmt.Println("Update response err")
	}

	if response.Status != 0 {
		t.Error("expected UPDATE_SUCCESS, got: ", response.Status)
	}

	queryResponse, err := client.NodeReputation(context.Background(),
		&NodeQuery{
			Source:   "Test",
			NodeName: "Alice",
		},
	)
	if err != nil {
		fmt.Println("Query response err")
	}

	if queryResponse.Uptime != 2 {
		t.Error("expected uptime of 2, got:", queryResponse.Uptime)
	}

	response1, err := client.UpdateReputation(context.Background(),
		&proto.NodeUpdate{
			Source:      "Test",
			NodeName:    "Alice",
			ColumnName:  ColumnName_uptime,
			ColumnValue: "100",
		},
	)
	if err != nil {
		fmt.Println("Update response1 err")
	}

	if response1.Status != 0 {
		t.Error("expected UPDATE_SUCCESS, got: ", response1.Status)
	}

	queryResponse1, err := client.NodeReputation(context.Background(),
		&proto.NodeQuery{
			Source:   "Test",
			NodeName: "Alice",
		},
	)
	if err != nil {
		fmt.Println("Query response 1 err")
	}

	if queryResponse1.Uptime != 100 {
		t.Error("expected uptime of 100, got:", queryResponse1.Uptime)
	}

	filterResponse, err := client.FilterNodeReputation(context.Background(),
		&NodeFilter{
			Source:      "Test",
			ColumnName:  ColumnName_uptime,
			Operand:     NodeFilter_LESS_THAN,
			ColumnValue: "100",
		},
	)
	if err != nil {
		fmt.Println("Query response 1 err")
	}

	if filterResponse.Records[0].Uptime >= 100 {
		t.Error("expected uptime less than 100, got:", filterResponse.Records)
	}

	prune, err := client.PruneNodeReputation(context.Background(),
		&proto.NodeQuery{
			Source:   "Bob",
			NodeName: "Alice",
		},
	)
	if err != nil {
		fmt.Println("Prune err")
	}

	if prune.Status != UpdateReply_UPDATE_SUCCESS {
		t.Error("expected prune success got", prune.Status)
	}

	queryResponse1, err = client.NodeReputation(context.Background(),
		&proto.NodeQuery{
			Source:   "Test",
			NodeName: "Alice",
		},
	)
	if err != nil {
		fmt.Println("Query response err")
	}

	if queryResponse1.Uptime != 100 {
		t.Error("expected uptime of 100, got:", queryResponse1)
	}

	os.Remove("./Server.db")
}
