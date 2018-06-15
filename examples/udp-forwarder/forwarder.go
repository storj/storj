// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net"
)

func main() {
	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:10001")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	receiver1, err := net.ResolveUDPAddr("udp", "127.0.0.1:7777")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	receiver2, err := net.ResolveUDPAddr("udp", "127.0.0.1:8888")
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

	buf := make([]byte, 1024*10)

	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Println("Received ", string(buf[0:n]), " from ", addr)

		_, err = conn.WriteTo(buf[0:n], receiver1)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		_, err = conn.WriteTo(buf[0:n], receiver2)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	}
}
