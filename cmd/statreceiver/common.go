// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"
	"time"
)

// Deliver kicks off a goroutine that reads packets from s and delivers them
// to p.
func Deliver(s Source, p PacketDest) {
	go func() {
		for {
			data, ts, err := s.Next()
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
