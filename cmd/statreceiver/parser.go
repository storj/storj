// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"
	"sync"
	"time"

	"github.com/zeebo/admission/admproto"
)

const (
	kb = 1024
)

// Parser is a PacketDest that sends data to a MetricDest
type Parser struct {
	dest    MetricDest
	filters []*PacketFilter
	scratch sync.Pool
}

// NewParser creates a Parser. It sends metrics to dest, provided they pass all
// of the provided PacketFilters
func NewParser(dest MetricDest, filters ...*PacketFilter) *Parser {
	return &Parser{
		dest:    dest,
		filters: filters,
		scratch: sync.Pool{
			New: func() interface{} {
				var x [10 * kb]byte
				return &x
			},
		},
	}
}

// Packet implements PacketDest
func (p *Parser) Packet(data []byte, ts time.Time) (err error) {
	data, err = admproto.CheckChecksum(data)
	if err != nil {
		return err
	}

	scratch := p.scratch.Get().(*[10 * kb]byte)
	defer p.scratch.Put(scratch)

	r := admproto.NewReaderWith((*scratch)[:])
	data, appb, instb, err := r.Begin(data)
	if err != nil {
		return err
	}

	app, inst := string(appb), string(instb)
	for _, filter := range p.filters {
		if !filter.Filter(app, inst) {
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
		err = p.dest.Metric(app, inst, key, value, ts)
		if err != nil {
			log.Printf("failed to write metric: %v", err)
			continue
		}
	}

	return nil
}
