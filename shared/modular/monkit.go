// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

import (
	"fmt"

	"github.com/spacemonkeygo/monkit/v3"
)

type MonkitReport struct {
}

func NewMonkitReport() *MonkitReport {
	return &MonkitReport{}
}

func (m *MonkitReport) Close() {
	monkit.Default.Stats(func(key monkit.SeriesKey, field string, val float64) {
		fmt.Println(key, field, val)
	})
}
