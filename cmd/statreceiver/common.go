// Copyright (C) 2019 Storj Labs, Inc.
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

// Deliver kicks off a goroutine that reads packets from source and delivers them
// to dest. To stop delivery, call Close on the return value then close the source.
func Deliver(source Source, dest PacketDest) io.Closer {
	done := new(uint32)

	go func() {
		for atomic.LoadUint32(done) == 0 {
			data, ts, err := source.Next()
			if err != nil {
				log.Printf("failed getting packet: %v", err)
				continue
			}
			err = dest.Packet(data, ts)
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
	Metric(application, instance string, key []byte, val float64, ts time.Time) error
}
