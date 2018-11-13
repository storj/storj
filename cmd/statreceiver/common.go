// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"
	"time"
)

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

type Source interface {
	Next() (data []byte, ts time.Time, err error)
}

type PacketDest interface {
	Packet(data []byte, ts time.Time) error
}

type MetricDest interface {
	Metric(application, instance string, key []byte, val float64, ts time.Time) (
		err error)
}
