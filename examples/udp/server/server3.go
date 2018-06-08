// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net"
)

func main() {
	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8888")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	conn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer conn.Close()

	buf := make([]byte, 1024)

	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Println("Received ", string(buf[0:n]), " from ", addr)
	}
}
