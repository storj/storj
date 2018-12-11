// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"log"
	"time"

	"storj.io/storj/pkg/utils"
)

// PacketCopier sends the same packet to multiple destinations
type PacketCopier struct {
	d []PacketDest
}

// NewPacketCopier creates a packet copier that sends the same packets to
// the provided different destinations
func NewPacketCopier(d ...PacketDest) *PacketCopier {
	return &PacketCopier{d: d}
}

// Packet implements the PacketDest interface
func (p *PacketCopier) Packet(data []byte, ts time.Time) (ferr error) {
	var errs utils.ErrorGroup
	for _, d := range p.d {
		errs.Add(d.Packet(data, ts))
	}
	return errs.Finish()
}

// MetricCopier sends the same metric to multiple destinations
type MetricCopier struct {
	d []MetricDest
}

// NewMetricCopier creates a metric copier that sends the same metrics to
// the provided different destinations
func NewMetricCopier(d ...MetricDest) *MetricCopier {
	return &MetricCopier{d: d}
}

// Metric implements the MetricDest interface
func (m *MetricCopier) Metric(application, instance string,
	key []byte, val float64, ts time.Time) (ferr error) {
	var errs utils.ErrorGroup
	for _, d := range m.d {
		errs.Add(d.Metric(application, instance, key, val, ts))
	}
	return errs.Finish()
}

// Packet represents a single packet
type Packet struct {
	Data []byte
	TS   time.Time
}

// PacketBuffer is a packet buffer. It has a given buffer size and allows
// packets to buffer in memory to deal with potentially variable processing
// speeds. PacketBuffers drop packets if the buffer is full.
type PacketBuffer struct {
	ch chan Packet
}

// NewPacketBuffer makes a packet buffer with a buffer size of bufsize
func NewPacketBuffer(p PacketDest, bufsize int) *PacketBuffer {
	ch := make(chan Packet, bufsize)
	go func() {
		for pkt := range ch {
			err := p.Packet(pkt.Data, pkt.TS)
			if err != nil {
				log.Printf("failed delivering buffered packet: %v", err)
			}
		}
	}()
	return &PacketBuffer{ch: ch}
}

// Packet implements the PacketDest interface
func (p *PacketBuffer) Packet(data []byte, ts time.Time) error {
	select {
	case p.ch <- Packet{Data: data, TS: ts}:
		return nil
	default:
		return fmt.Errorf("packet buffer overrun")
	}
}

// Metric represents a single metric
type Metric struct {
	Application string
	Instance    string
	Key         []byte
	Val         float64
	TS          time.Time
}

// MetricBuffer is a metric buffer. It has a given buffer size and allows
// metrics to buffer in memory to deal with potentially variable processing
// speeds. MetricBuffers drop metrics if the buffer is full.
type MetricBuffer struct {
	ch chan Metric
}

// NewMetricBuffer makes a metric buffer with a buffer size of bufsize
func NewMetricBuffer(p MetricDest, bufsize int) *MetricBuffer {
	ch := make(chan Metric, bufsize)
	go func() {
		for pkt := range ch {
			err := p.Metric(pkt.Application, pkt.Instance, pkt.Key, pkt.Val, pkt.TS)
			if err != nil {
				log.Printf("failed delivering buffered metric: %v", err)
			}
		}
	}()
	return &MetricBuffer{ch: ch}
}

// Metric implements the MetricDest interface
func (p *MetricBuffer) Metric(application, instance string, key []byte,
	val float64, ts time.Time) error {
	select {
	case p.ch <- Metric{
		Application: application,
		Instance:    instance,
		Key:         key,
		Val:         val,
		TS:          ts}:
		return nil
	default:
		return fmt.Errorf("metric buffer overrun")
	}
}

// PacketBufPrep prepares a packet destination for a packet buffer.
// By default, packet memory is reused, which would cause data race conditions
// when a buffer is also used. PacketBufPrep copies the memory to make sure
// there are no data races
type PacketBufPrep struct {
	d PacketDest
}

// NewPacketBufPrep creates a PacketBufPrep
func NewPacketBufPrep(d PacketDest) *PacketBufPrep {
	return &PacketBufPrep{d: d}
}

// Packet implements the PacketDest interface
func (p *PacketBufPrep) Packet(data []byte, ts time.Time) error {
	return p.d.Packet(append([]byte(nil), data...), ts)
}

// MetricBufPrep prepares a metric destination for a metric buffer.
// By default, metric key memory is reused, which would cause data race
// conditions when a buffer is also used. MetricBufPrep copies the memory to
// make sure there are no data races
type MetricBufPrep struct {
	d MetricDest
}

// NewMetricBufPrep creates a MetricBufPrep
func NewMetricBufPrep(d MetricDest) *MetricBufPrep {
	return &MetricBufPrep{d: d}
}

// Metric implements the MetricDest interface
func (p *MetricBufPrep) Metric(application, instance string, key []byte,
	val float64, ts time.Time) error {
	return p.d.Metric(application, instance, append([]byte(nil), key...), val, ts)
}
