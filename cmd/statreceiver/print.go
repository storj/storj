// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"sync"
	"time"
)

// Printer is a MetricDest that writes to stdout
type Printer struct {
	mtx sync.Mutex
}

// NewPrinter creates a Printer
func NewPrinter() *Printer {
	return &Printer{}
}

// Metric implements MetricDest
func (p *Printer) Metric(application, instance string, key []byte, val float64,
	ts time.Time) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	_, err := fmt.Println(application, instance, string(key), val, ts.Unix())
	return err
}
