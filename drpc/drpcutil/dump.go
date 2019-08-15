// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcutil

import (
	"fmt"
	"io"
	"sync"

	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcwire"
)

type Dumper struct {
	out io.Writer
	mu  sync.Mutex
	err error
	buf []byte
}

func NewDumper(out io.Writer) *Dumper {
	return &Dumper{out: out}
}

func (d *Dumper) Write(p []byte) (n int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.err != nil {
		return 0, d.err
	}
	d.buf = append(d.buf, p...)

	fmt.Fprintf(d.out, "write: %x\n", p)

	defer func() {
		if err != nil && d.err == nil {
			d.err = err
		}
	}()

	for {
		advance, token, err := drpcwire.PacketScanner(d.buf, false)
		if err != nil {
			return len(p), err
		} else if token == nil {
			return len(p), nil
		}

		rem, pkt, ok, err := drpcwire.ParsePacket(token)
		if !ok || err != nil || len(rem) > 0 {
			return len(p), drpc.InternalError.New("invalid parse after scanner")
		}
		d.buf = d.buf[advance:]

		if _, err := fmt.Fprintf(d.out, "     | %s\n", pkt.String()); err != nil {
			return len(p), err
		}
	}
}
