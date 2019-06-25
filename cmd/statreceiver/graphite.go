// Copyright (C) 2019 Storj Labs, Inc.
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
	address string

	mu      sync.Mutex
	conn    net.Conn
	buf     *bufio.Writer
	stopped bool
}

// NewGraphiteDest creates a GraphiteDest with TCP address address. Because
// this function is called in a Lua pipeline domain-specific language, the DSL
// wants a graphite destination to be flushing every few seconds, so this
// constructor will start that process. Use Close to stop it.
func NewGraphiteDest(address string) *GraphiteDest {
	rv := &GraphiteDest{address: address}
	go rv.flush()
	return rv
}

// Metric implements MetricDest
func (d *GraphiteDest) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.conn == nil {
		conn, err := net.Dial("tcp", d.address)
		if err != nil {
			return err
		}
		// TODO(leak): free connection
		d.conn = conn
		d.buf = bufio.NewWriter(conn)
	}

	_, err := fmt.Fprintf(d.buf, "%s.%s.%s %v %d\n", application, instance, string(key), val, ts.Unix())
	return err
}

// Close stops the flushing goroutine
func (d *GraphiteDest) Close() error {
	d.mu.Lock()
	d.stopped = true
	d.mu.Unlock()
	return nil
}

func (d *GraphiteDest) flush() {
	for {
		time.Sleep(5 * time.Second)
		d.mu.Lock()
		if d.stopped {
			d.mu.Unlock()
			return
		}
		var err error
		if d.buf != nil {
			err = d.buf.Flush()
		}
		d.mu.Unlock()
		if err != nil {
			log.Printf("failed flushing: %v", err)
		}
	}
}
