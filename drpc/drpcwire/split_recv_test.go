// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire_test

import (
	"fmt"
	"io"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	"storj.io/storj/drpc/drpctest"
	"storj.io/storj/drpc/drpcutil"
	"storj.io/storj/drpc/drpcwire"
	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
)

func TestSplitRecv(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	start := time.Now()
	const numPackets = 100

	// set up shared state for the senders and receivers to communicate
	pr, pw := io.Pipe()
	chRecv := make(chan drpcwire.Packet, numPackets)
	chSent := make(chan drpcwire.Packet, numPackets)

	// launch a goroutine to receive packets and send them down a channel
	ctx.Go(func() error {
		defer close(chRecv)
		defer pr.Close()

		recv := drpcwire.NewReceiver(pr)
		for {
			pkt, err := recv.ReadPacket()
			if err != nil {
				fmt.Println(err)
				return err
			} else if pkt == nil {
				return nil
			}
			chRecv <- *pkt
		}
	})

	// launch a group of goroutines to send a packet to the receiver
	var group errgroup.Group
	for i := 0; i < numPackets; i++ {
		group.Go(func() error {
			buf := drpcutil.NewBuffer(pw, 64*1024)
			pkt := drpctest.RandPacket()
			chSent <- pkt

			err := drpcwire.Split(pkt, buf.Write)
			if err != nil {
				pw.CloseWithError(err)
				return err
			}
			if err := buf.Flush(); err != nil {
				pw.CloseWithError(err)
				return err
			}
			return nil
		})
	}

	// launch a goroutine to wait on the senders to complete to signal that
	// no more sends will happen to the reader.
	ctx.Go(func() error {
		defer close(chSent)
		defer pw.Close()
		return group.Wait()
	})

	// wait for everything to happen
	ctx.Wait()
	stop := time.Now()

	// record what was sent and what was received in a way that makes it easy
	// to compare the two and then ensure they are equal.
	size := int64(0)
	got := make(map[drpcwire.PacketID][]byte)
	for pkt := range chRecv {
		got[pkt.PacketID] = pkt.Data
		size += int64(len(pkt.Data))
	}
	exp := make(map[drpcwire.PacketID][]byte)
	for pkt := range chSent {
		exp[pkt.PacketID] = pkt.Data
		size += int64(len(pkt.Data))
	}
	t.Logf("rate: %s/s", memory.Size(float64(size)/stop.Sub(start).Seconds()))

	if len(got) != len(exp) {
		t.Fatalf("got:%d exp:%d", len(got), len(exp))
	}
	for pid, gdata := range got {
		if edata, ok := exp[pid]; !ok || string(gdata) != string(edata) {
			t.Log("ok: ", ok)
			t.Log("got:", &drpcwire.Packet{PacketID: pid, Data: gdata})
			t.Log("exp:", &drpcwire.Packet{PacketID: pid, Data: edata})
			t.FailNow()
		}
	}
	for pid, edata := range exp {
		if gdata, ok := got[pid]; !ok || string(edata) != string(gdata) {
			t.Log("ok: ", ok)
			t.Log("got:", &drpcwire.Packet{PacketID: pid, Data: gdata})
			t.Log("exp:", &drpcwire.Packet{PacketID: pid, Data: edata})
			t.FailNow()
		}
	}
}
