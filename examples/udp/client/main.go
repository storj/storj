// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

func main() {

	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:10001")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer conn.Close()

	i := 0
	for {
		msg := strconv.Itoa(i)
		i++
		buf := []byte(msg)
		_, err := conn.WriteTo(buf, serverAddr)
		if err != nil {
			fmt.Println(msg, err)
		}
		time.Sleep(time.Second * 1)
	}

}
