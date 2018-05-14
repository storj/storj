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
			ColumnName:  "Uptime",
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
			ColumnName:  "Uptime",
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
			ColumnName:  "Uptime",
			Operand:     nodereputation.NodeFilter_LESS_THAN,
			ColumnValue: "10",
		},
	)
	if err != nil {
		fmt.Println("Filter response err")
	}
	fmt.Println("Filter reply", filter.Records)

}
