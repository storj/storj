// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"sync"
	"time"
)

// Printer is a MetricDest that writes to stdout
type Printer struct {
	mu sync.Mutex
}

// NewPrinter creates a Printer
func NewPrinter() *Printer {
	return &Printer{}
}

// Metric implements MetricDest
func (p *Printer) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, err := fmt.Println(application, instance, string(key), val, ts.Unix())
	return err
}

// PacketPrinter is a PacketDest that writes to stdout
type PacketPrinter struct {
	mu sync.Mutex
}

// NewPacketPrinter returns a PacketPrinter
func NewPacketPrinter() *PacketPrinter {
	return &PacketPrinter{}
}

// Packet implements PacketDest
func (p *PacketPrinter) Packet(data []byte, ts time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, err := fmt.Println("printing packet", len(data), ts.Unix())
	return err
}
