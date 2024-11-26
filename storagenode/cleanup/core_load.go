// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import (
	"runtime"

	"github.com/zeebo/errs"
)

// CoreLoadConfig is the configuration for core load.
type CoreLoadConfig struct {
	MaxCoreLoad float64 `help:"max Linux load / core for executing background jobs. Jobs will be cancelled if load is higher" default:"10"`
}

// CoreLoad is an availability check which is false if load/core is high.
type CoreLoad struct {
	config CoreLoadConfig
}

// NewCoreLoad creates a new CoreLoad.
func NewCoreLoad(config CoreLoadConfig) *CoreLoad {
	return &CoreLoad{config: config}
}

var monLoad = mon.FloatVal("core_load")

// Enabled implements Enablement.
func (s *CoreLoad) Enabled() (bool, error) {
	cores := float64(runtime.NumCPU())
	load, err := getLoad()
	if err != nil {
		return false, errs.Wrap(err)
	}
	monLoad.Observe(load / cores)
	return load/cores < s.config.MaxCoreLoad, nil
}
