package main

import (
	"fmt"
	"log"

	"storj.io/storj/pkg/netstate"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/process"
)

func main() {
	fmt.Println("starting up overlay cache and dht network")
	err := process.Main(&overlay.Service{}, &netstate.Service{})

	if err != nil {
		log.Fatal(err)
	}

	return
}
