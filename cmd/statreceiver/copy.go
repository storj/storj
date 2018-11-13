// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"log"
	"time"
)

type PacketCopier struct {
	d []PacketDest
}

func NewPacketCopier(d ...PacketDest) *PacketCopier {
	return &PacketCopier{d: d}
}

func (p *PacketCopier) Packet(data []byte, ts time.Time) (ferr error) {
	for _, d := range p.d {
		err := d.Packet(data, ts)
		if ferr == nil && err != nil {
			ferr = err
		}
	}
	return ferr
}

type MetricCopier struct {
	d []MetricDest
}

func NewMetricCopier(d ...MetricDest) *MetricCopier {
	return &MetricCopier{d: d}
}

func (m *MetricCopier) Metric(application, instance string,
	key []byte, val float64, ts time.Time) (ferr error) {
	for _, d := range m.d {
		err := d.Metric(application, instance, key, val, ts)
		if ferr == nil && err != nil {
			ferr = err
		}
	}
	return ferr
}

type Packet struct {
	Data []byte
	TS   time.Time
}

type PacketBuffer struct {
	ch chan Packet
}

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

func (p *PacketBuffer) Packet(data []byte, ts time.Time) error {
	select {
	case p.ch <- Packet{Data: append([]byte(nil), data...), TS: ts}:
		return nil
	default:
		return fmt.Errorf("packet buffer overrun")
	}
}

type Metric struct {
	Application, Instance string
	Key                   []byte
	Val                   float64
	TS                    time.Time
}

type MetricBuffer struct {
	ch chan Metric
}

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

func (p *MetricBuffer) Metric(application, instance string, key []byte,
	val float64, ts time.Time) error {
	select {
	case p.ch <- Metric{
		Application: application,
		Instance:    instance,
		Key:         append([]byte(nil), key...),
		Val:         val,
		TS:          ts}:
		return nil
	default:
		return fmt.Errorf("metric buffer overrun")
	}
}
