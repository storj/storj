// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

import (
	"fmt"

	"github.com/spacemonkeygo/monkit/v3"
)

// MonkitReport is helper to print out monkit report on exit.
type MonkitReport struct {
}

// NewMonkitReport creates a new monkit report.
func NewMonkitReport() *MonkitReport {
	return &MonkitReport{}
}

// Close prints out all monkit metrics during close phase (on exit...).
func (m *MonkitReport) Close() {
	monkit.Default.Stats(func(key monkit.SeriesKey, field string, val float64) {
		fmt.Println(key, field, val)
	})
}
