// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"

	"cloud.google.com/go/profiler"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

var (
	initProfilerError = errs.Class("initializing profiler")

	debugProfilerName = flag.String("debug.profilername", "", "provide the name of the peer to enable continuous cpu/mem profiling for")
)

func initProfiler(log *zap.Logger) error {
	name := *debugProfilerName
	if name != "" {
		if err := profiler.Start(profiler.Config{
			Service:        name,
			ServiceVersion: "",
		}); err != nil {
			return initProfilerError.Wrap(err)
		}
		log.Debug("success debug profiler init")
	}
	return nil
}
