package main

import (
	"fmt"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"storj.io/storj/reputation"
)

func main() {

	var conn *grpc.ClientConn

	conn, err := grpc.Dial(":7777", grpc.WithInsecure())
	if err != nil {
		fmt.Println("conn err")
	}
	defer conn.Close()

	c := reputation.NewBridgeClient(conn)

	response, err := c.UpdateReputation(context.Background(),
		&reputation.NodeUpdate{
			Source:      "Bob",
			NodeName:    "Alice",
			ColumnName:  "Uptime",
			ColumnValue: "2",
		},
	)

	if err != nil {
		fmt.Println("Update response err")
	}
	fmt.Println("Reply receive", response.Status)

	agg, err := c.QueryAggregatedNodeInfo(context.Background(),
		&reputation.NodeQuery{
			Source:   "Bob",
			NodeName: "Alice",
		},
	)
	if err != nil {
		fmt.Println("Agg respnse err")
	}
	fmt.Println("Agg receive", agg)

}
