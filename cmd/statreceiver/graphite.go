// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// GraphiteDest is a MetricDest that sends data with the Graphite TCP wire
// protocol
type GraphiteDest struct {
	mtx     sync.Mutex
	address string
	conn    net.Conn
	buf     *bufio.Writer
}

// NewGraphiteDest creates a GraphiteDest with TCP address address
func NewGraphiteDest(address string) *GraphiteDest {
	rv := &GraphiteDest{address: address}
	go rv.flush()
	return rv
}

// Metric implements MetricDest
func (d *GraphiteDest) Metric(application, instance string,
	key []byte, val float64, ts time.Time) error {

	d.mtx.Lock()
	defer d.mtx.Unlock()

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
	return err
}

func (d *GraphiteDest) flush() {
	for {
		time.Sleep(5 * time.Second)
		d.mtx.Lock()
		var err error
		if d.buf != nil {
			err = d.buf.Flush()
		}
		d.mtx.Unlock()
		if err != nil {
			log.Printf("failed flushing: %v", err)
		}
	}
}
