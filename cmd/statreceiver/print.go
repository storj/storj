// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"sync"
	"time"
)

type Printer struct {
	mtx sync.Mutex
}

func NewPrinter() *Printer {
	return &Printer{}
}

func (p *Printer) Metric(application, instance string, key []byte, val float64,
	ts time.Time) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	_, err := fmt.Println(application, instance, string(key), val, ts.Unix())
	return err
}
