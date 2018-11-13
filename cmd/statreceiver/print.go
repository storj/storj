// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"time"
)

type Printer struct{}

func NewPrinter() *Printer {
	return &Printer{}
}

func (p *Printer) Metric(application, instance string, key []byte, val float64,
	ts time.Time) error {
	_, err := fmt.Println(application, instance, string(key), val, ts.Unix())
	return err
}
