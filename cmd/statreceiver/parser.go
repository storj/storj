// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"
	"time"

	"github.com/zeebo/admission/admproto"
)

type Parser struct {
	d       MetricDest
	f       []PacketFilter
	scratch [1024 * 10]byte
}

func NewParser(d MetricDest, f ...PacketFilter) *Parser {
	return &Parser{d: d, f: f}
}

func (p *Parser) Packet(data []byte, ts time.Time) (err error) {
	data, err = admproto.CheckChecksum(data)
	if err != nil {
		return err
	}
	r := admproto.NewReaderWith(p.scratch[:])
	data, appb, instb, err := r.Begin(data)
	if err != nil {
		return err
	}
	app, inst := string(appb), string(instb)
	for _, f := range p.f {
		if !f.Filter(app, inst) {
			return nil
		}
	}
	var key []byte
	var value float64
	for len(data) > 0 {
		data, key, value, err = r.Next(data)
		if err != nil {
			return err
		}
		err = p.d.Metric(app, inst, key, value, ts)
		if err != nil {
			log.Printf("failed to write metric: %v", err)
			continue
		}
	}
	return nil
}
