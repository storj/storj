// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

var (
	fromPort = flag.Int("from", 0, "first port")
	toPort   = flag.Int("to", 10000, "last port")
)

func main() {
	flag.Parse()

	var listeners []net.Listener
	var unableToStart []int
	for port := *fromPort; port < *toPort; port++ {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			unableToStart = append(unableToStart, port)
			continue
		}
		listeners = append(listeners, listener)
	}
	fmt.Printf("use-ports: unable to start on %v\n", unableToStart)
	fmt.Printf("use-ports: listening on ports %v to %v\n", *fromPort, *toPort)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGQUIT)
	<-sigs

	for _, listener := range listeners {
		err := listener.Close()
		if err != nil {
			fmt.Printf("unable to close: %v\n", err)
		}
	}
}
