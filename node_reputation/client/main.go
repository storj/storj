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

	c := nodereputation.NewNodeReputationClient(conn)

	response, err := c.UpdateReputation(context.Background(),
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

	agg, err := c.QueryAggregatedNodeInfo(context.Background(),
		&nodereputation.NodeQuery{
			Source:   "Bob",
			NodeName: "Alice",
		},
	)
	if err != nil {
		fmt.Println("Agg respnse err")
	}
	fmt.Println("Agg receive", agg)

}
