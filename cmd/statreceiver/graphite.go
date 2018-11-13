// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

type GraphiteDest struct {
	address string
	conn    net.Conn
	buf     *bufio.Writer
	last    time.Time
}

func NewGraphiteDest(address string) *GraphiteDest {
	return &GraphiteDest{address: address}
}

func (d *GraphiteDest) Metric(application, instance string,
	key []byte, val float64, ts time.Time) error {

	if d.conn == nil {
		conn, err := net.Dial("tcp", d.address)
		if err != nil {
			return err
		}
		d.conn = conn
		d.buf = bufio.NewWriter(conn)
	}

	_, err := fmt.Fprintf(d.buf, "%s.%s.%s %v %d\n", application, string(key),
		instance, val, ts.Unix())

	if err != nil {
		return err
	}

	if time.Since(d.last) > 5*time.Second {
		err = d.buf.Flush()
		if err != nil {
			return err
		}
		d.last = time.Now()
	}
	return nil
}
