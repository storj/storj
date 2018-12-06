// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"io"
	"log"
	"sync/atomic"
	"time"
)

type closerfunc func() error

func (f closerfunc) Close() error { return f() }

// Deliver kicks off a goroutine that reads packets from s and delivers them
// to p. To stop delivery, call Close on the return value then close the source.
func Deliver(s Source, p PacketDest) io.Closer {
	done := new(uint32)
	go func() {
		for {
			data, ts, err := s.Next()
			if atomic.LoadUint32(done) == 1 {
				return
			}
			if err != nil {
				log.Printf("failed getting packet: %v", err)
				continue
			}
			err = p.Packet(data, ts)
			if err != nil {
				log.Printf("failed delivering packet: %v", err)
				continue
			}
		}
	}()
	return closerfunc(func() error {
		atomic.StoreUint32(done, 1)
		return nil
	})
}

// Source reads incoming packets
type Source interface {
	Next() (data []byte, ts time.Time, err error)
}

// PacketDest handles packets
type PacketDest interface {
	Packet(data []byte, ts time.Time) error
}

// MetricDest handles metrics
type MetricDest interface {
	Metric(application, instance string, key []byte, val float64, ts time.Time) (
		err error)
}
