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
		fmt.Println("err")
	}
	defer conn.Close()

	c := reputation.NewBridgeClient(conn)

	response, err := c.UpdateReputation(context.Background(),
		&reputation.NodeReputation{
			Source:             "Bob",
			NodeName:           "Alice",
			Timestamp:          "",
			Uptime:             1,
			AuditSuccess:       1,
			AuditFail:          0,
			Latency:            1,
			AmountOfDataStored: 1,
			FalseClaims:        0,
			ShardsModified:     0,
		})

	if err != nil {
		fmt.Println("err")
	}
	fmt.Println("Reply %v", response)

}
