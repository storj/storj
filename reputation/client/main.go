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
			ColumnValue: "111",
		})

	if err != nil {
		fmt.Println("response err")
	}
	fmt.Println("Reply receive", response.Status)

}
