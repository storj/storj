// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux || windows || (darwin && cgo)

package storagenode

import (
	"os"
	"sync"

	hw "github.com/jtolds/monkit-hw/v2"
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/storj/storagenode/load"
)

var (
	onceInitializeDiskMon sync.Once
)

func init() {
	// Don't just use hw.Register(), which includes
	// too many things. In particular, hw.Disk() and
	// hw.Network() seem a bit too invasive to monitor.
	// Instead, selectively choose just some things.
	pkg := monkit.Default.ScopeNamed("github.com/jtolds/monkit-hw/v2")
	pkg.Chain(hw.CPU())
	pkg.Chain(hw.Misc())
	pkg.Chain(hw.Memory())
	pkg.Chain(hw.Load())
}

func initializeDiskMon(log *zap.Logger) {
	onceInitializeDiskMon.Do(func() {
		pid := os.Getpid()
		mon.Chain(load.DiskIO(log.Named("diskio"), int32(pid)))
	})
}
