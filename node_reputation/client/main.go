// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"storj.io/storj/node_reputation"
)

func main() {

	var conn *grpc.ClientConn

	conn, err := grpc.Dial(":7777", grpc.WithInsecure())
	if err != nil {
		fmt.Println("conn err")
	}
	defer conn.Close()

	client := nodereputation.NewNodeReputationClient(conn)

	response, err := client.UpdateReputation(context.Background(),
		&nodereputation.NodeUpdate{
			Source:      "Bob",
			NodeName:    "Alice",
			ColumnName:  nodereputation.ColumnName_uptime,
			ColumnValue: "30",
		},
	)

	if err != nil {
		fmt.Println("Update response err")
	}
	fmt.Println("Reply receive", response.Status)

	rep, err := client.NodeReputation(context.Background(),
		&nodereputation.NodeQuery{
			Source:   "Bob",
			NodeName: "Alice",
		},
	)
	if err != nil {
		fmt.Println("Rep respnse err")
	}
	fmt.Println("Rep receive", rep)

	response1, err := client.UpdateReputation(context.Background(),
		&nodereputation.NodeUpdate{
			Source:      "Bob",
			NodeName:    "Alice",
			ColumnName:  nodereputation.ColumnName_uptime,
			ColumnValue: "3",
		},
	)

	if err != nil {
		fmt.Println("Update response1 err")
	}
	fmt.Println("Reply receive", response1.Status)

	filter, err := client.FilterNodeReputation(context.Background(),
		&nodereputation.NodeFilter{
			Source:      "Bob",
			ColumnName:  nodereputation.ColumnName_uptime,
			Operand:     nodereputation.NodeFilter_LESS_THAN,
			ColumnValue: "10",
		},
	)
	if err != nil {
		fmt.Println("Filter response err")
	}
	fmt.Println("Filter reply uptime less than 10", filter.Records)

	response2, err := client.UpdateReputation(context.Background(),
		&nodereputation.NodeUpdate{
			Source:      "Bob",
			NodeName:    "Alice",
			ColumnName:  nodereputation.ColumnName_uptime,
			ColumnValue: "42",
		},
	)

	if err != nil {
		fmt.Println("Update response2 err")
	}
	fmt.Println("Reply receive", response2.Status)

	prune, err := client.PruneNodeReputation(context.Background(),
		&nodereputation.NodeQuery{
			Source:   "Bob",
			NodeName: "Alice",
		},
	)
	if err != nil {
		fmt.Println("Prune err")
	}

	fmt.Println("Prune status", prune.Status)

	rep2, err := client.NodeReputation(context.Background(),
		&nodereputation.NodeQuery{
			Source:   "Bob",
			NodeName: "Alice",
		},
	)
	if err != nil {
		fmt.Println("Rep2 respnse err")
	}
	fmt.Println("Rep2 receive", rep2)
}
